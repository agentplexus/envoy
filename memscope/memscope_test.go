package memscope

import (
	"context"
	"testing"
)

func TestFromContext_Unset(t *testing.T) {
	if _, ok := FromContext(context.Background()); ok {
		t.Fatal("FromContext reported a scope on a bare context")
	}
}

func TestNewContext_EmptyScopeIsNoOp(t *testing.T) {
	ctx := NewContext(context.Background(), Scope{})
	if _, ok := FromContext(ctx); ok {
		t.Fatal("an entirely empty scope should not be stamped")
	}
}

func TestNewContextRoundTrip(t *testing.T) {
	want := Scope{TenantID: "team", SubjectID: "chat:abc"}
	ctx := NewContext(context.Background(), want)
	got, ok := FromContext(ctx)
	if !ok {
		t.Fatal("FromContext did not find the stamped scope")
	}
	if got != want {
		t.Errorf("round-trip = %+v, want %+v", got, want)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct {
		name                  string
		scope                 *Scope
		tenantIn, subjectIn   string
		tenantOut, subjectOut string
	}{
		{
			name:     "no scope keeps defaults",
			scope:    nil,
			tenantIn: "acme", subjectIn: "session:1",
			tenantOut: "acme", subjectOut: "session:1",
		},
		{
			name:     "full scope overrides both",
			scope:    &Scope{TenantID: "team", SubjectID: "chat:x"},
			tenantIn: "acme", subjectIn: "session:1",
			tenantOut: "team", subjectOut: "chat:x",
		},
		{
			name:     "subject-only scope keeps tenant default",
			scope:    &Scope{SubjectID: "chat:x"},
			tenantIn: "acme", subjectIn: "session:1",
			tenantOut: "acme", subjectOut: "chat:x",
		},
		{
			name:     "tenant-only scope keeps subject default",
			scope:    &Scope{TenantID: "team"},
			tenantIn: "acme", subjectIn: "session:1",
			tenantOut: "team", subjectOut: "session:1",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.scope != nil {
				ctx = NewContext(ctx, *tc.scope)
			}
			gotTenant, gotSubject := Resolve(ctx, tc.tenantIn, tc.subjectIn)
			if gotTenant != tc.tenantOut || gotSubject != tc.subjectOut {
				t.Errorf("Resolve = (%q, %q), want (%q, %q)",
					gotTenant, gotSubject, tc.tenantOut, tc.subjectOut)
			}
		})
	}
}
