package backend

import (
	"testing"
)

func TestParseRef(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantType  BackendType
		wantPath  string
		wantErr   bool
	}{
		{
			name:     "ps prefix",
			input:    "ps:/app/prod/DB_HOST",
			wantType: BackendTypePS,
			wantPath: "/app/prod/DB_HOST",
		},
		{
			name:     "psa prefix",
			input:    "psa:/app/prod/DB_HOST",
			wantType: BackendTypePSA,
			wantPath: "/app/prod/DB_HOST",
		},
		{
			name:     "sm prefix",
			input:    "sm:my-secret",
			wantType: BackendTypeSM,
			wantPath: "my-secret",
		},
		{
			name:     "sm prefix with slash",
			input:    "sm:prod/db-password",
			wantType: BackendTypeSM,
			wantPath: "prod/db-password",
		},
		{
			name:    "empty ref",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no prefix",
			input:   "/app/prod/DB_HOST",
			wantErr: true,
		},
		{
			name:    "unknown prefix",
			input:   "xyz:/app/key",
			wantErr: true,
		},
		{
			name:    "ps prefix with no path",
			input:   "ps:",
			wantErr: true,
		},
		{
			name:    "sm prefix with no name",
			input:   "sm:",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ref, err := ParseRef(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRef(%q) expected error, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseRef(%q) unexpected error: %v", tt.input, err)
			}
			if ref.Type != tt.wantType {
				t.Errorf("ParseRef(%q).Type = %v, want %v", tt.input, ref.Type, tt.wantType)
			}
			if ref.Path != tt.wantPath {
				t.Errorf("ParseRef(%q).Path = %q, want %q", tt.input, ref.Path, tt.wantPath)
			}
		})
	}
}
