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

func newExportTestContext(t *testing.T) (*backend.MockBackend, *Context) {
	t.Helper()
	mb := backend.NewMockBackend()
	return mb, &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
	}
}

func setupExportCmd(from, format string, opts ...func(*ExportCmd)) *ExportCmd {
	cmd := &ExportCmd{
		From:           from,
		Format:         format,
		NoFlatten:      false,
		ArrayMode:      "join",
		ArrayJoinDelim: ",",
		FlattenDelim:   "_",
		Upper:          true,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

func TestExportCmd(t *testing.T) {
	tests := []struct {
		id      string
		from    string
		format  string
		setup   func(mb *backend.MockBackend)
		cmdOpts []func(*ExportCmd)
		want    []string
		wantErr string
	}{
		{
			id:     "E-01",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export DB_HOST='localhost'",
				"export DB_PORT='5432'",
			},
		},
		{
			id:     "E-02",
			from:   "ps:/app/prod/",
			format: "dotenv",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"DB_HOST=localhost",
				"DB_PORT=5432",
			},
		},
		{
			id:     "E-03",
			from:   "ps:/app/prod/",
			format: "direnv",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export DB_HOST='localhost'",
				"export DB_PORT='5432'",
			},
		},
		{
			id:     "E-04",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"db":{"host":"localhost"}}`, StoreMode: tags.StoreModeJSON})
			},
			want: []string{
				"export CONFIG_DB_HOST='localhost'",
			},
		},
		{
			id:     "E-05",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"db":{"host":"localhost"}}`, StoreMode: tags.StoreModeJSON})
			},
			cmdOpts: []func(*ExportCmd){
				func(c *ExportCmd) { c.Upper = false },
			},
			want: []string{
				"export config_db_host='localhost'",
			},
		},
		{
			id:     "E-06",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
			},
			cmdOpts: []func(*ExportCmd){
				func(c *ExportCmd) { c.NoFlatten = true },
			},
			want: []string{
				"export DB_HOST='localhost'",
			},
		},
		{
			id:     "E-07",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"db":{"host":"localhost"}}`, StoreMode: tags.StoreModeJSON})
			},
			cmdOpts: []func(*ExportCmd){
				func(c *ExportCmd) { c.NoFlatten = true },
			},
			want: []string{
				`export CONFIG='{"db":{"host":"localhost"}}'`,
			},
		},
		{
			id:     "E-08",
			from:   "ps:/app/prod/",
			format: "shell",
			setup:  func(mb *backend.MockBackend) {}, // no data
			want:   []string{},
		},
		{
			id:     "E-09",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/KEY", backend.PutOptions{Value: "it's a test", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export KEY='it'\"'\"'s a test'",
			},
		},
		{
			id:     "E-10",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/TAGS", backend.PutOptions{Value: `["a","b","c"]`, StoreMode: tags.StoreModeJSON})
			},
			want: []string{
				"export TAGS='a,b,c'",
			},
		},
		{
			id:     "E-11",
			from:   "psa:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "psa:/app/prod/KEY", backend.PutOptions{Value: "val", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export KEY='val'",
			},
		},
		{
			id:     "E-12",
			from:   "ps:/app/prod/MY_KEY",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/MY_KEY", backend.PutOptions{Value: "myvalue", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export MY_KEY='myvalue'",
			},
		},
		{
			id:     "E-13",
			from:   "ps:/app/prod/MY_KEY",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/MY_KEY", backend.PutOptions{Value: "myvalue", StoreMode: tags.StoreModeRaw})
			},
			cmdOpts: []func(*ExportCmd){
				func(c *ExportCmd) { c.Upper = false },
			},
			want: []string{
				"export my_key='myvalue'",
			},
		},
		{
			id:     "E-14",
			from:   "ps:/app/prod/MISSING",
			format: "shell",
			setup:  func(mb *backend.MockBackend) {}, // no data
			wantErr: "key not found",
		},
		{
			id:     "E-15",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/api.url", backend.PutOptions{Value: "http://x", StoreMode: tags.StoreModeRaw})
			},
			want: []string{
				"export API_URL='http://x'",
			},
		},
		{
			id:     "E-16",
			from:   "ps:/app/prod/",
			format: "shell",
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CFG", backend.PutOptions{Value: `{"api.url":"http://x"}`, StoreMode: tags.StoreModeJSON})
			},
			want: []string{
				"export CFG_API_URL='http://x'",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			mb, appCtx := newExportTestContext(t)
			tc.setup(mb)

			var buf bytes.Buffer
			cmd := setupExportCmd(tc.from, tc.format, tc.cmdOpts...)
			cmd.out = &buf

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

func TestExportCmd_Errors(t *testing.T) {
	tests := []struct {
		id      string
		from    string
		setup   func(t *testing.T) *Context
		wantErr string
	}{
		{
			id:   "EE-01",
			from: "sm:secret-id",
			setup: func(t *testing.T) *Context {
				t.Helper()
				mb := backend.NewMockBackend()
				return &Context{
					Config: &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
						return mb, nil
					},
				}
			},
			wantErr: "sm: backend is not supported",
		},
		{
			id:   "EE-02",
			from: "invalid-ref",
			setup: func(t *testing.T) *Context {
				t.Helper()
				return &Context{
					Config: &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
						return nil, fmt.Errorf("should not be called")
					},
				}
			},
			wantErr: "invalid ref",
		},
		{
			id:   "EE-03",
			from: "ps:/app/prod/",
			setup: func(t *testing.T) *Context {
				t.Helper()
				return &Context{
					Config: &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
						return nil, fmt.Errorf("backend creation failed")
					},
				}
			},
			wantErr: "backend creation failed",
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			appCtx := tc.setup(t)
			var buf bytes.Buffer
			cmd := setupExportCmd(tc.from, "shell")
			cmd.out = &buf

			err := cmd.Run(appCtx)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
