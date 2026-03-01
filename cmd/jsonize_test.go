package cmd

import (
	"bytes"
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

func (e *getByPrefixErrorBackend) Describe(_ context.Context, _ string) (*backend.DescribeOutput, error) {
	return nil, fmt.Errorf("not implemented")
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

// setupJsonizeStdoutCmd は stdout モード（--to なし）の JsonizeCmd を作成する。
func setupJsonizeStdoutCmd(frompath string, opts ...func(*JsonizeCmd)) *JsonizeCmd {
	cmd := &JsonizeCmd{
		Frompath: []string{frompath},
	}
	for _, opt := range opts {
		opt(cmd)
	}
	return cmd
}

// setupJsonizeStdoutCmdWithBuf は stdout モードで出力を bytes.Buffer にキャプチャする JsonizeCmd を作成する。
func setupJsonizeStdoutCmdWithBuf(frompath string, opts ...func(*JsonizeCmd)) (*JsonizeCmd, *bytes.Buffer) {
	var buf bytes.Buffer
	cmd := setupJsonizeStdoutCmd(frompath, opts...)
	cmd.out = &buf
	return cmd, &buf
}

// setupJsonizeSaveCmd は save モード（--to あり）の JsonizeCmd を作成する。
func setupJsonizeSaveCmd(frompath, to string, opts ...func(*JsonizeCmd)) *JsonizeCmd {
	cmd := &JsonizeCmd{
		Frompath: []string{frompath},
		To:       &to,
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

// --- JS-NN: stdout モード正常系 ---

func TestJsonizeCmdStdout(t *testing.T) {
	t.Run("JS-01", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: "raw"})

		cmd, buf := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{"db":{"host":"localhost","port":5432}}`)
	})

	t.Run("JS-02", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd, buf := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{}`)
	})

	t.Run("JS-03-compact", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd, buf := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/",
			func(c *JsonizeCmd) { c.Compact = true })
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := strings.TrimSpace(buf.String())
		// compact: 余分な空白・改行なし
		if strings.Contains(got, "\n") {
			t.Errorf("expected compact JSON (no newlines), got: %s", got)
		}
		assertJSONEqual(t, got, `{"db":{"host":"localhost"}}`)
	})

	t.Run("JS-04-indent-default", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd, buf := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := buf.String()
		// インデント付き出力には改行とスペースが含まれる
		if !strings.Contains(got, "\n") || !strings.Contains(got, "  ") {
			t.Errorf("expected indented JSON, got: %s", got)
		}
		assertJSONEqual(t, strings.TrimSpace(got), `{"db":{"host":"localhost"}}`)
	})

	t.Run("JS-05-json-storemode", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/CONFIG", backend.PutOptions{Value: `{"timeout":30,"enabled":true}`, StoreMode: "json"})

		cmd, buf := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := strings.TrimSpace(buf.String())
		var gotMap map[string]interface{}
		if err := json.Unmarshal([]byte(got), &gotMap); err != nil {
			t.Fatalf("unmarshal got: %v", err)
		}
		if _, ok := gotMap["db"]; !ok {
			t.Errorf("expected 'db' key in result JSON")
		}
		if _, ok := gotMap["config"]; !ok {
			t.Errorf("expected 'config' key in result JSON")
		}
	})
}

// --- JS-06~: 複数 frompath / 末端パラメータ ---

