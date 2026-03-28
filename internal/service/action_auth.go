package service

import (
	"context"
	"errors"
	"strings"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

type AuthorizationRepository interface {
	GetEffectiveAuthorizationPolicy(
		ctx context.Context,
		projectID string,
		appID string,
		principalID string,
		bucketName string,
	) (database.EffectiveAuthorizationPolicy, error)
}

type AuthorizationService struct {
	repo AuthorizationRepository
}

func NewAuthorizationService(repo AuthorizationRepository) *AuthorizationService {
	return &AuthorizationService{repo: repo}
}

func (s *AuthorizationService) Authorize(
	ctx context.Context,
	claims auth.Claims,
	bucketName string,
	action auth.Action,
	objectKey string,
) auth.Decision {
	if claims.ProjectID == "" || claims.AppID == "" || claims.Subject == "" || bucketName == "" || !action.Valid() {
		return auth.Decision{Allowed: false, Reason: auth.DecisionReasonInvalidInput}
	}

	policy, err := s.repo.GetEffectiveAuthorizationPolicy(ctx, claims.ProjectID, claims.AppID, claims.Subject, bucketName)
	if err != nil {
		if errors.Is(err, database.ErrPolicyNotFound) {
			return auth.Decision{Allowed: false, Reason: auth.DecisionReasonBucketScope}
		}
		return auth.Decision{Allowed: false, Reason: auth.DecisionReasonBucketScope}
	}

	if !isActionAllowed(policy, action) {
		return auth.Decision{Allowed: false, Reason: auth.DecisionReasonActionScope}
	}

	if !isPrefixAllowed(policy.ConnectionPrefixes, policy.PrincipalPrefixes, objectKey, action) {
		return auth.Decision{Allowed: false, Reason: auth.DecisionReasonPrefixScope}
	}

	return auth.Decision{Allowed: true}
}

func isActionAllowed(policy database.EffectiveAuthorizationPolicy, action auth.Action) bool {
	switch action {
	case auth.ActionRead:
		return policy.CanRead
	case auth.ActionWrite:
		return policy.CanWrite
	case auth.ActionDelete:
		return policy.CanDelete
	case auth.ActionList:
		return policy.CanList
	default:
		return false
	}
}

func isPrefixAllowed(connectionPrefixes []string, principalPrefixes []string, objectKey string, action auth.Action) bool {
	effective := intersectPrefixes(connectionPrefixes, principalPrefixes)
	if len(effective) == 0 {
		return false
	}

	if action == auth.ActionList && objectKey == "" {
		return false
	}

	for _, prefix := range effective {
		if strings.HasPrefix(objectKey, prefix) {
			return true
		}
	}

	return false
}

func intersectPrefixes(connectionPrefixes []string, principalPrefixes []string) []string {
	if len(connectionPrefixes) == 0 || len(principalPrefixes) == 0 {
		return nil
	}

	allowed := make(map[string]struct{}, len(connectionPrefixes))
	for _, prefix := range connectionPrefixes {
		if prefix == "" {
			continue
		}
		allowed[prefix] = struct{}{}
	}

	result := make([]string, 0, len(principalPrefixes))
	for _, prefix := range principalPrefixes {
		if _, ok := allowed[prefix]; ok {
			result = append(result, prefix)
		}
	}

	return result
}
