package cmd

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
	"github.com/youyo/bundr/internal/tags"
)

// MockRunner records calls and returns configured results.
type MockRunner struct {
	runs     []mockRunCall
	exitCode int
	err      error
}

type mockRunCall struct {
	name string
	args []string
	env  []string
}

func (m *MockRunner) Run(name string, args []string, env []string) (int, error) {
	m.runs = append(m.runs, mockRunCall{name: name, args: args, env: env})
	return m.exitCode, m.err
}

func (m *MockRunner) Called() bool {
	return len(m.runs) > 0
}

func (m *MockRunner) LastEnv() []string {
	if len(m.runs) == 0 {
		return nil
	}
	return m.runs[len(m.runs)-1].env
}

// envMap converts a []string "KEY=VALUE" slice to a map for easier assertion.
func envMap(env []string) map[string]string {
	m := make(map[string]string)
	for _, e := range env {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			m[e] = ""
			continue
		}
		m[e[:idx]] = e[idx+1:]
	}
	return m
}

// isExitCodeError checks if err is *ExitCodeError and sets the pointer.
func isExitCodeError(err error, target **ExitCodeError) bool {
	if e, ok := err.(*ExitCodeError); ok {
		*target = e
		return true
	}
	return false
}

func newExecTestContext(t *testing.T) (*backend.MockBackend, *Context) {
	t.Helper()
	mb := backend.NewMockBackend()
	return mb, &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
	}
}

