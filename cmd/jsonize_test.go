package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/config"
)

// --- テスト用ヘルパー Backend ---

// putErrorBackend は Put が常にエラーを返す Backend。
type putErrorBackend struct {
	*backend.MockBackend
}

func (e *putErrorBackend) Put(_ context.Context, _ string, _ backend.PutOptions) error {
	return fmt.Errorf("simulated put error")
}

// getErrorBackend は Get が指定エラーを返す Backend。
type getErrorBackend struct {
	*backend.MockBackend
	getErr error
}

func (e *getErrorBackend) Get(_ context.Context, _ string, _ backend.GetOptions) (string, error) {
	return "", e.getErr
}

// getByPrefixErrorBackend は GetByPrefix が指定エラーを返す Backend。
type getByPrefixErrorBackend struct {
	err error
}

func (e *getByPrefixErrorBackend) Put(_ context.Context, _ string, _ backend.PutOptions) error {
	return nil
}

func (e *getByPrefixErrorBackend) Get(_ context.Context, _ string, _ backend.GetOptions) (string, error) {
	return "", fmt.Errorf("key not found: not found")
}

func (e *getByPrefixErrorBackend) GetByPrefix(_ context.Context, _ string, _ backend.GetByPrefixOptions) ([]backend.ParameterEntry, error) {
	return nil, e.err
}

// --- テストヘルパー関数 ---

func newJsonizeTestContext(t *testing.T) (*backend.MockBackend, *Context) {
	t.Helper()
	mb := backend.NewMockBackend()
	return mb, &Context{
		Config: &config.Config{},
		BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
			return mb, nil
		},
	}
}

