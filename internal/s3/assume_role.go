package s3

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

const defaultSessionDuration = 15 * time.Minute

var ErrInvalidAssumeRoleInput = errors.New("invalid assume role input")

type BucketRoleReference struct {
	Region     string
	RoleARN    string
	ExternalID *string
}

type providerFactory func(BucketRoleReference, time.Duration) (aws.CredentialsProvider, error)

type AssumeRoleSessionCache struct {
	baseConfig      aws.Config
	sessionDuration time.Duration
	retryPolicy     retryPolicy

	mu       sync.RWMutex
	sessions map[string]aws.Config

	providerFactory providerFactory
}

type AssumeRoleSessionCacheOption func(*AssumeRoleSessionCache)

func WithAssumeRoleRetryPolicy(policy retryPolicy) AssumeRoleSessionCacheOption {
	return func(c *AssumeRoleSessionCache) {
		c.retryPolicy = policy
	}
}

func WithSessionDuration(duration time.Duration) AssumeRoleSessionCacheOption {
	return func(c *AssumeRoleSessionCache) {
		if duration > 0 {
			c.sessionDuration = duration
		}
	}
}

func WithProviderFactory(factory providerFactory) AssumeRoleSessionCacheOption {
	return func(c *AssumeRoleSessionCache) {
		if factory != nil {
			c.providerFactory = factory
		}
	}
}

func NewAssumeRoleSessionCache(ctx context.Context, baseConfig aws.Config, opts ...AssumeRoleSessionCacheOption) (*AssumeRoleSessionCache, error) {
	cfg := baseConfig
	if cfg.Credentials == nil {
		loaded, err := awsconfig.LoadDefaultConfig(ctx)
		if err != nil {
			return nil, fmt.Errorf("load default aws config: %w", err)
		}
		cfg = loaded
	}

	cache := &AssumeRoleSessionCache{
		baseConfig:      cfg,
		sessionDuration: defaultSessionDuration,
		retryPolicy:     defaultRetryPolicy(),
		sessions:        make(map[string]aws.Config),
	}
	cache.providerFactory = cache.defaultProviderFactory

	for _, opt := range opts {
		opt(cache)
	}

	return cache, nil
}

func (c *AssumeRoleSessionCache) ConfigForRole(ctx context.Context, ref BucketRoleReference) (aws.Config, error) {
	if ref.Region == "" || ref.RoleARN == "" {
		return aws.Config{}, fmt.Errorf("%w: region and role ARN are required", ErrInvalidAssumeRoleInput)
	}

	key := cacheKey(ref)
	if cfg, ok := c.get(key); ok {
		return cfg, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if cfg, ok := c.sessions[key]; ok {
		return cfg, nil
	}

	provider, err := c.providerFactory(ref, c.sessionDuration)
	if err != nil {
		return aws.Config{}, fmt.Errorf("build assume role provider: %w", err)
	}

	cfg := c.baseConfig.Copy()
	cfg.Region = ref.Region
	cfg.Credentials = aws.NewCredentialsCache(provider)

	// Prime once so invalid trust policy/external ID fails early.
	_, err = retryAWS(ctx, c.retryPolicy, func() (aws.Credentials, error) {
		return cfg.Credentials.Retrieve(ctx)
	})
	if err != nil {
		return aws.Config{}, fmt.Errorf("assume role for %s: %w", ref.RoleARN, err)
	}

	c.sessions[key] = cfg
	return cfg, nil
}

func (c *AssumeRoleSessionCache) get(key string) (aws.Config, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cfg, ok := c.sessions[key]
	return cfg, ok
}

func (c *AssumeRoleSessionCache) defaultProviderFactory(ref BucketRoleReference, duration time.Duration) (aws.CredentialsProvider, error) {
	stsClient := sts.NewFromConfig(c.baseConfig, func(options *sts.Options) {
		options.Region = ref.Region
	})

	provider := stscreds.NewAssumeRoleProvider(stsClient, ref.RoleARN, func(options *stscreds.AssumeRoleOptions) {
		options.Duration = duration
		if ref.ExternalID != nil && *ref.ExternalID != "" {
			options.ExternalID = ref.ExternalID
		}
	})

	return provider, nil
}

func cacheKey(ref BucketRoleReference) string {
	externalID := ""
	if ref.ExternalID != nil {
		externalID = *ref.ExternalID
	}
	return ref.Region + "|" + ref.RoleARN + "|" + externalID
}
