package cmd

import (
	"bytes"
	"context"
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
			// L-03: --recursive なし → 直下のみ（サブパス除外）
			id:   "L-03",
			from: "ps:/app/",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/sub/nested", backend.PutOptions{Value: "value", StoreMode: tags.StoreModeRaw})
			},
			recursive: false,
			want: []string{
				"ps:/app/db_host",
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
