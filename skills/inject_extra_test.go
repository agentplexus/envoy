package skills

import "testing"

func TestFilterByName(t *testing.T) {
	skills := []*Skill{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}

	got := FilterByName(skills, []string{"beta", "gamma", "not-present"})
	if len(got) != 2 {
		t.Fatalf("FilterByName() = %+v, want 2 entries", got)
	}
	names := map[string]bool{}
	for _, s := range got {
		names[s.Name] = true
	}
	if !names["beta"] || !names["gamma"] {
		t.Errorf("FilterByName() = %v, want beta+gamma", names)
	}

	if got := FilterByName(skills, nil); len(got) != 0 {
		t.Errorf("FilterByName(nil names) = %+v, want empty", got)
	}
}

func TestExcludeByName(t *testing.T) {
	skills := []*Skill{
		{Name: "alpha"},
		{Name: "beta"},
		{Name: "gamma"},
	}

	got := ExcludeByName(skills, []string{"beta"})
	if len(got) != 2 {
		t.Fatalf("ExcludeByName() = %+v, want 2 entries", got)
	}
	for _, s := range got {
		if s.Name == "beta" {
			t.Errorf("ExcludeByName() kept excluded skill %q", s.Name)
		}
	}

	// No names to exclude: everything passes through.
	if got := ExcludeByName(skills, nil); len(got) != 3 {
		t.Errorf("ExcludeByName(nil names) = %+v, want all 3 skills", got)
	}
}