func TestJsonizeCmdMultiFrompath(t *testing.T) {
	t.Run("JS-06-multi-frompath-prefix", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/a/app/HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/a/db/PORT", backend.PutOptions{Value: "5432", StoreMode: "raw"})

		var buf bytes.Buffer
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/app/", "ps:/a/db/"},
			out:      &buf,
		}
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{"host":"localhost","port":5432}`)
	})

	t.Run("JS-07-leaf-param", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/a/b/host", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		var buf bytes.Buffer
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/b/host"},
			out:      &buf,
		}
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{"host":"localhost"}`)
	})

	t.Run("JS-08-leaf-dotname", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/a/apiGateway.url", backend.PutOptions{Value: "https://api.example.com", StoreMode: "raw"})

		var buf bytes.Buffer
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/apiGateway.url"},
			out:      &buf,
		}
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{"apigateway.url":"https://api.example.com"}`)
	})

	t.Run("JS-09-multi-leaf", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/a/url", backend.PutOptions{Value: "https://example.com", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/a/arn", backend.PutOptions{Value: "arn:aws:lambda:us-east-1:123:function:test", StoreMode: "raw"})

		var buf bytes.Buffer
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/url", "ps:/a/arn"},
			out:      &buf,
		}
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := strings.TrimSpace(buf.String())
		assertJSONEqual(t, got, `{"url":"https://example.com","arn":"arn:aws:lambda:us-east-1:123:function:test"}`)
	})

	t.Run("JS-10-leaf-not-found", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		var buf bytes.Buffer
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/nonexistent"},
			out:      &buf,
		}
		err := cmd.Run(appCtx)
		if err == nil {
			t.Fatal("expected error for non-existent leaf parameter, got nil")
		}
		if !strings.Contains(err.Error(), "key not found") {
			t.Errorf("error = %q, want to contain %q", err.Error(), "key not found")
		}
	})
}

func TestJsonizeCmdMultiFrompathSaveErrors(t *testing.T) {
	t.Run("JSaveE-08-multi-self-ref", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		to := "ps:/b/config"
		cmd := &JsonizeCmd{
			Frompath: []string{"ps:/a/", "ps:/b/"},
			To:       &to,
		}
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "overlaps")
	})
}

// --- JSave-NN: save モード正常系 ---

func TestJsonizeCmdSave(t *testing.T) {
	t.Run("JSave-01", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{Value: "5432", StoreMode: "raw"})

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config")
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

	t.Run("JSave-02-store-raw", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/APP_NAME", backend.PutOptions{Value: "myapp", StoreMode: "raw"})

		store := "raw"
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config",
			func(c *JsonizeCmd) { c.Store = &store })
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
	})

	t.Run("JSave-03-empty", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		assertJSONEqual(t, putCall.Opts.Value, `{}`)
	})

	t.Run("JSave-04-force", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{Value: "old", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		putCountBefore := len(mb.PutCalls)

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config",
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

	t.Run("JSave-05-psa", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "psa:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd := setupJsonizeSaveCmd("psa:/app/prod/", "psa:/app/config")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "psa:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for psa:/app/config, not found")
		}
	})

	t.Run("JSave-06-sm-target", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "sm:app-config")
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "sm:app-config")
		if putCall == nil {
			t.Fatal("expected Put call for sm:app-config, not found")
		}
	})

	t.Run("JSave-07-value-type-secure", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		vt := "secure"
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config",
			func(c *JsonizeCmd) { c.ValueType = &vt })
		if err := cmd.Run(appCtx); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		putCall := findTargetPutCall(mb, "ps:/app/config")
		if putCall == nil {
			t.Fatal("expected Put call for ps:/app/config, not found")
		}
		if putCall.Opts.ValueType != "secure" {
			t.Errorf("ValueType: got %q, want %q", putCall.Opts.ValueType, "secure")
		}
	})
}

// --- JSE-NN: stdout モード固有エラー ---

func TestJsonizeCmdStdoutErrors(t *testing.T) {
	t.Run("JSE-01-store-without-to", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		store := "raw"
		cmd := setupJsonizeStdoutCmd("ps:/app/prod/",
			func(c *JsonizeCmd) { c.Store = &store })
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--store is only valid with --to")
	})

	t.Run("JSE-02-value-type-without-to", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		vt := "secure"
		cmd := setupJsonizeStdoutCmd("ps:/app/prod/",
			func(c *JsonizeCmd) { c.ValueType = &vt })
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--value-type is only valid with --to")
	})

	t.Run("JSE-03-force-without-to", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeStdoutCmd("ps:/app/prod/",
			func(c *JsonizeCmd) { c.Force = true })
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--force is only valid with --to")
	})
}

// --- JSaveE-NN: save モード固有エラー ---

func TestJsonizeCmdSaveErrors(t *testing.T) {
	t.Run("JSaveE-01-compact-with-to", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config",
			func(c *JsonizeCmd) { c.Compact = true })
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--compact is only valid without --to")
	})

	t.Run("JSaveE-02-invalid-target-ref", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "invalid-ref")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "invalid target ref")
	})

	t.Run("JSaveE-03-self-reference", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/prod/config")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "overlaps")
	})

	t.Run("JSaveE-04-target-exists-no-force", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/config", backend.PutOptions{Value: "old", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		putCountBefore := len(mb.PutCalls)

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "target already exists")

		if len(mb.PutCalls) != putCountBefore {
			t.Errorf("expected no new Put calls after error, PutCalls changed: before=%d, after=%d",
				putCountBefore, len(mb.PutCalls))
		}
	})

	t.Run("JSaveE-05-getbyprefix-error", func(t *testing.T) {
		errBackend := &getByPrefixErrorBackend{err: fmt.Errorf("simulated getByPrefix error")}
		appCtx := &Context{
			Config: &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
				return errBackend, nil
			},
		}
		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "get parameters")
	})

	t.Run("JSaveE-06-put-error", func(t *testing.T) {
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

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "sm:app-config")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "put target")
	})

	t.Run("JSaveE-07-check-existence-error", func(t *testing.T) {
		mb := backend.NewMockBackend()
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "localhost", StoreMode: "raw"})

		accessDenied := fmt.Errorf("AccessDeniedException: User is not authorized")
		errBackend := &getErrorBackend{MockBackend: mb, getErr: accessDenied}

		appCtx := &Context{
			Config: &config.Config{},
			BackendFactory: func(bt backend.BackendType) (backend.Backend, error) {
				return errBackend, nil
			},
		}

		cmd := setupJsonizeSaveCmd("ps:/app/prod/", "ps:/app/config")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "check target existence")
	})
}

// --- JCE-NN: 共通異常系 ---

func TestJsonizeCmdCommonErrors(t *testing.T) {
	t.Run("JCE-01-frompath-sm", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeStdoutCmd("sm:secret")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "--frompath sm: backend is not supported")
	})

	t.Run("JCE-02-invalid-frompath-ref", func(t *testing.T) {
		_, appCtx := newJsonizeTestContext(t)
		cmd := setupJsonizeStdoutCmd("invalid-ref")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "invalid frompath ref")
	})

	t.Run("JCE-03-build-conflict", func(t *testing.T) {
		mb, appCtx := newJsonizeTestContext(t)
		ctx := context.Background()
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{Value: "x", StoreMode: "raw"})
		_ = mb.Put(ctx, "ps:/app/prod/DB_HOST_SUB", backend.PutOptions{Value: "y", StoreMode: "raw"})

		// stdout モードでもビルドエラーは同様に発生する
		cmd, _ := setupJsonizeStdoutCmdWithBuf("ps:/app/prod/")
		err := cmd.Run(appCtx)
		assertErrContains(t, err, "build json")
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
