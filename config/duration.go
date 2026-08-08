package config

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

// Duration wraps time.Duration so config files can use human-readable
// duration strings ("4h", "30s") in both YAML and JSON. Plain integers are
// accepted as nanoseconds for backward compatibility.
type Duration time.Duration

// Duration returns the wrapped time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// String returns the canonical duration string.
func (d Duration) String() string {
	return time.Duration(d).String()
}

// MarshalJSON encodes the duration as a string.
func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.String())
}

// UnmarshalJSON decodes a duration string or integer nanoseconds.
func (d *Duration) UnmarshalJSON(b []byte) error {
	var raw any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	return d.set(raw)
}

// MarshalYAML encodes the duration as a string.
func (d Duration) MarshalYAML() (any, error) {
	return d.String(), nil
}

// UnmarshalYAML decodes a duration string or integer nanoseconds.
func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw any
	if err := value.Decode(&raw); err != nil {
		return err
	}
	return d.set(raw)
}

func (d *Duration) set(raw any) error {
	switch v := raw.(type) {
	case string:
		if v == "" {
			*d = 0
			return nil
		}
		parsed, err := time.ParseDuration(v)
		if err != nil {
			return fmt.Errorf("invalid duration %q: %w", v, err)
		}
		*d = Duration(parsed)
	case int:
		*d = Duration(v)
	case int64:
		*d = Duration(v)
	case float64:
		*d = Duration(v)
	default:
		return fmt.Errorf("invalid duration value %v (%T)", raw, raw)
	}
	return nil
}
