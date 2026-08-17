package compiled_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnivault/providers/memory"
	"github.com/plexusone/omnivault/vault"

	"github.com/plexusone/omniagent/internal/redact"
	"github.com/plexusone/omniagent/skills/compiled"
	"github.com/plexusone/omniagent/skills/remote/mcp"
	"github.com/plexusone/omniagent/skills/remote/openapi"
)

// fakeSkill is a minimal compiled.Skill + compiled.SecretsAware, standing in
// for a hand-written (non-MCP, non-OpenAPI) compiled skill.
type fakeSkill struct {
	name string
	env  map[string]string
}

func (s *fakeSkill) Name() string                     { return s.name }
func (s *fakeSkill) Description() string              { return "fake skill for redaction testing" }
func (s *fakeSkill) Tools() []skill.Tool              { return nil }
func (s *fakeSkill) Init(context.Context) error       { return nil }
func (s *fakeSkill) Close() error                     { return nil }
func (s *fakeSkill) SetSecrets(env map[string]string) { s.env = env }

var (
	_ compiled.Skill        = (*fakeSkill)(nil)
	_ compiled.SecretsAware = (*fakeSkill)(nil)
)

// resolveFromFakeVault stores and fetches a secret through an in-memory
// omnivault.Vault, mirroring how a real skill secret reaches an env map —
// the "fake vault" the acceptance criteria calls for.
func resolveFromFakeVault(t *testing.T, name, value string) string {
	t.Helper()
	v := memory.New()
	ctx := context.Background()
	if err := v.Set(ctx, name, &vault.Secret{Value: value}); err != nil {
		t.Fatalf("fake vault Set: %v", err)
	}
	sec, err := v.Get(ctx, name)
	if err != nil {
		t.Fatalf("fake vault Get: %v", err)
	}
	return sec.Value
}

// assertNeverLeaks feeds secret (already redact.Register'd by the caller)
// through a %+v dump of dump — the realistic accidental-leak path, since
// fmt's reflection-based formatting prints unexported struct fields — and
// confirms a redacted logger never lets the raw value through.
func assertNeverLeaks(t *testing.T, label string, secret string, dump any) {
	t.Helper()

	var buf bytes.Buffer
	logger := slog.New(redact.NewHandler(slog.NewTextHandler(&buf, nil)))
	logger.Info("accidental debug dump", "skill", fmt.Sprintf("%+v", dump))

	out := buf.String()
	if strings.Contains(out, secret) {
		t.Errorf("%s: log output leaked the secret value: %s", label, out)
	}
}

func TestCrossTypeRedaction_MCPSkill(t *testing.T) {
	t.Cleanup(redact.Reset)
	secret := resolveFromFakeVault(t, "GITHUB_TOKEN", "ghp_fakeTokenValueForTest1234")
	redact.Register(secret)

	sk := mcp.NewSkill(mcp.Config{
		Name:    "github",
		Command: []string{"echo", "unused"},
	})
	sk.SetSecrets(map[string]string{"GITHUB_TOKEN": secret})

	assertNeverLeaks(t, "mcp.Skill", secret, sk)
}

func TestCrossTypeRedaction_OpenAPISkill(t *testing.T) {
	t.Cleanup(redact.Reset)
	secret := resolveFromFakeVault(t, "API_TOKEN", "sk-fakeApiTokenForTest5678")
	redact.Register(secret)

	sk := openapi.NewSkill(openapi.Config{
		Name:     "petstore",
		SpecFile: "unused.json",
		Auth: openapi.AuthConfig{
			Type:     openapi.AuthBearer,
			TokenEnv: "API_TOKEN",
		},
	})
	sk.SetSecrets(map[string]string{"API_TOKEN": secret})

	assertNeverLeaks(t, "openapi.Skill", secret, sk)
}

func TestCrossTypeRedaction_CompiledSkill(t *testing.T) {
	t.Cleanup(redact.Reset)
	secret := resolveFromFakeVault(t, "WEATHER_KEY", "wk-fakeWeatherKeyForTest9012")
	redact.Register(secret)

	sk := &fakeSkill{name: "weather"}
	sk.SetSecrets(map[string]string{"WEATHER_KEY": secret})

	assertNeverLeaks(t, "compiled.Skill", secret, sk)
}

// TestCrossTypeRedaction_PublicSurfaceNeverExposesValue covers the
// "absent from prompt/transcript" half of the acceptance criteria: the only
// thing that ever reaches an LLM prompt from a compiled.Skill is
// compiled.Info's Name/Description/Tools — none of which are derived from
// injected secrets, so they must never contain one.
func TestCrossTypeRedaction_PublicSurfaceNeverExposesValue(t *testing.T) {
	t.Cleanup(redact.Reset)
	secret := resolveFromFakeVault(t, "WEATHER_KEY", "wk-fakePromptSurfaceTest3456")
	redact.Register(secret)

	sk := &fakeSkill{name: "weather"}
	sk.SetSecrets(map[string]string{"WEATHER_KEY": secret})

	info := compiled.Info(sk)
	if strings.Contains(info.Name, secret) || strings.Contains(info.Description, secret) {
		t.Fatalf("compiled.Info leaked the secret: %+v", info)
	}
	for _, tool := range info.Tools {
		if strings.Contains(tool.Name, secret) || strings.Contains(tool.Description, secret) {
			t.Fatalf("tool info leaked the secret: %+v", tool)
		}
	}
}
