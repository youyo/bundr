package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
	"github.com/youyo/bundr/internal/tags"
)

func newLsTestContext(t *testing.T) (*backend.MockBackend, *Context) {
	t.Helper()
	mb := backend.NewMockBackend()
	return mb, &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
	}
}

func TestLsCmd(t *testing.T) {
	tests := []struct {
		id        string
		from      string
		recursive bool
		setup     func(mb *backend.MockBackend)
		want      []string
		wantErr   string
	}{
		{
			// L-01: 3パラメータ → 3行のフル ref パスを昇順で出力
			id:   "L-01",
			from: "ps:/app/",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/api_key", backend.PutOptions{Value: "secret", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"ps:/app/api_key",
				"ps:/app/db_host",
				"ps:/app/db_port",
			},
		},
		{
			// L-02: 空 prefix → 0行（エラーなし）
			id:    "L-02",
			from:  "ps:/app/",
			setup: func(mb *backend.MockBackend) {},
			want:  []string{},
		},
		{
			// L-03: --recursive なし → 次レベルのみ（leaf + virtual directory）
			id:   "L-03",
			from: "ps:/app/",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/sub/nested", backend.PutOptions{Value: "value", StoreMode: tags.StoreModeRaw})
			},
			recursive: false,
			want: []string{
				"ps:/app/db_host", // leaf
				"ps:/app/sub",     // virtual directory prefix
			},
		},
		{
			// L-06: ルート ps:/ → トップレベルディレクトリが返る
			id:   "L-06",
			from: "ps:/",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/key", backend.PutOptions{Value: "v1", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/other/key2", backend.PutOptions{Value: "v2", StoreMode: tags.StoreModeRaw})
			},
			recursive: false,
			want: []string{
				"ps:/app",
				"ps:/other",
			},
		},
		{
			// L-04: sm:prefix 指定 → sm: backend でシークレット一覧を取得できる
			id:   "L-04",
			from: "sm:partner-ops/",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "sm:partner-ops/api-key", backend.PutOptions{Value: "secret", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "sm:partner-ops/db-pass", backend.PutOptions{Value: "pass", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"sm:partner-ops/api-key",
				"sm:partner-ops/db-pass",
			},
		},
		{
			// L-04b: sm: (全シークレット) → 全シークレット一覧
			id:   "L-04b",
			from: "sm:",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "sm:secret-a", backend.PutOptions{Value: "v1", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "sm:secret-b", backend.PutOptions{Value: "v2", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"sm:secret-a",
				"sm:secret-b",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			mb, appCtx := newLsTestContext(t)
			tc.setup(mb)

			var buf bytes.Buffer
			cmd := &LsCmd{
				From:      tc.from,
				Recursive: tc.recursive,
				out:       &buf,
			}

			err := cmd.Run(appCtx)

			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			output := buf.String()
			lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
			if output == "" {
				lines = []string{}
			}

			if len(lines) != len(tc.want) {
				t.Fatalf("line count mismatch: got %d %q, want %d %q", len(lines), lines, len(tc.want), tc.want)
			}

			for i, wantLine := range tc.want {
				if lines[i] != wantLine {
					t.Errorf("line %d: got %q, want %q", i, lines[i], wantLine)
				}
			}
		})
	}
}

func TestLsCmd_AWSError(t *testing.T) {
	// L-05: バックエンド生成エラー → "ls command failed" を含むエラー返却
	appCtx := &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return nil, fmt.Errorf("AWS connection error")
		},
	}
	cmd := &LsCmd{
		From: "ps:/app/",
		out:  &bytes.Buffer{},
	}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ls command failed") {
		t.Errorf("error %q does not contain %q", err.Error(), "ls command failed")
	}
}

func TestLsCmd_WritesCache(t *testing.T) {
	mb := backend.NewMockBackend()
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
	_ = mb.Put(ctx, "ps:/app/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})

	mockStore := &MockStore{}
	appCtx := &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
		CacheStore: mockStore,
	}

	var buf bytes.Buffer
	cmd := &LsCmd{From: "ps:/app/", out: &buf}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// キャッシュ Write が呼ばれたこと
	if len(mockStore.WriteCalls) != 1 {
		t.Fatalf("Write called %d times, want 1", len(mockStore.WriteCalls))
	}
	if got := mockStore.WriteCalls[0].BackendType; got != "ps" {
		t.Errorf("backendType = %q, want %q", got, "ps")
	}
	if got := len(mockStore.WriteCalls[0].Entries); got != 2 {
		t.Errorf("entries count = %d, want 2", got)
	}
}

// LS-D-01: ls --describe outputs valid JSON array with "ref" field
func TestLsCmd_Describe_PS(t *testing.T) {
	mb, appCtx := newLsTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
	_ = mb.Put(ctx, "ps:/app/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})

	var buf bytes.Buffer
	cmd := &LsCmd{From: "ps:/app/", Describe: true, out: &buf}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Output must be valid JSON array
	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 2 {
		t.Fatalf("got %d entries, want 2", len(result))
	}
	// Each entry must have "ref" field
	for _, entry := range result {
		if _, ok := entry["ref"]; !ok {
			t.Errorf("entry missing 'ref' field: %v", entry)
		}
		// ref should start with "ps:"
		ref, _ := entry["ref"].(string)
		if !strings.HasPrefix(ref, "ps:") {
			t.Errorf("ref = %q, want prefix %q", ref, "ps:")
		}
	}
	// Sorted by path: db_host before db_port
	ref0, _ := result[0]["ref"].(string)
	ref1, _ := result[1]["ref"].(string)
	if ref0 != "ps:/app/db_host" {
		t.Errorf("result[0].ref = %q, want %q", ref0, "ps:/app/db_host")
	}
	if ref1 != "ps:/app/db_port" {
		t.Errorf("result[1].ref = %q, want %q", ref1, "ps:/app/db_port")
	}
}

// LS-D-EDGE-01: ls --describe with 0 entries outputs empty JSON array
func TestLsCmd_Describe_Empty(t *testing.T) {
	_, appCtx := newLsTestContext(t)

	var buf bytes.Buffer
	cmd := &LsCmd{From: "ps:/empty/", Describe: true, out: &buf}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// Output must be valid JSON array (empty)
	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 0 {
		t.Errorf("got %d entries, want 0", len(result))
	}
}

// LS-D-02: ls --describe with SM prefix outputs valid JSON array
func TestLsCmd_Describe_SM(t *testing.T) {
	mb, appCtx := newLsTestContext(t)
	ctx := context.Background()
	_ = mb.Put(ctx, "sm:partner-ops/api-key", backend.PutOptions{Value: "secret", StoreMode: tags.StoreModeRaw})

	var buf bytes.Buffer
	cmd := &LsCmd{From: "sm:partner-ops/", Describe: true, out: &buf}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	var result []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("output is not valid JSON array: %v\noutput: %s", err, buf.String())
	}
	if len(result) != 1 {
		t.Fatalf("got %d entries, want 1", len(result))
	}
	ref, _ := result[0]["ref"].(string)
	if ref != "sm:partner-ops/api-key" {
		t.Errorf("ref = %q, want %q", ref, "sm:partner-ops/api-key")
	}
}

func TestLsCmd_NilCacheStore(t *testing.T) {
	// CacheStore が nil でもパニックしないこと
	mb := backend.NewMockBackend()
	appCtx := &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
		CacheStore: nil,
	}

	var buf bytes.Buffer
	cmd := &LsCmd{From: "ps:/app/", out: &buf}
	if err := cmd.Run(appCtx); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}
