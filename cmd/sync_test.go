package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
	"github.com/youyo/bundr/internal/tags"
)

func newSyncTestContext(t *testing.T) (*backend.MockBackend, *Context) {
	t.Helper()
	mb := backend.NewMockBackend()
	return mb, &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
	}
}

func TestSyncCmd_DotenvToPS_JSON(t *testing.T) {
	// .env file → PS JSON一括 (--to ps:/app/config)
	mb, appCtx := newSyncTestContext(t)
	_ = mb // pre-setup not needed for this direction

	tmpFile := writeTempEnv(t, "DB_HOST=localhost\nDB_PORT=5432\n")

	cmd := &SyncCmd{From: tmpFile, To: "ps:/app/config"}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mb.PutCalls) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(mb.PutCalls))
	}
	call := mb.PutCalls[0]
	if call.Ref != "ps:/app/config" {
		t.Errorf("ref = %q, want %q", call.Ref, "ps:/app/config")
	}
	if call.Opts.StoreMode != tags.StoreModeJSON {
		t.Errorf("storeMode = %q, want %q", call.Opts.StoreMode, tags.StoreModeJSON)
	}
	// Value should be valid JSON containing both keys
	if !strings.Contains(call.Opts.Value, "DB_HOST") || !strings.Contains(call.Opts.Value, "DB_PORT") {
		t.Errorf("value = %q, expected to contain DB_HOST and DB_PORT", call.Opts.Value)
	}
}

func TestSyncCmd_DotenvToPS_Flat(t *testing.T) {
	// .env file → PS flat (--to ps:/app/)
	mb, appCtx := newSyncTestContext(t)

	tmpFile := writeTempEnv(t, "DB_HOST=localhost\nDB_PORT=5432\n")

	cmd := &SyncCmd{From: tmpFile, To: "ps:/app/"}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mb.PutCalls) != 2 {
		t.Fatalf("expected 2 Put calls, got %d", len(mb.PutCalls))
	}

	// Keys should be lowercased
	refs := make(map[string]string)
	for _, call := range mb.PutCalls {
		refs[call.Ref] = call.Opts.Value
	}
	if v, ok := refs["ps:/app/db_host"]; !ok || v != "localhost" {
		t.Errorf("expected ps:/app/db_host=localhost, got %v=%v", ok, v)
	}
	if v, ok := refs["ps:/app/db_port"]; !ok || v != "5432" {
		t.Errorf("expected ps:/app/db_port=5432, got %v=%v", ok, v)
	}
}

func TestSyncCmd_DotenvToSM(t *testing.T) {
	// .env file → SM (--to sm:myapp)
	mb, appCtx := newSyncTestContext(t)

	tmpFile := writeTempEnv(t, "API_KEY=secret123\n")

	cmd := &SyncCmd{From: tmpFile, To: "sm:myapp"}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mb.PutCalls) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(mb.PutCalls))
	}
	call := mb.PutCalls[0]
	if call.Ref != "sm:myapp" {
		t.Errorf("ref = %q, want %q", call.Ref, "sm:myapp")
	}
	if call.Opts.StoreMode != tags.StoreModeJSON {
		t.Errorf("storeMode = %q, want %q", call.Opts.StoreMode, tags.StoreModeJSON)
	}
}

func TestSyncCmd_PS_JSON_ToStdout(t *testing.T) {
	// PS JSON value → stdout (expand)
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{
		Value:     `{"DB_HOST":"localhost","DB_PORT":"5432"}`,
		StoreMode: tags.StoreModeJSON,
	})

	// Capture stdout
	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/config", To: "-"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "DB_HOST=localhost") {
		t.Errorf("output should contain DB_HOST=localhost, got %q", output)
	}
	if !strings.Contains(output, "DB_PORT=5432") {
		t.Errorf("output should contain DB_PORT=5432, got %q", output)
	}
}

