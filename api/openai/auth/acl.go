package auth

import "strings"

// ACL handles email-based access control.
type ACL struct {
	allowedEmails  map[string]struct{}
	allowedDomains []string
}

// NewACL creates a new ACL from the configuration.
func NewACL(cfg *Config) *ACL {
	acl := &ACL{
		allowedEmails:  make(map[string]struct{}),
		allowedDomains: make([]string, 0),
	}

	// Build email lookup map (case-insensitive)
	for _, email := range cfg.AllowedEmails {
		acl.allowedEmails[strings.ToLower(email)] = struct{}{}
	}

	// Store domains (ensure they start with @)
	for _, domain := range cfg.AllowedDomains {
		d := strings.ToLower(domain)
		if !strings.HasPrefix(d, "@") {
			d = "@" + d
		}
		acl.allowedDomains = append(acl.allowedDomains, d)
	}

	return acl
}

// IsAllowed checks if an email address is allowed by the ACL.
// If no ACL rules are configured, all authenticated users are allowed.
func (a *ACL) IsAllowed(email string) bool {
	// If no ACL is configured, allow all authenticated users
	if len(a.allowedEmails) == 0 && len(a.allowedDomains) == 0 {
		return true
	}

	email = strings.ToLower(email)

	// Check exact email match
	if _, ok := a.allowedEmails[email]; ok {
		return true
	}

	// Check domain match
	for _, domain := range a.allowedDomains {
		if strings.HasSuffix(email, domain) {
			return true
		}
	}

	return false
}

// IsEmpty returns true if no ACL rules are configured.
func (a *ACL) IsEmpty() bool {
	return len(a.allowedEmails) == 0 && len(a.allowedDomains) == 0
}
