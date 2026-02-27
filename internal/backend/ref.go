package backend

import (
	"fmt"
	"strings"
)

// BackendType represents the type of AWS backend.
type BackendType string

const (
	BackendTypePS  BackendType = "ps"
	BackendTypePSA BackendType = "psa"
	BackendTypeSM  BackendType = "sm"
)

// Ref represents a parsed backend reference.
type Ref struct {
	Type BackendType
	Path string
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

	var bt BackendType
	switch prefix {
	case "ps":
		bt = BackendTypePS
	case "psa":
		bt = BackendTypePSA
	case "sm":
		bt = BackendTypeSM
	default:
		return Ref{}, fmt.Errorf("unknown backend prefix %q in ref %q", prefix, raw)
	}

	if path == "" {
		return Ref{}, fmt.Errorf("invalid ref %q: path is empty", raw)
	}

	return Ref{Type: bt, Path: path}, nil
}
