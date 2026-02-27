package cmd

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/cache"
	"github.com/youyo/bundr/internal/tags"
)

// MockStore は cache.Store インターフェースのテスト用実装。
type MockStore struct {
	ReadFunc            func(backendType string) ([]cache.CacheEntry, error)
	WriteFunc           func(backendType string, entries []cache.CacheEntry) error
	LastRefreshedAtFunc func(backendType string) time.Time
	ReadCalls           []string
	WriteCalls          []WriteCall
}

// WriteCall は Write 呼び出しの記録。
type WriteCall struct {
	BackendType string
	Entries     []cache.CacheEntry
}

func (m *MockStore) Read(backendType string) ([]cache.CacheEntry, error) {
	m.ReadCalls = append(m.ReadCalls, backendType)
	if m.ReadFunc != nil {
		return m.ReadFunc(backendType)
	}
	return nil, cache.ErrCacheNotFound
}

func (m *MockStore) Write(backendType string, entries []cache.CacheEntry) error {
	m.WriteCalls = append(m.WriteCalls, WriteCall{BackendType: backendType, Entries: entries})
	if m.WriteFunc != nil {
		return m.WriteFunc(backendType, entries)
	}
	return nil
}

func (m *MockStore) LastRefreshedAt(backendType string) time.Time {
	if m.LastRefreshedAtFunc != nil {
		return m.LastRefreshedAtFunc(backendType)
	}
	return time.Time{}
}

// cmd-cache-001: cache refresh --prefix ps:/app/prod/ → GetByPrefix が呼ばれ、CacheStore.Write が呼ばれる
func TestCacheRefreshCmd_Success(t *testing.T) {
	mock := backend.NewMockBackend()
	ctx := context.Background()

	// バックエンドにデータを入れておく
	_ = mock.Put(ctx, "ps:/app/prod/DB_HOST", backend.PutOptions{
		Value:     "localhost",
		StoreMode: tags.StoreModeRaw,
	})
	_ = mock.Put(ctx, "ps:/app/prod/DB_PORT", backend.PutOptions{
		Value:     "5432",
		StoreMode: tags.StoreModeRaw,
	})

	mockStore := &MockStore{}

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			return mock, nil
		},
		CacheStore: mockStore,
	}

	cmd := &CacheRefreshCmd{Prefix: "ps:/app/prod/"}
	err := cmd.Run(appCtx)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	// GetByPrefix が呼ばれたことを確認
	if len(mock.GetByPrefixCalls) != 1 {
		t.Errorf("GetByPrefix called %d times, want 1", len(mock.GetByPrefixCalls))
	}
	if got := mock.GetByPrefixCalls[0].Opts.Recursive; !got {
		t.Errorf("GetByPrefix Recursive = %v, want true", got)
	}

	// CacheStore.Write が呼ばれたことを確認
	if len(mockStore.WriteCalls) != 1 {
		t.Fatalf("Write called %d times, want 1", len(mockStore.WriteCalls))
	}
	if got := mockStore.WriteCalls[0].BackendType; got != "ps" {
		t.Errorf("Write backendType = %q, want %q", got, "ps")
	}
	if got := len(mockStore.WriteCalls[0].Entries); got != 2 {
		t.Errorf("Write entries count = %d, want 2", got)
	}
}

// cmd-cache-002: prefix が sm: → エラー（sm: は補完対象外）
func TestCacheRefreshCmd_SMPrefix(t *testing.T) {
	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			t.Fatal("BackendFactory should not be called for sm: prefix")
			return nil, nil
		},
		CacheStore: &MockStore{},
	}

	cmd := &CacheRefreshCmd{Prefix: "sm:my-secret"}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error for sm: prefix, got nil")
	}
}

// cmd-cache-003: AWS エラー時 → エラーを返す
func TestCacheRefreshCmd_AWSError(t *testing.T) {
	awsErr := errors.New("AWS connection refused")
	mockStore := &MockStore{}

	appCtx := &Context{
		BackendFactory: func(_ backend.BackendType) (backend.Backend, error) {
			errBackend := &errorBackend{err: awsErr}
			return errBackend, nil
		},
		CacheStore: mockStore,
	}

	cmd := &CacheRefreshCmd{Prefix: "ps:/app/prod/"}
	err := cmd.Run(appCtx)
	if err == nil {
		t.Error("Run() expected error when AWS fails, got nil")
	}

	// キャッシュには書き込まれないこと
	if len(mockStore.WriteCalls) != 0 {
		t.Errorf("Write called %d times, want 0 (no write on error)", len(mockStore.WriteCalls))
	}
}

// errorBackend は GetByPrefix でエラーを返すテスト用 Backend。
type errorBackend struct {
	err error
}

func (e *errorBackend) Put(_ context.Context, _ string, _ backend.PutOptions) error {
	return e.err
}

func (e *errorBackend) Get(_ context.Context, _ string, _ backend.GetOptions) (string, error) {
	return "", e.err
}

func (e *errorBackend) GetByPrefix(_ context.Context, _ string, _ backend.GetByPrefixOptions) ([]backend.ParameterEntry, error) {
	return nil, e.err
}
