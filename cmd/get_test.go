package cmd

import (
	"bytes"
	"context"
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
