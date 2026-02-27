package cmd

import (
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
	"github.com/youyo/bundr/internal/tags"
)

func TestPutCmd_RunRaw(t *testing.T) {
	mock := backend.NewMockBackend()
	factory := func(_ backend.BackendType) (backend.Backend, error) {
		return mock, nil
	}

	cmd := &PutCmd{
		Ref:   "ps:/app/test/KEY",
		Value: "hello",
		Store: tags.StoreModeRaw,
	}

	appCtx := &Context{
		Config:         &config.Config{},
		BackendFactory: factory,
	}

	err := cmd.Run(appCtx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(mock.PutCalls) != 1 {
		t.Fatalf("expected 1 PutCall, got %d", len(mock.PutCalls))
	}

	call := mock.PutCalls[0]
	if call.Ref != "ps:/app/test/KEY" {
		t.Errorf("PutCall ref = %q, want %q", call.Ref, "ps:/app/test/KEY")
	}
	if call.Opts.Value != "hello" {
		t.Errorf("PutCall value = %q, want %q", call.Opts.Value, "hello")
	}
	if call.Opts.StoreMode != tags.StoreModeRaw {
		t.Errorf("PutCall storeMode = %q, want %q", call.Opts.StoreMode, tags.StoreModeRaw)
	}
}

func TestPutCmd_RunJSON(t *testing.T) {
	mock := backend.NewMockBackend()
	factory := func(_ backend.BackendType) (backend.Backend, error) {
		return mock, nil
	}

	cmd := &PutCmd{
		Ref:   "ps:/app/test/KEY",
		Value: "hello",
		Store: tags.StoreModeJSON,
	}

	appCtx := &Context{
		Config:         &config.Config{},
		BackendFactory: factory,
	}

	err := cmd.Run(appCtx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if len(mock.PutCalls) != 1 {
		t.Fatalf("expected 1 PutCall, got %d", len(mock.PutCalls))
	}

	call := mock.PutCalls[0]
	if call.Opts.StoreMode != tags.StoreModeJSON {
		t.Errorf("PutCall storeMode = %q, want %q", call.Opts.StoreMode, tags.StoreModeJSON)
	}
}

func TestPutCmd_RunSecure(t *testing.T) {
	mock := backend.NewMockBackend()
	factory := func(_ backend.BackendType) (backend.Backend, error) {
		return mock, nil
	}

	cmd := &PutCmd{
		Ref:    "ps:/app/test/SECRET",
		Value:  "secret-value",
		Store:  tags.StoreModeRaw,
		Secure: true,
	}

	appCtx := &Context{
		Config:         &config.Config{},
		BackendFactory: factory,
	}

	err := cmd.Run(appCtx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	call := mock.PutCalls[0]
	if call.Opts.ValueType != "secure" {
		t.Errorf("PutCall valueType = %q, want %q", call.Opts.ValueType, "secure")
	}
}

func TestPutCmd_RunSM(t *testing.T) {
	mock := backend.NewMockBackend()
	var requestedType backend.BackendType
	factory := func(bt backend.BackendType) (backend.Backend, error) {
		requestedType = bt
		return mock, nil
	}

	cmd := &PutCmd{
		Ref:   "sm:my-secret",
		Value: "secret-value",
		Store: tags.StoreModeRaw,
	}

	appCtx := &Context{
		Config:         &config.Config{},
		BackendFactory: factory,
	}

	err := cmd.Run(appCtx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if requestedType != backend.BackendTypeSM {
		t.Errorf("BackendFactory received type %v, want %v", requestedType, backend.BackendTypeSM)
	}
}

func TestPutCmd_RunInvalidRef(t *testing.T) {
	factory := func(_ backend.BackendType) (backend.Backend, error) {
		return backend.NewMockBackend(), nil
	}

	cmd := &PutCmd{
		Ref:   "invalid",
		Value: "hello",
		Store: tags.StoreModeRaw,
	}

	appCtx := &Context{
		Config:         &config.Config{},
		BackendFactory: factory,
	}

	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error for invalid ref, got nil")
	}
}
