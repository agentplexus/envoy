package auth

import "testing"

func TestACL_IsAllowed(t *testing.T) {
	tests := []struct {
		name           string
		allowedEmails  []string
		allowedDomains []string
		email          string
		want           bool
	}{
		{
			name:  "no ACL - allow all",
			email: "anyone@anywhere.com",
			want:  true,
		},
		{
			name:          "exact email match",
			allowedEmails: []string{"user@example.com"},
			email:         "user@example.com",
			want:          true,
		},
		{
			name:          "exact email match - case insensitive",
			allowedEmails: []string{"User@Example.com"},
			email:         "user@example.com",
			want:          true,
		},
		{
			name:          "email not in list",
			allowedEmails: []string{"user@example.com"},
			email:         "other@example.com",
			want:          false,
		},
		{
			name:           "domain match",
			allowedDomains: []string{"@company.com"},
			email:          "user@company.com",
			want:           true,
		},
		{
			name:           "domain match without @",
			allowedDomains: []string{"company.com"},
			email:          "user@company.com",
			want:           true,
		},
		{
			name:           "domain match - case insensitive",
			allowedDomains: []string{"@Company.com"},
			email:          "user@company.com",
			want:           true,
		},
		{
			name:           "domain not in list",
			allowedDomains: []string{"@company.com"},
			email:          "user@other.com",
			want:           false,
		},
		{
			name:           "subdomain not matched",
			allowedDomains: []string{"@company.com"},
			email:          "user@sub.company.com",
			want:           false,
		},
		{
			name:           "multiple domains",
			allowedDomains: []string{"@company.com", "@corp.com"},
			email:          "user@corp.com",
			want:           true,
		},
		{
			name:           "email takes precedence",
			allowedEmails:  []string{"special@other.com"},
			allowedDomains: []string{"@company.com"},
			email:          "special@other.com",
			want:           true,
		},
		{
			name:           "mixed - email not matched but domain matched",
			allowedEmails:  []string{"admin@admin.com"},
			allowedDomains: []string{"@company.com"},
			email:          "user@company.com",
			want:           true,
		},
		{
			name:           "mixed - neither matched",
			allowedEmails:  []string{"admin@admin.com"},
			allowedDomains: []string{"@company.com"},
			email:          "user@other.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AllowedEmails:  tt.allowedEmails,
				AllowedDomains: tt.allowedDomains,
			}
			acl := NewACL(cfg)

			if got := acl.IsAllowed(tt.email); got != tt.want {
				t.Errorf("IsAllowed(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestACL_IsEmpty(t *testing.T) {
	tests := []struct {
		name           string
		allowedEmails  []string
		allowedDomains []string
		want           bool
	}{
		{
			name: "empty",
			want: true,
		},
		{
			name:          "has emails",
			allowedEmails: []string{"user@example.com"},
			want:          false,
		},
		{
			name:           "has domains",
			allowedDomains: []string{"@company.com"},
			want:           false,
		},
		{
			name:           "has both",
			allowedEmails:  []string{"user@example.com"},
			allowedDomains: []string{"@company.com"},
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				AllowedEmails:  tt.allowedEmails,
				AllowedDomains: tt.allowedDomains,
			}
			acl := NewACL(cfg)

			if got := acl.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}
