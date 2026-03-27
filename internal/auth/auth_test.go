package auth

import "testing"

func TestParseRole(t *testing.T) {
	t.Run("accepts supported roles", func(t *testing.T) {
		cases := []Role{RoleAdmin, RoleProjectClient, RoleReadOnlyClient}
		for _, tc := range cases {
			got, err := ParseRole(string(tc))
			if err != nil {
				t.Fatalf("expected no error for role %q, got %v", tc, err)
			}
			if got != tc {
				t.Fatalf("expected role %q, got %q", tc, got)
			}
		}
	})

	t.Run("rejects unsupported role", func(t *testing.T) {
		_, err := ParseRole("invalid_role")
		if err == nil {
			t.Fatal("expected error for invalid role")
		}
		if err != ErrInvalidRole {
			t.Fatalf("expected ErrInvalidRole, got %v", err)
		}
	})
}