func TestSyncCmd_PS_JSON_ToStdout_Raw(t *testing.T) {
	// PS JSON value → stdout with --raw (no expand)
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{
		Value:     `{"DB_HOST":"localhost","DB_PORT":"5432"}`,
		StoreMode: tags.StoreModeJSON,
	})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/config", To: "-", Raw: true}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	// Raw mode: the JSON string is treated as a single entry value, not expanded
	// Key is "config" (path.Base), value is the raw JSON
	if !strings.Contains(output, "config=") {
		t.Errorf("output should contain config= key, got %q", output)
	}
	if !strings.Contains(output, "DB_HOST") {
		t.Errorf("output should contain raw JSON with DB_HOST, got %q", output)
	}
}

func TestSyncCmd_PS_Prefix_ToStdout(t *testing.T) {
	// PS prefix → stdout (.env format)
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
	_ = mb.Put(ctx, "ps:/app/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/", To: "-"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "DB_HOST=localhost") {
		t.Errorf("output should contain DB_HOST=localhost, got %q", output)
	}
	if !strings.Contains(output, "DB_PORT=5432") {
		t.Errorf("output should contain DB_PORT=5432, got %q", output)
	}
}

func TestSyncCmd_PS_ToSM(t *testing.T) {
	// PS single → SM (copy)
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{
		Value:     `{"key":"value"}`,
		StoreMode: tags.StoreModeJSON,
	})

	cmd := &SyncCmd{From: "ps:/app/config", To: "sm:myapp-config"}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the Put call to sm:
	var smCall *backend.PutCall
	for i, call := range mb.PutCalls {
		if call.Ref == "sm:myapp-config" {
			smCall = &mb.PutCalls[i]
			break
		}
	}
	if smCall == nil {
		t.Fatal("expected Put call to sm:myapp-config")
	}
	if smCall.Opts.StoreMode != tags.StoreModeJSON {
		t.Errorf("storeMode = %q, want %q", smCall.Opts.StoreMode, tags.StoreModeJSON)
	}
}

func TestSyncCmd_Stdin(t *testing.T) {
	// stdin → PS JSON
	mb, appCtx := newSyncTestContext(t)

	// Replace stdin
	r, w, _ := os.Pipe()
	oldStdin := os.Stdin
	os.Stdin = r
	if _, err := w.WriteString("KEY1=val1\nKEY2=val2\n"); err != nil {
		t.Fatalf("failed to write: %v", err)
	}
	w.Close()
	defer func() { os.Stdin = oldStdin }()

	cmd := &SyncCmd{From: "-", To: "ps:/app/config"}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mb.PutCalls) != 1 {
		t.Fatalf("expected 1 Put call, got %d", len(mb.PutCalls))
	}
	call := mb.PutCalls[0]
	if call.Ref != "ps:/app/config" {
		t.Errorf("ref = %q, want %q", call.Ref, "ps:/app/config")
	}
	if !strings.Contains(call.Opts.Value, "KEY1") {
		t.Errorf("value should contain KEY1, got %q", call.Opts.Value)
	}
}

func TestSyncCmd_FormatExport_ToStdout(t *testing.T) {
	// .env file → stdout with --format export
	_, appCtx := newSyncTestContext(t)

	tmpFile := writeTempEnv(t, "DB_HOST=localhost\nDB_PORT=5432\n")

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: tmpFile, To: "-", Format: "export"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "export DB_HOST=localhost") {
		t.Errorf("output should contain 'export DB_HOST=localhost', got %q", output)
	}
	if !strings.Contains(output, "export DB_PORT=5432") {
		t.Errorf("output should contain 'export DB_PORT=5432', got %q", output)
	}
}

func TestSyncCmd_FormatExport_PS_Prefix_ToStdout(t *testing.T) {
	// PS prefix → stdout with --format export
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
	_ = mb.Put(ctx, "ps:/app/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/", To: "-", Format: "export"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "export DB_HOST=localhost") {
		t.Errorf("output should contain 'export DB_HOST=localhost', got %q", output)
	}
	if !strings.Contains(output, "export DB_PORT=5432") {
		t.Errorf("output should contain 'export DB_PORT=5432', got %q", output)
	}
}

