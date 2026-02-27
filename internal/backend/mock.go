package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/youyo/bundr/internal/tags"
)

// PutCall records a call to Put.
type PutCall struct {
	Ref  string
	Opts PutOptions
}

// GetCall records a call to Get.
type GetCall struct {
	Ref  string
	Opts GetOptions
}

type mockEntry struct {
	Value     string
	StoreMode string
	Tags      map[string]string
}

// MockBackend is an in-memory Backend implementation for testing.
type MockBackend struct {
	mu       sync.RWMutex
	store    map[string]mockEntry
	PutCalls []PutCall
	GetCalls []GetCall
}

// NewMockBackend creates a new MockBackend.
func NewMockBackend() *MockBackend {
	return &MockBackend{
		store: make(map[string]mockEntry),
	}
}

// Put stores a value in the in-memory store.
func (m *MockBackend) Put(_ context.Context, ref string, opts PutOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.PutCalls = append(m.PutCalls, PutCall{Ref: ref, Opts: opts})

	storedValue := opts.Value

	// JSON mode: encode scalar values
	if opts.StoreMode == tags.StoreModeJSON {
		if !json.Valid([]byte(opts.Value)) {
			// Scalar: JSON-encode it
			encoded, err := json.Marshal(opts.Value)
			if err != nil {
				return fmt.Errorf("json encode: %w", err)
			}
			storedValue = string(encoded)
		}
		// Valid JSON (objects, arrays, already-encoded strings) stored as-is
	}

	entryTags := tags.ManagedTags(opts.StoreMode)
	for k, v := range opts.Tags {
		entryTags[k] = v
	}

	m.store[ref] = mockEntry{
		Value:     storedValue,
		StoreMode: opts.StoreMode,
		Tags:      entryTags,
	}

	return nil
}

// Get retrieves a value from the in-memory store.
func (m *MockBackend) Get(_ context.Context, ref string, opts GetOptions) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.GetCalls = append(m.GetCalls, GetCall{Ref: ref, Opts: opts})

	entry, ok := m.store[ref]
	if !ok {
		return "", fmt.Errorf("key not found: %s", ref)
	}

	// ForceRaw: return stored value as-is
	if opts.ForceRaw {
		return entry.Value, nil
	}

	// ForceJSON or storeMode=json: decode JSON
	if opts.ForceJSON || entry.StoreMode == tags.StoreModeJSON {
		return decodeJSON(entry.Value)
	}

	return entry.Value, nil
}

// decodeJSON decodes a JSON-encoded value.
// If the value is a JSON string, it returns the unquoted string.
// Otherwise, it returns the value as-is (objects, arrays, etc.).
func decodeJSON(raw string) (string, error) {
	var s string
	if err := json.Unmarshal([]byte(raw), &s); err == nil {
		return s, nil
	}

	// Not a JSON string — return as-is (object, array, number, bool, null)
	if json.Valid([]byte(raw)) {
		return raw, nil
	}

	return "", fmt.Errorf("invalid JSON: %s", raw)
}
