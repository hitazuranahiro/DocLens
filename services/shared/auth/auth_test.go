package auth_test

import (
	"context"
	"errors"
	"testing"

	"github.com/tomeku/doclens/services/shared/auth"
	"github.com/tomeku/doclens/services/shared/auth/local"
)

func TestLocalAuthenticator_Verify(t *testing.T) {
	t.Parallel()

	a := local.New()
	ctx := context.Background()

	tests := []struct {
		name    string
		token   string
		wantErr error
		wantID  string
	}{
		{"valid", "dev:user_1:alice@example.com", nil, "user_1"},
		{"missing token", "", auth.ErrMissingToken, ""},
		{"wrong prefix", "prod:user_1:alice@example.com", auth.ErrInvalidToken, ""},
		{"too few parts", "dev:user_1", auth.ErrInvalidToken, ""},
		{"empty user", "dev::alice@example.com", auth.ErrInvalidToken, ""},
		{"empty email", "dev:user_1:", auth.ErrInvalidToken, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := a.Verify(ctx, tt.token)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("err = %v, want %v", err, tt.wantErr)
			}
			if id.UserID != tt.wantID {
				t.Fatalf("userID = %q, want %q", id.UserID, tt.wantID)
			}
		})
	}
}