func TestSyncCmd_PS_Prefix_DotNormalization_ToStdout(t *testing.T) {
	// PS prefix with dot-containing keys → stdout: dots become underscores, keys uppercased
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/apigateway.url", backend.PutOptions{Value: "https://example.com", StoreMode: tags.StoreModeRaw})
	_ = mb.Put(ctx, "ps:/app/knowledgebase.id", backend.PutOptions{Value: "kb-123", StoreMode: tags.StoreModeRaw})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/", To: "-"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "APIGATEWAY_URL=https://example.com") {
		t.Errorf("output should contain APIGATEWAY_URL=https://example.com, got %q", output)
	}
	if !strings.Contains(output, "KNOWLEDGEBASE_ID=kb-123") {
		t.Errorf("output should contain KNOWLEDGEBASE_ID=kb-123, got %q", output)
	}
}

func TestSyncCmd_PS_JSON_DotNormalization_ToStdout(t *testing.T) {
	// PS JSON value with dot-containing keys → stdout: dots become underscores, keys uppercased
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{
		Value:     `{"apigateway.url":"https://example.com","knowledgebase.id":"kb-123"}`,
		StoreMode: tags.StoreModeJSON,
	})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/config", To: "-"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	if !strings.Contains(output, "APIGATEWAY_URL=https://example.com") {
		t.Errorf("output should contain APIGATEWAY_URL=https://example.com, got %q", output)
	}
	if !strings.Contains(output, "KNOWLEDGEBASE_ID=kb-123") {
		t.Errorf("output should contain KNOWLEDGEBASE_ID=kb-123, got %q", output)
	}
}

func TestSyncCmd_PS_Prefix_JSON_Value_NotExpanded(t *testing.T) {
	// PS prefix with a key whose value is a JSON string → stdout: value is NOT expanded
	// This is the regression test for the "test" key disappearing bug.
	mb, appCtx := newSyncTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/test", backend.PutOptions{
		Value:     `{"apigateway_url":"https://example.com"}`,
		StoreMode: tags.StoreModeRaw,
	})

	r, w, _ := os.Pipe()
	oldStdout := os.Stdout
	os.Stdout = w

	cmd := &SyncCmd{From: "ps:/app/", To: "-"}
	err := cmd.Run(appCtx)
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("failed to read: %v", err)
	}
	output := string(out)

	// Key should be "TEST", value should be the raw JSON string (not expanded)
	if !strings.Contains(output, `TEST={"apigateway_url":"https://example.com"}`) {
		t.Errorf("expected TEST key with raw JSON value, got %q", output)
	}
	// Expanded key "APIGATEWAY_URL" must NOT appear
	if strings.Contains(output, "APIGATEWAY_URL") {
		t.Errorf("APIGATEWAY_URL should not appear (JSON expansion must be suppressed), got %q", output)
	}
}

func TestSyncCmd_Helpers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) bool
		in   string
		want bool
	}{
		{"isBackendRef ps:", isBackendRef, "ps:/app/key", true},
		{"isBackendRef sm:", isBackendRef, "sm:secret", true},
		{"isBackendRef file", isBackendRef, "/tmp/file.env", false},
		{"isBackendRef stdin", isBackendRef, "-", false},
		{"isPrefix yes", isPrefix, "ps:/app/", true},
		{"isPrefix no", isPrefix, "ps:/app/key", false},
		{"isStdio yes", isStdio, "-", true},
		{"isStdio no", isStdio, "file.env", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.fn(tc.in); got != tc.want {
				t.Errorf("%s(%q) = %v, want %v", tc.name, tc.in, got, tc.want)
			}
		})
	}
}

func writeTempEnv(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return p
}