func setupJsonizeCmd(target, frompath string, opts ...func(*JsonizeCmd)) *JsonizeCmd {
	cmd := &JsonizeCmd{
		Target:    target,
		Frompath:  frompath,
		Store:     "json",
		ValueType: "string",
		Force:     false,
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

// findTargetPutCall は PutCalls から target ref への Put を返す。
func findTargetPutCall(mb *backend.MockBackend, targetRef string) *backend.PutCall {
	for i := len(mb.PutCalls) - 1; i >= 0; i-- {
		if mb.PutCalls[i].Ref == targetRef {
			call := mb.PutCalls[i]
			return &call
		}
	}
	return nil
}

func TestJsonizeCmd(t *testing.T) {
	t.Run("JC-01", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: "raw"})

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		if putCall.Opts.StoreMode != "json" {
			t.Errorf("StoreMode: got %q, want %q", putCall.Opts.StoreMode, "json")
		}
		assertJSONEqual(t, putCall.Opts.Value, `{"db":{"host":"localhost","port":5432}}`)
	})

	t.Run("JC-02", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/APP_NAME", backend.PutOptions{Value: "myapp", StoreMode: "raw"})

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/",
			func(c *JsonizeCmd) { c.Store = "raw" })
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		if putCall.Opts.StoreMode != "raw" {
			t.Errorf("StoreMode: got %q, want %q", putCall.Opts.StoreMode, "raw")
		}
		assertJSONEqual(t, putCall.Opts.Value, `{"app":{"name":"myapp"}}`)
	})

	t.Run("JC-03", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		assertJSONEqual(t, putCall.Opts.Value, `{}`)
	})

	t.Run("JC-04-force", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		// target に既存値をセット
		_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{Value: "old", StoreMode: "raw"})
		// frompath にデータをセット
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		putCountBefore := len(mb.PutCalls)

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/",
			func(c *JsonizeCmd) { c.Force = true })
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mb.PutCalls) <= putCountBefore {
			t.Fatal("expected additional Put call for target, but PutCalls did not increase")
		}
		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config after --force, not found")
		}
	})

	t.Run("JC-05-psa", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "psa:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd := setupJsonizeCmd("psa:/app/config", "psa:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "psa:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for psa:/app/config, not found")
		}
	})

	t.Run("JC-06-sm-target", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd := setupJsonizeCmd("sm:app-config", "ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "sm:app-config")
		if putCall == nil {
			t.Fatal("expected Put call for sm:app-config, not found")
		}
	})

	t.Run("JC-07-raw-json-mixed", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"timeout":30,"enabled":true}`, StoreMode: "json"})

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		if putCall.Opts.StoreMode != "json" {
			t.Errorf("StoreMode: got %q, want %q", putCall.Opts.StoreMode, "json")
		}
		// db.host と config.timeout/enabled が両方含まれることを確認
		var gotMap map[string]interface{}
		if err := json.Unmarshal([]byte(putCall.Opts.Value), &gotMap); err != nil {
			t.Fatalf("unmarshal value: %v", err)
		}
		if _, ok := gotMap["db"]; !ok {
			t.Errorf("expected 'db' key in result JSON, got: %s", putCall.Opts.Value)
		}
		if _, ok := gotMap["config"]; !ok {
			t.Errorf("expected 'config' key in result JSON, got: %s", putCall.Opts.Value)
		}
	})
}

func TestJsonizeCmd_Errors(t *testing.T) {
	t.Run("JCE-01-frompath-sm", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeCmd("ps:/app/config", "sm:secret")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--frompath sm: backend is not supported")
	})

	t.Run("JCE-02-invalid-target-ref", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeCmd("invalid-ref", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "invalid target ref")
	})

	t.Run("JCE-03-invalid-frompath-ref", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeCmd("ps:/app/config", "invalid-ref")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "invalid frompath ref")
	})

	t.Run("JCE-04-target-exists-no-force", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		// target に既存値をセット
		_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{Value: "old", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		putCountBefore := len(mb.PutCalls)

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "target already exists")

		// Run 内の Put は呼ばれていないことを確認
		if len(mb.PutCalls) != putCountBefore {
			t.Errorf("expected no new Put calls after error, but PutCalls changed: before=%d, after=%d",
				putCountBefore, len(mb.PutCalls))
		}
	})

	t.Run("JCE-05-getbyprefix-error", func(t *testing.T) {
		errBackend := &getByPrefixErrorBackend{err: fmt.Errorf("simulated getByPrefix error")}
		appCtx := &Context{
			Config: &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
				return errBackend, nil
			},
		}
		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "get parameters")
	})

	t.Run("JCE-06-build-conflict", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		// DB_HOST と DB_HOST_SUB が競合する
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "x", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST_SUB", backend.PutOptions{Value: "y", StoreMode: "raw"})

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "build json")
	})

	t.Run("JCE-07-put-error", func(t *testing.T) {
		mb := backend.NewMockBackend()
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		putErrBackend := &putErrorBackend{MockBackend: mb}
		appCtx := &Context{
			Config: &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
				if bt == backend.BackendTypeSM {
					return putErrBackend, nil
				}
				return mb, nil
			},
		}

		cmd := setupJsonizeCmd("sm:app-config", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "put target")
	})

	t.Run("JCE-08-get-permission-error", func(t *testing.T) {
		mb := backend.NewMockBackend()
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		// ParameterNotFound ではない権限エラー
		accessDenied := fmt.Errorf("AccessDeniedException: User is not authorized")
		errBackend := &getErrorBackend{MockBackend: mb, getErr: accessDenied}

		appCtx := &Context{
			Config: &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
				return errBackend, nil
			},
		}

		cmd := setupJsonizeCmd("ps:/app/config", "ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "check target existence")
	})
}

// assertJSONEqual は2つの JSON 文字列が意味的に等しいことを検証する。
func assertJSONEqual(t *testing.T, got, want string) {
	t.Helper()
	var gotMap, wantMap interface{}
	if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
		t.Fatalf("unmarshal got %q: %v", got, err)
	}
	if err := json.Unmarshal([]byte(want), &wantMap); err != nil {
		t.Fatalf("unmarshal want %q: %v", want, err)
	}
	if !reflect.DeepEqual(gotMap, wantMap) {
		t.Errorf("JSON mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

// assertErrContains はエラーが期待する文字列を含むことを検証する。
func assertErrContains(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", want)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}