func setupExecCmd(from []string, args []string, opts ...func(*ExecCmd)) *ExecCmd {
	cmd := &ExecCmd{
		From:           from,
		Args:           args,
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

// ─── 正常系 ───────────────────────────────────────────────────────────────────

func TestExecCmd(t *testing.T) {
	tests := []struct {
		id       string
		from     []string
		args     []string
		setup    func(mb *backend.MockBackend)
		cmdOpts  []func(*ExecCmd)
		wantEnv  map[string]string
		wantCode int
	}{
		{
			id:   "R-01",
			from: []string{"ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			wantEnv: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
		},
		{
			id:   "R-02",
			from: []string{"ps:/shared/", "ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/shared/DB_HOST", backend.PutOptions{Value: "shared-host", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "prod-host", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			wantEnv: map[string]string{
				"DB_HOST": "prod-host",
				"DB_PORT": "5432",
			},
		},
		{
			id:   "R-03",
			from: []string{"ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/db_host", backend.PutOptions{Value: "localhost", StoreMode: tags.StoreModeRaw})
				_ = mb.Put(ctx, "ps:/app/prod/db_port", backend.PutOptions{Value: "5432", StoreMode: tags.StoreModeRaw})
			},
			wantEnv: map[string]string{
				"DB_HOST": "localhost",
				"DB_PORT": "5432",
			},
		},
		{
			id:   "R-04",
			from: []string{"ps:/app/prod/"},
			args: []string{"false"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/KEY", backend.PutOptions{Value: "val", StoreMode: tags.StoreModeRaw})
			},
			cmdOpts: []func(*ExecCmd){
				func(c *ExecCmd) {
					c.runner = &MockRunner{exitCode: 1, err: fmt.Errorf("exit status 1")}
				},
			},
			wantCode: 1,
		},
		{
			id:   "R-05",
			from: []string{"ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"db":{"host":"localhost"}}`, StoreMode: tags.StoreModeJSON})
			},
			wantEnv: map[string]string{
				"CONFIG_DB_HOST": "localhost",
			},
		},
		{
			id:   "R-06",
			from: []string{"psa:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "psa:/app/prod/SECRET_KEY", backend.PutOptions{Value: "abc123", StoreMode: tags.StoreModeRaw})
			},
			wantEnv: map[string]string{
				"SECRET_KEY": "abc123",
			},
		},
		{
			id:   "R-07",
			from: []string{"ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/NEW_VAR", backend.PutOptions{Value: "injected", StoreMode: tags.StoreModeRaw})
			},
			wantEnv: map[string]string{
				"NEW_VAR": "injected",
				// EXISTING_VAR would be in os.Environ() - checked separately
			},
		},
		{
			id:   "R-08",
			from: []string{"ps:/app/prod/"},
			args: []string{"env"},
			setup: func(mb *backend.MockBackend) {
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"db":{"host":"localhost"}}`, StoreMode: tags.StoreModeJSON})
			},
			cmdOpts: []func(*ExecCmd){
				func(c *ExecCmd) { c.NoFlatten = true },
			},
			wantEnv: map[string]string{
				"CONFIG": `{"db":{"host":"localhost"}}`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			mb, appCtx := newExecTestContext(t)
			tc.setup(mb)

			mr := &MockRunner{exitCode: 0}
			cmd := setupExecCmd(tc.from, tc.args, tc.cmdOpts...)
			// R-04 sets its own runner; others use the shared MockRunner
			if cmd.runner == nil {
				cmd.runner = mr
			}

			err := cmd.Run(appCtx)

			if tc.wantCode != 0 {
				if err == nil {
					t.Fatalf("expected ExitCodeError with code %d, got nil", tc.wantCode)
				}
				var exitErr *ExitCodeError
				if !isExitCodeError(err, &exitErr) {
					t.Fatalf("expected *ExitCodeError, got %T: %v", err, err)
				}
				if exitErr.Code != tc.wantCode {
					t.Errorf("exit code: got %d, want %d", exitErr.Code, tc.wantCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			var envToCheck []string
			if cmd.runner == mr {
				envToCheck = mr.LastEnv()
			}

			if envToCheck != nil {
				got := envMap(envToCheck)
				for key, wantVal := range tc.wantEnv {
					if gotVal, ok := got[key]; !ok {
						t.Errorf("env missing key %q", key)
					} else if gotVal != wantVal {
						t.Errorf("env[%q] = %q, want %q", key, gotVal, wantVal)
					}
				}
			}
		})
	}
}

// ─── 異常系 ───────────────────────────────────────────────────────────────────

func TestExecCmd_Errors(t *testing.T) {
	tests := []struct {
		id           string
		from         []string
		args         []string
		setup        func(t *testing.T) *Context
		cmdOpts      []func(*ExecCmd)
		wantErr      string
		runnerCalled bool
	}{
		{
			id:   "RE-01",
			from: []string{"ps:/app/prod/"},
			args: []string{},
			setup: func(t *testing.T) *Context {
				t.Helper()
				mb := backend.NewMockBackend()
				return &Context{
					Config:         &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
				}
			},
			wantErr:      "no command specified",
			runnerCalled: false,
		},
		{
			id:   "RE-02",
			from: []string{"ps:/app/prod/"},
			args: []string{"python", "app.py"},
			setup: func(t *testing.T) *Context {
				t.Helper()
				return &Context{
					Config: &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
						return nil, fmt.Errorf("backend creation failed")
					},
				}
			},
			wantErr:      "backend creation failed",
			runnerCalled: false,
		},
		{
			id:   "RE-03",
			from: []string{"sm:secret-id"},
			args: []string{"env"},
			setup: func(t *testing.T) *Context {
				t.Helper()
				mb := backend.NewMockBackend()
				return &Context{
					Config:         &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
				}
			},
			wantErr:      "sm: backend is not supported",
			runnerCalled: false,
		},
		{
			id:   "RE-04",
			from: []string{"invalid-ref"},
			args: []string{"env"},
			setup: func(t *testing.T) *Context {
				t.Helper()
				mb := backend.NewMockBackend()
				return &Context{
					Config:         &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
				}
			},
			wantErr:      "invalid ref",
			runnerCalled: false,
		},
		{
			id:   "RE-05",
			from: []string{"ps:/app/prod/", "ps:/shared/"},
			args: []string{"env"},
			setup: func(t *testing.T) *Context {
				t.Helper()
				callCount := 0
				mb := backend.NewMockBackend()
				ctx := context.Background()
				_ = mb.Put(ctx, "ps:/app/prod/KEY", backend.PutOptions{Value: "val", StoreMode: tags.StoreModeRaw})
				return &Context{
					Config: &config.Config{},
					BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
						callCount++
						if callCount > 1 {
							return nil, fmt.Errorf("second backend failed")
						}
						return mb, nil
					},
				}
			},
			wantErr:      "second backend failed",
			runnerCalled: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			appCtx := tc.setup(t)
			mr := &MockRunner{}
			cmd := setupExecCmd(tc.from, tc.args, tc.cmdOpts...)
			cmd.runner = mr

			err := cmd.Run(appCtx)

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
			if tc.runnerCalled != mr.Called() {
				t.Errorf("runner called: got %v, want %v", mr.Called(), tc.runnerCalled)
			}
		})
	}
}

// ─── エッジケース ─────────────────────────────────────────────────────────────

func TestExecCmd_EdgeCases(t *testing.T) {
	t.Run("RE-06-empty-prefix", func(t *testing.T) {
		mb := backend.NewMockBackend()
		appCtx := &Context{
			Config:         &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
		}
		mr := &MockRunner{exitCode: 0}
		cmd := setupExecCmd([]string{"ps:/empty/prefix/"}, []string{"env"})
		cmd.runner = mr

		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !mr.Called() {
			t.Error("runner should have been called even with empty prefix")
		}
	})

	t.Run("RE-07-duplicate-key-last-wins", func(t *testing.T) {
		mb := backend.NewMockBackend()
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/a/KEY", backend.PutOptions{Value: "value_a", StoreMode: tags.StoreModeRaw})
		_ = mb.Put(ctx, "ps:/b/KEY", backend.PutOptions{Value: "value_b", StoreMode: tags.StoreModeRaw})

		appCtx := &Context{
			Config:         &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
		}
		mr := &MockRunner{exitCode: 0}
		cmd := setupExecCmd([]string{"ps:/a/", "ps:/b/"}, []string{"env"})
		cmd.runner = mr

		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := envMap(mr.LastEnv())
		if got["KEY"] != "value_b" {
			t.Errorf("env[KEY] = %q, want %q", got["KEY"], "value_b")
		}
	})

	t.Run("RE-08-special-chars", func(t *testing.T) {
		mb := backend.NewMockBackend()
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/MSG", backend.PutOptions{Value: "it's ok", StoreMode: tags.StoreModeRaw})

		appCtx := &Context{
			Config:         &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) { return mb, nil },
		}
		mr := &MockRunner{exitCode: 0}
		cmd := setupExecCmd([]string{"ps:/app/prod/"}, []string{"printenv", "MSG"})
		cmd.runner = mr

		err := cmd.Run(appCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := envMap(mr.LastEnv())
		if got["MSG"] != "it's ok" {
			t.Errorf("env[MSG] = %q, want %q", got["MSG"], "it's ok")
		}
	})
}
