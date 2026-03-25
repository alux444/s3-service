package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrTokenExpired = errors.New("token expired")
	ErrTokenInvalid = errors.New("invalid token")
	ErrMissingClaim = errors.New("missing required claim")
)

type Config struct {
	Issuer   string
	Audience string
	JWKSURL  string
	Enabled  bool
}

type Claims struct {
	Subject string `json:"sub"`
	AppID   string `json:"app_id"`
}

type JWTVerifier struct {
	enabled  bool
	issuer   string
	audience string
	jwks     *keyfunc.JWKS
}

func NewJWTVerifier(cfg Config) (*JWTVerifier, error) {
	v := &JWTVerifier{
		enabled:  cfg.Enabled,
		issuer:   cfg.Issuer,
		audience: cfg.Audience,
	}

	if !cfg.Enabled {
		return v, nil
	}

	if cfg.Issuer == "" || cfg.Audience == "" || cfg.JWKSURL == "" {
		return nil, errors.New("issuer, audience, and JWKS URL must be provided when JWT is enabled")
	}

	jwks, err := keyfunc.Get(cfg.JWKSURL, keyfunc.Options{
		RefreshInterval:   time.Hour,
		RefreshUnknownKID: true,
	})
	if err != nil {
		return nil, fmt.Errorf("load JWKS: %w", err)
	}

	v.jwks = jwks
	return v, nil
}

func (v *JWTVerifier) Verify(tokenString string) (Claims, error) {
	return v.VerifyWithContext(context.Background(), tokenString)
}

func (v *JWTVerifier) VerifyWithContext(ctx context.Context, tokenString string) (Claims, error) {
	if !v.enabled {
		return Claims{}, nil
	}

	select {
	case <-ctx.Done():
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenInvalid, ctx.Err())
	default:
	}

	token, err := jwt.Parse(
		tokenString,
		v.jwks.Keyfunc,
		jwt.WithAudience(v.audience),
		jwt.WithIssuer(v.issuer),
		jwt.WithLeeway(60*time.Second),
		jwt.WithValidMethods([]string{"RS256"}),
	)

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return Claims{}, ErrTokenExpired
		}
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	if !token.Valid {
		return Claims{}, ErrTokenInvalid
	}

	mc, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return Claims{}, ErrTokenInvalid
	}

	sub, _ := mc["sub"].(string)
	appID, _ := mc["app_id"].(string)
	if sub == "" || appID == "" {
		return Claims{}, fmt.Errorf("%w: %w", ErrTokenInvalid, ErrMissingClaim)
	}

	return Claims{
		Subject: sub,
		AppID:   appID,
	}, nil
}

// keyfunc runs a goroutine to refresh JWKS keys in the background. Call EndBackground() to stop it.
func (v *JWTVerifier) Close() {
	if v.jwks != nil {
		v.jwks.EndBackground()
	}
}
