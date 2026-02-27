package backend

import (
	"context"
	"testing"

	"github.com/youyo/bundr/internal/tags"
)

func TestMockBackend_PutAndGet(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	// Put raw value
	err := mock.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Get raw value
	val, err := mock.Get(ctx, "ps:/app/test/KEY", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get() = %q, want %q", val, "hello")
	}
}

func TestMockBackend_PutJSON(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	// Put JSON scalar
	err := mock.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// Get without force flags should decode JSON
	val, err := mock.Get(ctx, "ps:/app/test/KEY", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	// JSON-encoded scalar "hello" is stored as "\"hello\"", decoded back to "hello"
	if val != "hello" {
		t.Errorf("Get() = %q, want %q", val, "hello")
	}
}

func TestMockBackend_GetForceRaw(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	err := mock.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     "hello",
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	// ForceRaw should return the raw stored value
	val, err := mock.Get(ctx, "ps:/app/test/KEY", GetOptions{ForceRaw: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	// JSON-encoded scalar "hello" stored as "\"hello\""
	if val != `"hello"` {
		t.Errorf("Get(ForceRaw) = %q, want %q", val, `"hello"`)
	}
}

func TestMockBackend_GetForceJSON(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	// Store raw but force JSON decode on get
	err := mock.Put(ctx, "ps:/app/test/KEY", PutOptions{
		Value:     `"hello"`,
		StoreMode: tags.StoreModeRaw,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	val, err := mock.Get(ctx, "ps:/app/test/KEY", GetOptions{ForceJSON: true})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != "hello" {
		t.Errorf("Get(ForceJSON) = %q, want %q", val, "hello")
	}
}

func TestMockBackend_GetNotFound(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	_, err := mock.Get(ctx, "ps:/nonexistent", GetOptions{})
	if err == nil {
		t.Error("Get() expected error for nonexistent key, got nil")
	}
}

func TestMockBackend_CallRecording(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	_ = mock.Put(ctx, "ps:/app/key1", PutOptions{Value: "v1", StoreMode: tags.StoreModeRaw})
	_ = mock.Put(ctx, "ps:/app/key2", PutOptions{Value: "v2", StoreMode: tags.StoreModeJSON})

	if len(mock.PutCalls) != 2 {
		t.Errorf("PutCalls count = %d, want 2", len(mock.PutCalls))
	}

	_, _ = mock.Get(ctx, "ps:/app/key1", GetOptions{})
	if len(mock.GetCalls) != 1 {
		t.Errorf("GetCalls count = %d, want 1", len(mock.GetCalls))
	}
}

func TestMockBackend_PutJSONObject(t *testing.T) {
	ctx := context.Background()
	mock := NewMockBackend()

	// JSON object should be stored as-is
	err := mock.Put(ctx, "ps:/app/test/OBJ", PutOptions{
		Value:     `{"key":"value"}`,
		StoreMode: tags.StoreModeJSON,
	})
	if err != nil {
		t.Fatalf("Put() error: %v", err)
	}

	val, err := mock.Get(ctx, "ps:/app/test/OBJ", GetOptions{})
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if val != `{"key":"value"}` {
		t.Errorf("Get() = %q, want %q", val, `{"key":"value"}`)
	}
}
