package cmd

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/tags"
)

func TestDescribeCmd_PSRef(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{
		Value:     "localhost",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(refType backend.BackendType) (backend.Backend, error) {
			if refType != backend.BackendTypePS {
				t.Errorf("expected BackendTypePS, got %v", refType)
			}
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "ps:/app/prod/DB_HOST"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// Verify key fields are present
	if !strings.Contains(output, "ps:/app/prod/DB_HOST") {
		t.Errorf("output should contain ref, got:\n%s", output)
	}
	if !strings.Contains(output, "/app/prod/DB_HOST") {
		t.Errorf("output should contain path, got:\n%s", output)
	}
	if !strings.Contains(output, "Parameter Store (Standard)") {
		t.Errorf("output should contain backend label, got:\n%s", output)
	}
	if !strings.Contains(output, "String") {
		t.Errorf("output should contain parameter type, got:\n%s", output)
	}
	if !strings.Contains(output, "Standard") {
		t.Errorf("output should contain tier, got:\n%s", output)
	}
	if !strings.Contains(output, tags.TagCLIValue) {
		t.Errorf("output should contain tag value 'bundr', got:\n%s", output)
	}
}

func TestDescribeCmd_PSARef(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "psa:/app/prod/BIG_VALUE", backend.PutOptions{
		Value:     "large-data",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(refType backend.BackendType) (backend.Backend, error) {
			if refType != backend.BackendTypePSA {
				t.Errorf("expected BackendTypePSA, got %v", refType)
			}
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "psa:/app/prod/BIG_VALUE"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	if !strings.Contains(output, "Parameter Store (Advanced)") {
		t.Errorf("output should contain 'Parameter Store (Advanced)', got:\n%s", output)
	}
	if !strings.Contains(output, "Advanced") {
		t.Errorf("output should contain tier 'Advanced', got:\n%s", output)
	}
}

func TestDescribeCmd_SMRef(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "sm:my-secret", backend.PutOptions{
		Value:     "secret-value",
		StoreMode: tags.StoreModeJSON,
	})

	appCtx := &Context{
		BackendFactory: func(refType backend.BackendType) (backend.Backend, error) {
			if refType != backend.BackendTypeSM {
				t.Errorf("expected BackendTypeSM, got %v", refType)
			}
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "sm:my-secret"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	if !strings.Contains(output, "sm:my-secret") {
		t.Errorf("output should contain ref, got:\n%s", output)
	}
	if !strings.Contains(output, "Secrets Manager") {
		t.Errorf("output should contain backend label, got:\n%s", output)
	}
	if !strings.Contains(output, tags.StoreModeJSON) {
		t.Errorf("output should contain store mode tag 'json', got:\n%s", output)
	}
}

func TestDescribeCmd_JSONOutput(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "ps:/app/test/KEY", JSON: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// Verify it's valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if result["ref"] != "ps:/app/test/KEY" {
		t.Errorf("JSON ref = %v, want %q", result["ref"], "ps:/app/test/KEY")
	}
	if result["path"] != "/app/test/KEY" {
		t.Errorf("JSON path = %v, want %q", result["path"], "/app/test/KEY")
	}
	if result["backend"] != "Parameter Store (Standard)" {
		t.Errorf("JSON backend = %v, want %q", result["backend"], "Parameter Store (Standard)")
	}
}

func TestDescribeCmd_InvalidRef(t *testing.T) {
	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			t.Fatal("BackendFactory should not be called for invalid ref")
			return nil, nil
		},
	}

	cmd := &DescribeCmd{Ref: "invalid-ref"}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error for invalid ref, got nil")
	}
}

func TestDescribeCmd_NotFound(t *testing.T) {
	mock := backend.NewMockBackend()

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "ps:/nonexistent/key"}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error for nonexistent key, got nil")
	}
	if !strings.Contains(err.Error(), "describe command failed") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "describe command failed")
	}
}

func TestDescribeCmd_TagsSorted(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &DescribeCmd{Ref: "ps:/app/test/KEY"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// Tags should appear in sorted order: cli, cli-schema, cli-store-mode
	cliIdx := strings.Index(output, "cli ")
	schemaIdx := strings.Index(output, "cli-schema")
	storeModeIdx := strings.Index(output, "cli-store-mode")

	if cliIdx < 0 || schemaIdx < 0 || storeModeIdx < 0 {
		t.Fatalf("expected all 3 managed tags in output, got:\n%s", output)
	}
	if !(cliIdx < schemaIdx && schemaIdx < storeModeIdx) {
		t.Errorf("tags should be sorted: cli(%d) < cli-schema(%d) < cli-store-mode(%d)", cliIdx, schemaIdx, storeModeIdx)
	}
}
