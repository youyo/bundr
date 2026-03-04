package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/tags"
)

func TestGetCmd_JSONMode(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	// Set up: put a JSON-encoded value
	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/test/KEY"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// JSON mode should decode "\"hello\"" → "hello"
	if output != "hello\n" {
		t.Errorf("output = %q, want %q", output, "hello\n")
	}
}

func TestGetCmd_RawMode(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     "plain-text",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/test/KEY"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	if output != "plain-text\n" {
		t.Errorf("output = %q, want %q", output, "plain-text\n")
	}
}

func TestGetCmd_ForceRawFlag(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	// Put JSON-encoded value
	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/test/KEY", Raw: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// --raw should return the raw stored value: "\"hello\""
	if output != "\"hello\"\n" {
		t.Errorf("output = %q, want %q", output, "\"hello\"\n")
	}
}

func TestGetCmd_ForceJSONFlag(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	// Put raw but JSON-parseable value
	_ = mock.Put(ctx, "ps:/app/test/KEY", backend.PutOptions{
		Value:     `"hello"`,
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/test/KEY", JSON: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// --json should decode regardless of tag
	if output != "hello\n" {
		t.Errorf("output = %q, want %q", output, "hello\n")
	}
}

func TestGetCmd_SMRef(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "sm:my-secret", backend.PutOptions{
		Value:     "secret-value",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(refType backend.BackendType) (backend.Backend, error) {
			if refType != backend.BackendTypeSM {
				t.Errorf("expected BackendTypeSM, got %v", refType)
			}
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "sm:my-secret"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	if output != "secret-value\n" {
		t.Errorf("output = %q, want %q", output, "secret-value\n")
	}
}

func TestGetCmd_InvalidRef(t *testing.T) {
	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			t.Fatal("BackendFactory should not be called for invalid ref")
			return nil, nil
		},
	}

	cmd := &GetCmd{Ref: "invalid-ref"}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error for invalid ref, got nil")
	}
}

// GET-D-01: get --describe outputs valid JSON with metadata
func TestGetCmd_Describe_PS(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/db_host", backend.PutOptions{
		Value:     "localhost",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/db_host", Describe: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// Output must be valid JSON
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	// MockBackend.Describe returns tags + Value
	if result["Value"] != "localhost" {
		t.Errorf("Value = %v, want %q", result["Value"], "localhost")
	}
}

// GET-D-02: get --describe with SM ref outputs valid JSON
func TestGetCmd_Describe_SM(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "sm:myapp/db", backend.PutOptions{
		Value:     "mysecret",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "sm:myapp/db", Describe: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}
	if result["Value"] != "mysecret" {
		t.Errorf("Value = %v, want %q", result["Value"], "mysecret")
	}
}

// GET-D-EDGE-01: --describe takes precedence over --raw
func TestGetCmd_Describe_TakesPrecedenceOverRaw(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/key", backend.PutOptions{
		Value:     "val",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	// Both --describe and --raw: describe wins
	cmd := &GetCmd{Ref: "ps:/app/key", Describe: true, Raw: true}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	// Should be JSON (describe wins)
	var result map[string]any
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output should be JSON when --describe is set, got: %s", output)
	}
}

// TestGetCmd_Prefix tests get with trailing / to fetch multiple keys as JSON map.
func TestGetCmd_Prefix(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	_ = mock.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{
		Value:     "localhost",
		StoreMode: tags.StoreModeRaw,
	})
	_ = mock.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{
		Value:     "5432",
		StoreMode: tags.StoreModeRaw,
	})

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/app/prod/"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if result["DB_HOST"] != "localhost" {
		t.Errorf("DB_HOST = %q, want %q", result["DB_HOST"], "localhost")
	}
	if result["DB_PORT"] != "5432" {
		t.Errorf("DB_PORT = %q, want %q", result["DB_PORT"], "5432")
	}
}

// TestGetCmd_PrefixEmpty tests get with trailing / and no matching entries.
func TestGetCmd_PrefixEmpty(t *testing.T) {
	mock := backend.NewMockBackend()

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
	}

	cmd := &GetCmd{Ref: "ps:/empty/prefix/"}

	output := captureStdout(t, func() {
		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("Run() error: %v", err)
		}
	})

	var result map[string]string
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, output)
	}

	if len(result) != 0 {
		t.Errorf("expected empty map, got %v", result)
	}
}

// captureStdout captures stdout output during the execution of fn.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("io.Copy() error: %v", err)
	}
	return buf.String()
}
