package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/golang-jwt/jwt/v5"
)

type Role string

type PrincipalType string

const (
	RoleAdmin          Role = "admin"
	RoleProjectClient  Role = "project-client"
	RoleReadOnlyClient Role = "read-only-client"

	PrincipalTypeUser    PrincipalType = "user"
	PrincipalTypeService PrincipalType = "service"
)

var (
	ErrInvalidRole          = errors.New("invalid role")
	ErrInvalidPrincipalType = errors.New("invalid principal type")
	ErrTokenExpired         = errors.New("token expired")
	ErrTokenInvalid         = errors.New("invalid token")
	ErrMissingClaim         = errors.New("missing required claim")
)

type Config struct {
	Issuer   string
	Audience string
	JWKSURL  string
	Enabled  bool
}

type Claims struct {
	Subject       string        `json:"sub"`
	AppID         string        `json:"app_id"`
	ProjectID     string        `json:"project_id"`
	Role          Role          `json:"role"`
	PrincipalType PrincipalType `json:"principal_type"`
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
	projectID, _ := mc["project_id"].(string)
	if sub == "" || appID == "" || projectID == "" {
		return Claims{}, fmt.Errorf("%w: %w", ErrTokenInvalid, ErrMissingClaim)
	}

	roleRaw, _ := mc["role"].(string)
	role, err := ParseRole(roleRaw)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	principalTypeRaw, _ := mc["principal_type"].(string)
	if principalTypeRaw == "" {
		return Claims{}, fmt.Errorf("%w: %w", ErrTokenInvalid, ErrMissingClaim)
	}

	principalType, err := ParsePrincipalType(principalTypeRaw)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrTokenInvalid, err)
	}

	return Claims{
		Subject:       sub,
		AppID:         appID,
		ProjectID:     projectID,
		Role:          role,
		PrincipalType: principalType,
	}, nil
}

// keyfunc runs a goroutine to refresh JWKS keys in the background. Call EndBackground() to stop it.
func (v *JWTVerifier) Close() {
	if v.jwks != nil {
		v.jwks.EndBackground()
	}
}

func ParseRole(raw string) (Role, error) {
	switch Role(raw) {
	case RoleAdmin, RoleProjectClient, RoleReadOnlyClient:
		return Role(raw), nil
	default:
		return "", ErrInvalidRole
	}
}

func ParsePrincipalType(raw string) (PrincipalType, error) {
	switch PrincipalType(raw) {
	case PrincipalTypeUser, PrincipalTypeService:
		return PrincipalType(raw), nil
	default:
		return "", ErrInvalidPrincipalType
	}
}
