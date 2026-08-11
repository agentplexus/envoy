package skills

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// RequirementError describes a missing requirement.
type RequirementError struct {
	Type    string      // "binary" or "env"
	Name    string      // Name of the missing requirement
	Skill   string      // Skill that requires it
	Install []Installer // How to fix (for binaries)
}

func (e *RequirementError) Error() string {
	return fmt.Sprintf("skill %q requires %s %q", e.Skill, e.Type, e.Name)
}

// InstallHint returns a human-readable install suggestion.
func (e *RequirementError) InstallHint() string {
	if len(e.Install) == 0 {
		return ""
	}

	var hints []string
	for _, inst := range e.Install {
		switch inst.Kind {
		case "brew":
			hints = append(hints, fmt.Sprintf("brew install %s", inst.Formula))
		case "apt":
			hints = append(hints, fmt.Sprintf("apt install %s", inst.Package))
		case "go":
			hints = append(hints, fmt.Sprintf("go install %s", inst.Module))
		case "npm":
			hints = append(hints, fmt.Sprintf("npm install -g %s", inst.Package))
		default:
			if inst.Label != "" {
				hints = append(hints, inst.Label)
			}
		}
	}

	if len(hints) == 0 {
		return ""
	}
	return strings.Join(hints, " or ")
}

// UnmetRequiredSecrets returns the declared secrets a skill marks required
// whose env-var name is absent from available (the resolved secret env, e.g.
// an agent's set secrets), with an actionable message per entry. A skill with
// unmet required secrets should be treated as unavailable — running it would
// hit the API with a missing credential (INIT-OMNIAGENT-004). Optional
// (non-required) declared secrets never gate availability.
func (s *Skill) UnmetRequiredSecrets(available map[string]string) []*RequirementError {
	var errs []*RequirementError
	for _, sec := range s.DeclaredSecrets() {
		if !sec.Required {
			continue
		}
		if _, ok := available[sec.EnvVar()]; !ok {
			errs = append(errs, &RequirementError{
				Type:  "secret",
				Name:  sec.EnvVar(),
				Skill: s.Name,
			})
		}
	}
	return errs
}

// CheckRequirements verifies all skill prerequisites.
// Returns a slice of errors for missing requirements.
func (s *Skill) CheckRequirements() []error {
	var errs []error

	if s.Metadata.OpenClaw == nil || s.Metadata.OpenClaw.Requires == nil {
		return nil
	}

	req := s.Metadata.OpenClaw.Requires

	// Check required binaries
	for _, bin := range req.Bins {
		if _, err := exec.LookPath(bin); err != nil {
			errs = append(errs, &RequirementError{
				Type:    "binary",
				Name:    bin,
				Skill:   s.Name,
				Install: s.Metadata.OpenClaw.Install,
			})
		}
	}

	// Check anyBins (at least one must exist)
	if len(req.AnyBins) > 0 {
		found := false
		for _, bin := range req.AnyBins {
			if _, err := exec.LookPath(bin); err == nil {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, &RequirementError{
				Type:    "binary (any of)",
				Name:    strings.Join(req.AnyBins, ", "),
				Skill:   s.Name,
				Install: s.Metadata.OpenClaw.Install,
			})
		}
	}

	// Check required env vars
	for _, env := range req.Env {
		if os.Getenv(env) == "" {
			errs = append(errs, &RequirementError{
				Type:  "environment variable",
				Name:  env,
				Skill: s.Name,
			})
		}
	}

	return errs
}
