package compiled

import (
	"context"
	"testing"

	"github.com/plexusone/omniskill/skill"
	"github.com/plexusone/omnistorage"
)

// mockSkill is a test skill implementation.
type mockSkill struct {
	name        string
	description string
	tools       []skill.Tool
	initCalled  bool
	closeCalled bool
	storage     omnistorage.Store
}

func newMockSkill(name string) *mockSkill {
	return &mockSkill{
		name:        name,
		description: "Mock skill for testing",
		tools: []skill.Tool{
			skill.NewTool(
				name+"_tool",
				"A mock tool",
				map[string]skill.Parameter{
					"input": {
						Type:        "string",
						Description: "Input value",
						Required:    true,
					},
				},
				func(ctx context.Context, params map[string]any) (any, error) {
					return map[string]any{"result": "ok"}, nil
				},
			),
		},
	}
}

func (m *mockSkill) Name() string                   { return m.name }
func (m *mockSkill) Description() string            { return m.description }
func (m *mockSkill) Tools() []skill.Tool            { return m.tools }
func (m *mockSkill) Init(ctx context.Context) error { m.initCalled = true; return nil }
func (m *mockSkill) Close() error                   { m.closeCalled = true; return nil }
func (m *mockSkill) SetStorage(s omnistorage.Store) { m.storage = s }

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	skill1 := newMockSkill("skill1")
	if err := r.Register(skill1); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	// Duplicate registration should fail
	if err := r.Register(skill1); err == nil {
		t.Error("Register() should fail for duplicate skill")
	}

	if r.Count() != 1 {
		t.Errorf("Count() = %d, want 1", r.Count())
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	skill := newMockSkill("test")
	if err := r.Register(skill); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, ok := r.Get("test")
	if !ok {
		t.Fatal("Get() should find registered skill")
	}
	if got.Name() != "test" {
		t.Errorf("Get() name = %q, want %q", got.Name(), "test")
	}

	_, ok = r.Get("nonexistent")
	if ok {
		t.Error("Get() should not find nonexistent skill")
	}
}

func TestRegistry_AllTools(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(newMockSkill("skill1")); err != nil {
		t.Fatalf("Register(skill1) error = %v", err)
	}
	if err := r.Register(newMockSkill("skill2")); err != nil {
		t.Fatalf("Register(skill2) error = %v", err)
	}

	tools := r.AllTools()
	if len(tools) != 2 {
		t.Errorf("AllTools() len = %d, want 2", len(tools))
	}
}

func TestRegistry_FindTool(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(newMockSkill("skill1")); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	tool, sk, ok := r.FindTool("skill1_tool")
	if !ok {
		t.Fatal("FindTool() should find registered tool")
	}
	if tool.Name() != "skill1_tool" {
		t.Errorf("FindTool() tool name = %q, want %q", tool.Name(), "skill1_tool")
	}
	if sk.Name() != "skill1" {
		t.Errorf("FindTool() skill name = %q, want %q", sk.Name(), "skill1")
	}

	_, _, ok = r.FindTool("nonexistent")
	if ok {
		t.Error("FindTool() should not find nonexistent tool")
	}
}

func TestRegistry_InitAll(t *testing.T) {
	r := NewRegistry()
	skill1 := newMockSkill("skill1")
	skill2 := newMockSkill("skill2")
	if err := r.Register(skill1); err != nil {
		t.Fatalf("Register(skill1) error = %v", err)
	}
	if err := r.Register(skill2); err != nil {
		t.Fatalf("Register(skill2) error = %v", err)
	}

	if err := r.InitAll(context.Background()); err != nil {
		t.Fatalf("InitAll() error = %v", err)
	}

	if !skill1.initCalled {
		t.Error("skill1 Init() not called")
	}
	if !skill2.initCalled {
		t.Error("skill2 Init() not called")
	}
}

func TestRegistry_CloseAll(t *testing.T) {
	r := NewRegistry()
	skill1 := newMockSkill("skill1")
	skill2 := newMockSkill("skill2")
	if err := r.Register(skill1); err != nil {
		t.Fatalf("Register(skill1) error = %v", err)
	}
	if err := r.Register(skill2); err != nil {
		t.Fatalf("Register(skill2) error = %v", err)
	}

	if err := r.CloseAll(); err != nil {
		t.Fatalf("CloseAll() error = %v", err)
	}

	if !skill1.closeCalled {
		t.Error("skill1 Close() not called")
	}
	if !skill2.closeCalled {
		t.Error("skill2 Close() not called")
	}
}

func TestTool_ToJSONSchema(t *testing.T) {
	tool := skill.NewTool(
		"test_tool",
		"A test tool",
		map[string]skill.Parameter{
			"required_param": {
				Type:        "string",
				Description: "A required parameter",
				Required:    true,
			},
			"optional_param": {
				Type:        "integer",
				Description: "An optional parameter",
				Required:    false,
				Default:     42,
			},
		},
		nil,
	)

	schema := tool.ToJSONSchema()

	if schema["type"] != "object" {
		t.Errorf("schema type = %v, want object", schema["type"])
	}

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}

	if len(props) != 2 {
		t.Errorf("schema properties count = %d, want 2", len(props))
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("schema should have required array")
	}

	if len(required) != 1 || required[0] != "required_param" {
		t.Errorf("schema required = %v, want [required_param]", required)
	}
}
