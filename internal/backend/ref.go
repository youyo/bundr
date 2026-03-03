package backend

import (
	"fmt"
	"strings"
)

// BackendType represents the type of AWS backend.
type BackendType string

const (
	BackendTypePS BackendType = "ps"
	BackendTypeSM BackendType = "sm"
)

// ValueType constants for PutOptions.ValueType.
const (
	ValueTypeString = "string"
	ValueTypeSecure = "secure"
)

// Ref represents a parsed backend reference.
type Ref struct {
	Type         BackendType
	Path         string
	AdvancedTier bool // true when psa: prefix was used; ensures Advanced tier on Put
}

// ParseRef parses a ref string (e.g. "ps:/app/key", "sm:secret-name") into a Ref.
func ParseRef(raw string) (Ref, error) {
	if raw == "" {
		return Ref{}, fmt.Errorf("empty ref")
	}

	idx := strings.Index(raw, ":")
	if idx < 0 {
		return Ref{}, fmt.Errorf("invalid ref %q: missing prefix (expected ps:, psa:, or sm:)", raw)
	}

	prefix := raw[:idx]
	path := raw[idx+1:]

	switch prefix {
	case "ps":
		if path == "" {
			return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
		}
		return Ref{Type: BackendTypePS, Path: path}, nil
	case "psa":
		// psa: is an alias for ps: with Advanced tier; normalize to BackendTypePS
		if path == "" {
			return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
		}
		return Ref{Type: BackendTypePS, Path: path, AdvancedTier: true}, nil
	case "sm":
		if path == "" {
			return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
		}
		return Ref{Type: BackendTypeSM, Path: path}, nil
	default:
		return Ref{}, fmt.Errorf("unknown backend prefix %q in ref %q", prefix, raw)
	}
}
