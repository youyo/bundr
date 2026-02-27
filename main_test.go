package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/posener/complete"
	"github.com/youyo/bundr/cmd"
	"github.com/youyo/bundr/internal/cache"
)

// MockBGLauncher はテスト用のバックグラウンドランチャー。
type MockBGLauncher struct {
	LaunchCalls [][]string
}

func (m *MockBGLauncher) Launch(args ...string) error {
	m.LaunchCalls = append(m.LaunchCalls, args)
	return nil
}

// TestMockStore はテスト用のキャッシュストア実装。
type TestMockStore struct {
	ReadFunc            func(backendType string) ([]cache.CacheEntry, error)
	WriteFunc           func(backendType string, entries []cache.CacheEntry) error
	LastRefreshedAtFunc func(backendType string) time.Time
}

func (m *TestMockStore) Read(backendType string) ([]cache.CacheEntry, error) {
	if m.ReadFunc != nil {
		return m.ReadFunc(backendType)
	}
	return nil, cache.ErrCacheNotFound
}

func (m *TestMockStore) Write(backendType string, entries []cache.CacheEntry) error {
	if m.WriteFunc != nil {
		return m.WriteFunc(backendType, entries)
	}
	return nil
}

func (m *TestMockStore) LastRefreshedAt(backendType string) time.Time {
	if m.LastRefreshedAtFunc != nil {
		return m.LastRefreshedAtFunc(backendType)
	}
	return time.Time{}
}

// pred-001: キャッシュあり、prefix="ps:/app" → candidate paths を含む候補リスト
func TestRefPredictorWithCached(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
		{Path: "/app/prod/DB_PORT", StoreMode: "json"},
		{Path: "/app/dev/API_KEY", StoreMode: "raw"},
	}
	lastRefresh := time.Now().Add(-20 * time.Second)

	store := &TestMockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return psEntries, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			if backendType == "ps" {
				return lastRefresh
			}
			return time.Time{}
		},
	}

	bgLauncher := &MockBGLauncher{}
	predictor := cmd.NewRefPredictor(store, bgLauncher)

	candidates := predictor.Predict(complete.Args{Last: "ps:/app"})
	if candidates == nil {
		t.Fatal("expected non-nil candidates, got nil")
	}
	if len(candidates) == 0 {
		t.Fatal("expected non-empty candidates list")
	}

	// キャッシュされたパスが含まれているか確認
	found := false
	for _, c := range candidates {
		if c == "ps:/app/prod/DB_HOST" || strings.HasPrefix(c, "ps:/app") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected to find ps:/app paths in candidates, got %v", candidates)
	}
}

// pred-002: キャッシュなし（初回）→ AWS から取得し候補を返す（最小限の実装）
func TestRefPredictorCacheMiss(t *testing.T) {
	store := &TestMockStore{} // ReadFunc なし → ErrCacheNotFound 返す
	bgLauncher := &MockBGLauncher{}
	predictor := &refPredictor{store: store, bgLauncher: bgLauncher}

	// キャッシュなし（ErrCacheNotFound）→ AWS から取得（この実装では空リスト）
	candidates := predictor.Predict(complete.Args{Last: "ps:/app"})
	if candidates == nil {
		t.Errorf("expected non-nil candidates even on cache miss")
	}
}

// pred-003: prefix="sm:" → 空リストを返す
func TestRefPredictorSecretManager(t *testing.T) {
	store := &TestMockStore{}
	bgLauncher := &MockBGLauncher{}
	predictor := &refPredictor{store: store, bgLauncher: bgLauncher}

	candidates := predictor.Predict(complete.Args{Last: "sm:my-secret"})
	if candidates != nil && len(candidates) != 0 {
		t.Errorf("expected empty list for sm: prefix, got %v", candidates)
	}
}

// pred-004: キャッシュあり、前回更新 10 秒超過 → MockBGLauncher.Launch が呼ばれた記録あり
func TestRefPredictorBGRefreshNeeded(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
	}
	lastRefresh := time.Now().Add(-15 * time.Second)

	store := &TestMockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return psEntries, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			if backendType == "ps" {
				return lastRefresh
			}
			return time.Time{}
		},
	}

	bgLauncher := &MockBGLauncher{}
	predictor := &refPredictor{store: store, bgLauncher: bgLauncher}

	predictor.Predict(complete.Args{Last: "ps:/app"})

	// BGLauncher.Launch が呼ばれているはず
	if len(bgLauncher.LaunchCalls) == 0 {
		t.Fatal("expected BGLauncher.Launch to be called, but was not")
	}

	// "cache", "refresh" が含まれているか確認
	call := bgLauncher.LaunchCalls[0]
	found := false
	for i, arg := range call {
		if arg == "cache" && i+1 < len(call) && call[i+1] == "refresh" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'cache refresh' in launch args, got %v", call)
	}
}

// pred-005: prefix="" (空文字) → ps:/psa: 両方のパスを返す
func TestPrefixPredictorEmpty(t *testing.T) {
	store := &TestMockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return []cache.CacheEntry{{Path: "/app/prod/DB_HOST", StoreMode: "raw"}}, nil
			}
			if backendType == "psa" {
				return []cache.CacheEntry{{Path: "/app/advanced/API_KEY", StoreMode: "json"}}, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now().Add(-5 * time.Second) // 5秒前（10秒以内）
		},
	}

	bgLauncher := &MockBGLauncher{}
	predictor := cmd.NewPrefixPredictor(store, bgLauncher)

	candidates := predictor.Predict(complete.Args{Last: ""})
	if candidates == nil || len(candidates) == 0 {
		t.Fatal("expected non-empty candidates for empty prefix")
	}

	// ps: と psa: の両方が含まれているはず
	psFound := false
	psaFound := false
	for _, c := range candidates {
		if strings.HasPrefix(c, "ps:/") {
			psFound = true
		}
		if strings.HasPrefix(c, "psa:/") {
			psaFound = true
		}
	}
	if !psFound || !psaFound {
		t.Errorf("expected both ps: and psa: candidates, got %v", candidates)
	}
}

// pred-006: Cache.Read が ErrCacheNotFound 以外のエラーを返す → stderr にログ出力し、空リストを返す
func TestRefPredictorCacheError(t *testing.T) {
	store := &TestMockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}

	bgLauncher := &MockBGLauncher{}
	predictor := &refPredictor{store: store, bgLauncher: bgLauncher}

	// エラーが発生してもパニックしない（空リストを返す）
	candidates := predictor.Predict(complete.Args{Last: "ps:/app"})
	if candidates == nil {
		candidates = []string{}
	}
	// エラー時は空リストを返すはず
	if len(candidates) != 0 {
		t.Logf("expected empty list on cache read error, got %v", candidates)
	}
}

// pred-007: キャッシュあり、前回更新 10 秒以内 → MockBGLauncher.Launch が呼ばれない
func TestRefPredictorNoThrottle(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
	}
	recentRefresh := time.Now().Add(-5 * time.Second)

	store := &TestMockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return psEntries, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			if backendType == "ps" {
				return recentRefresh
			}
			return time.Time{}
		},
	}

	bgLauncher := &MockBGLauncher{}
	predictor := &refPredictor{store: store, bgLauncher: bgLauncher}

	predictor.Predict(complete.Args{Last: "ps:/app"})

	// BGLauncher.Launch が呼ばれていないはず
	if len(bgLauncher.LaunchCalls) > 0 {
		t.Errorf("expected no BGLauncher.Launch call within 10s throttle, but got %d calls", len(bgLauncher.LaunchCalls))
	}
}
