package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

type stubAuthorizationRepo struct {
	policy         database.EffectiveAuthorizationPolicy
	err            error
	principalType  string
	principalID    string
	lastBucketName string
}

func (s *stubAuthorizationRepo) GetEffectiveAuthorizationPolicy(
	_ context.Context,
	lookup database.AuthorizationPolicyLookup,
) (database.EffectiveAuthorizationPolicy, error) {
	s.principalType = lookup.PrincipalType
	s.principalID = lookup.PrincipalID
	s.lastBucketName = lookup.BucketName
	if s.err != nil {
		return database.EffectiveAuthorizationPolicy{}, s.err
	}
	return s.policy, nil
}

func TestAuthorizationService_Authorize(t *testing.T) {
	claims := auth.Claims{
		Subject:       "user-1",
		ProjectID:     "project-1",
		AppID:         "app-1",
		Role:          auth.RoleProjectClient,
		PrincipalType: auth.PrincipalTypeUser,
	}

	t.Run("allows when action and prefix are permitted", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanWrite:           true,
				ConnectionPrefixes: []string{"uploads/"},
				PrincipalPrefixes:  []string{"uploads/"},
			},
		}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: "bucket-a",
			Action:     auth.ActionWrite,
			ObjectKey:  "uploads/file.jpg",
		})
		if !d.Allowed {
			t.Fatalf("expected allowed, got denied with reason %s", d.Reason)
		}
		if repo.principalType != string(auth.PrincipalTypeUser) {
			t.Fatalf("expected principalType %q, got %q", auth.PrincipalTypeUser, repo.principalType)
		}
	})

	t.Run("forwards service principal type", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanRead:            true,
				ConnectionPrefixes: []string{"svc/"},
				PrincipalPrefixes:  []string{"svc/"},
			},
		}
		svc := NewAuthorizationService(repo)

		serviceClaims := claims
		serviceClaims.PrincipalType = auth.PrincipalTypeService
		serviceClaims.Subject = "service-1"

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     serviceClaims,
			BucketName: "bucket-a",
			Action:     auth.ActionRead,
			ObjectKey:  "svc/file.json",
		})
		if !d.Allowed {
			t.Fatalf("expected allowed, got denied with reason %s", d.Reason)
		}
		if repo.principalType != string(auth.PrincipalTypeService) {
			t.Fatalf("expected principalType %q, got %q", auth.PrincipalTypeService, repo.principalType)
		}
		if repo.principalID != "service-1" {
			t.Fatalf("expected principalID service-1, got %q", repo.principalID)
		}
	})

	t.Run("defaults missing principal type to user", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanRead:            true,
				ConnectionPrefixes: []string{"images/"},
				PrincipalPrefixes:  []string{"images/"},
			},
		}
		svc := NewAuthorizationService(repo)

		claimsWithoutType := claims
		claimsWithoutType.PrincipalType = ""

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claimsWithoutType,
			BucketName: "bucket-a",
			Action:     auth.ActionRead,
			ObjectKey:  "images/file.jpg",
		})
		if !d.Allowed {
			t.Fatalf("expected allowed, got denied with reason %s", d.Reason)
		}
		if repo.principalType != string(auth.PrincipalTypeUser) {
			t.Fatalf("expected default principalType %q, got %q", auth.PrincipalTypeUser, repo.principalType)
		}
	})

	t.Run("denies when action is not permitted", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanWrite:           false,
				ConnectionPrefixes: []string{"uploads/"},
				PrincipalPrefixes:  []string{"uploads/"},
			},
		}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: "bucket-a",
			Action:     auth.ActionWrite,
			ObjectKey:  "uploads/file.jpg",
		})
		if d.Allowed || d.Reason != auth.DecisionReasonActionScope {
			t.Fatalf("expected action_scope deny, got %+v", d)
		}
	})

	t.Run("denies when key is outside effective prefixes", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanRead:            true,
				ConnectionPrefixes: []string{"images/"},
				PrincipalPrefixes:  []string{"images/private/"},
			},
		}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: "bucket-a",
			Action:     auth.ActionRead,
			ObjectKey:  "images/public/a.jpg",
		})
		if d.Allowed || d.Reason != auth.DecisionReasonPrefixScope {
			t.Fatalf("expected prefix_scope deny, got %+v", d)
		}
	})

	t.Run("denies when no policy is found", func(t *testing.T) {
		repo := &stubAuthorizationRepo{err: database.ErrPolicyNotFound}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: "bucket-a",
			Action:     auth.ActionRead,
			ObjectKey:  "images/private/a.jpg",
		})
		if d.Allowed || d.Reason != auth.DecisionReasonBucketScope {
			t.Fatalf("expected bucket_scope deny, got %+v", d)
		}
	})

	t.Run("denies invalid input", func(t *testing.T) {
		repo := &stubAuthorizationRepo{
			policy: database.EffectiveAuthorizationPolicy{
				CanList:            true,
				ConnectionPrefixes: []string{"a/"},
				PrincipalPrefixes:  []string{"a/"},
			},
		}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     auth.Claims{},
			BucketName: "bucket-a",
			Action:     auth.ActionList,
			ObjectKey:  "a/",
		})
		if d.Allowed || d.Reason != auth.DecisionReasonInvalidInput {
			t.Fatalf("expected invalid_input deny, got %+v", d)
		}
	})

	t.Run("denies unknown repo errors as bucket scope", func(t *testing.T) {
		repo := &stubAuthorizationRepo{err: errors.New("db timeout")}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), auth.AuthorizationRequest{
			Claims:     claims,
			BucketName: "bucket-a",
			Action:     auth.ActionRead,
			ObjectKey:  "images/private/a.jpg",
		})
		if d.Allowed || d.Reason != auth.DecisionReasonBucketScope {
			t.Fatalf("expected bucket_scope deny, got %+v", d)
		}
	})
}
