package memory

import (
	"context"
	"testing"

	"github.com/plexusone/omniagent/memscope"
)

// TestMemoryContext_ScopeAware verifies the memory skill resolves its operation
// context from a memory scope stamped on ctx (RMI-OMNIAGENT-114): a chat turn
// scopes tool-driven memory to TenantID=team, SubjectID="chat:<id>", while an
// explicit subject_id argument still wins and an unscoped call is unchanged.
func TestMemoryContext_ScopeAware(t *testing.T) {
	s := &Skill{tenantID: "personal", agentID: "helper"}

	cases := []struct {
		name        string
		scope       *memscope.Scope
		argSubject  string
		wantTenant  string
		wantSubject string
	}{
		{
			name:        "unscoped, no subject defaults to tenant",
			wantTenant:  "personal",
			wantSubject: "personal",
		},
		{
			name:        "unscoped, explicit subject",
			argSubject:  "u1",
			wantTenant:  "personal",
			wantSubject: "u1",
		},
		{
			name:        "chat scope sets tenant and subject",
			scope:       &memscope.Scope{TenantID: "team", SubjectID: "chat:1"},
			wantTenant:  "team",
			wantSubject: "chat:1",
		},
		{
			name:        "explicit subject wins over chat scope subject",
			scope:       &memscope.Scope{TenantID: "team", SubjectID: "chat:1"},
			argSubject:  "u1",
			wantTenant:  "team",
			wantSubject: "u1",
		},
		{
			name:        "subject-only scope keeps skill tenant",
			scope:       &memscope.Scope{SubjectID: "chat:9"},
			wantTenant:  "personal",
			wantSubject: "chat:9",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.scope != nil {
				ctx = memscope.NewContext(ctx, *tc.scope)
			}
			got := s.memoryContext(ctx, tc.argSubject)
			if got.TenantID != tc.wantTenant || got.SubjectID != tc.wantSubject {
				t.Errorf("memoryContext = {tenant:%q subject:%q}, want {tenant:%q subject:%q}",
					got.TenantID, got.SubjectID, tc.wantTenant, tc.wantSubject)
			}
			if got.AgentID != "helper" {
				t.Errorf("AgentID = %q, want %q", got.AgentID, "helper")
			}
		})
	}
}
