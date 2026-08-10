package auth

import "testing"

func TestExtractGoogleIdentity(t *testing.T) {
	tests := []struct {
		name      string
		claims    googleClaims
		gotNonce  string
		wantNonce string
		wantErr   bool
		wantSub   string
		wantEmail string
	}{
		{
			name:      "happy path",
			claims:    googleClaims{Sub: "sub-1", Email: "user@example.com", EmailVerified: true},
			gotNonce:  "n1",
			wantNonce: "n1",
			wantSub:   "sub-1",
			wantEmail: "user@example.com",
		},
		{
			name:      "nonce mismatch",
			claims:    googleClaims{Sub: "sub-1", Email: "user@example.com", EmailVerified: true},
			gotNonce:  "n1",
			wantNonce: "n2",
			wantErr:   true,
		},
		{
			name:      "empty nonce",
			claims:    googleClaims{Sub: "sub-1", Email: "user@example.com", EmailVerified: true},
			gotNonce:  "",
			wantNonce: "",
			wantErr:   true,
		},
		{
			name:      "unverified email",
			claims:    googleClaims{Sub: "sub-1", Email: "user@example.com", EmailVerified: false},
			gotNonce:  "n1",
			wantNonce: "n1",
			wantErr:   true,
		},
		{
			name:      "missing sub",
			claims:    googleClaims{Email: "user@example.com", EmailVerified: true},
			gotNonce:  "n1",
			wantNonce: "n1",
			wantErr:   true,
		},
		{
			name:      "missing email",
			claims:    googleClaims{Sub: "sub-1", EmailVerified: true},
			gotNonce:  "n1",
			wantNonce: "n1",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub, email, err := extractGoogleIdentity(tt.claims, tt.gotNonce, tt.wantNonce)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sub != tt.wantSub || email != tt.wantEmail {
				t.Errorf("got (%q, %q), want (%q, %q)", sub, email, tt.wantSub, tt.wantEmail)
			}
		})
	}
}
