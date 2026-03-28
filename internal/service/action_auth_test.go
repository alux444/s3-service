package service

import (
	"context"
	"errors"
	"testing"

	"s3-service/internal/auth"
	"s3-service/internal/database"
)

type stubAuthorizationRepo struct {
	policy database.EffectiveAuthorizationPolicy
	err    error
}

func (s *stubAuthorizationRepo) GetEffectiveAuthorizationPolicy(
	_ context.Context,
	_ string,
	_ string,
	_ string,
	_ string,
) (database.EffectiveAuthorizationPolicy, error) {
	if s.err != nil {
		return database.EffectiveAuthorizationPolicy{}, s.err
	}
	return s.policy, nil
}

func TestAuthorizationService_Authorize(t *testing.T) {
	claims := auth.Claims{
		Subject:   "user-1",
		ProjectID: "project-1",
		AppID:     "app-1",
		Role:      auth.RoleProjectClient,
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

		d := svc.Authorize(context.Background(), claims, "bucket-a", auth.ActionWrite, "uploads/file.jpg")
		if !d.Allowed {
			t.Fatalf("expected allowed, got denied with reason %s", d.Reason)
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

		d := svc.Authorize(context.Background(), claims, "bucket-a", auth.ActionWrite, "uploads/file.jpg")
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

		d := svc.Authorize(context.Background(), claims, "bucket-a", auth.ActionRead, "images/public/a.jpg")
		if d.Allowed || d.Reason != auth.DecisionReasonPrefixScope {
			t.Fatalf("expected prefix_scope deny, got %+v", d)
		}
	})

	t.Run("denies when no policy is found", func(t *testing.T) {
		repo := &stubAuthorizationRepo{err: database.ErrPolicyNotFound}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), claims, "bucket-a", auth.ActionRead, "images/private/a.jpg")
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

		d := svc.Authorize(context.Background(), auth.Claims{}, "bucket-a", auth.ActionList, "a/")
		if d.Allowed || d.Reason != auth.DecisionReasonInvalidInput {
			t.Fatalf("expected invalid_input deny, got %+v", d)
		}
	})

	t.Run("denies unknown repo errors as bucket scope", func(t *testing.T) {
		repo := &stubAuthorizationRepo{err: errors.New("db timeout")}
		svc := NewAuthorizationService(repo)

		d := svc.Authorize(context.Background(), claims, "bucket-a", auth.ActionRead, "images/private/a.jpg")
		if d.Allowed || d.Reason != auth.DecisionReasonBucketScope {
			t.Fatalf("expected bucket_scope deny, got %+v", d)
		}
	})
}
