package cmd

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/posener/complete"
	"github.com/youyo/bundr/internal/cache"
)

// MockBGLauncher はテスト用のバックグラウンドランチャー（cmd パッケージ内）。
type MockBGLauncher struct {
	LaunchCalls [][]string
}

func (m *MockBGLauncher) Launch(args ...string) error {
	m.LaunchCalls = append(m.LaunchCalls, args)
	return nil
}

// pred-cmd-001: RefPredictor - キャッシュあり、10秒超過 → フィルタリング済み候補 + BG 起動
func TestNewRefPredictor_CacheHitBGRefresh(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
		{Path: "/app/prod/DB_PORT", StoreMode: "json"},
		{Path: "/other/key", StoreMode: "raw"},
	}
	lastRefresh := time.Now().Add(-20 * time.Second)

	store := &MockStore{
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

	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 2 {
		t.Errorf("expected 2 candidates, got %d: %v", len(candidates), candidates)
	}
	for _, c := range candidates {
		if !strings.HasPrefix(c, "ps:/app") {
			t.Errorf("unexpected candidate: %s", c)
		}
	}

	if len(bg.LaunchCalls) == 0 {
		t.Error("expected BGLauncher to be called (>10s throttle)")
	}
}

// pred-cmd-002: RefPredictor - ErrCacheNotFound → 空リスト
func TestNewRefPredictor_CacheMiss(t *testing.T) {
	store := &MockStore{} // ReadFunc なし → ErrCacheNotFound
	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("ps:/app")

	if candidates == nil {
		t.Error("expected non-nil slice")
	}
	if len(candidates) != 0 {
		t.Errorf("expected empty slice on cache miss, got %v", candidates)
	}
}

// pred-cmd-003: RefPredictor - sm: → 空リスト
func TestNewRefPredictor_SecretManager(t *testing.T) {
	store := &MockStore{}
	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("sm:my-secret")

	if len(candidates) != 0 {
		t.Errorf("expected empty list for sm:, got %v", candidates)
	}
}

// pred-cmd-004: RefPredictor - 非 ErrCacheNotFound エラー → 空リスト
func TestNewRefPredictor_CacheError(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}
	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 0 {
		t.Errorf("expected empty list on cache error, got %v", candidates)
	}
}

// pred-cmd-005: RefPredictor - 10 秒以内 → BG 起動しない
func TestNewRefPredictor_Throttle(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/key", StoreMode: "raw"},
	}
	recentRefresh := time.Now().Add(-5 * time.Second)

	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return psEntries, nil
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return recentRefresh
		},
	}

	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	fn("ps:/app")

	if len(bg.LaunchCalls) != 0 {
		t.Errorf("expected no BGLauncher call within 10s throttle, got %d", len(bg.LaunchCalls))
	}
}

// pred-cmd-006: RefPredictor - 無効な prefix → 空リスト
func TestNewRefPredictor_InvalidPrefix(t *testing.T) {
	store := &MockStore{}
	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("invalid-prefix")

	if len(candidates) != 0 {
		t.Errorf("expected empty list for invalid prefix, got %v", candidates)
	}
}

// pred-cmd-007: PrefixPredictor - 空文字 → ps: と psa: の両パスを返す（スロットリング内）
func TestNewPrefixPredictor_EmptyPrefix(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return []cache.CacheEntry{{Path: "/app/prod/KEY", StoreMode: "raw"}}, nil
			}
			if backendType == "psa" {
				return []cache.CacheEntry{{Path: "/app/advanced/KEY", StoreMode: "json"}}, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now().Add(-5 * time.Second)
		},
	}

	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("")

	if len(candidates) == 0 {
		t.Fatal("expected non-empty candidates for empty prefix")
	}

	psFound, psaFound := false, false
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

	if len(bg.LaunchCalls) != 0 {
		t.Errorf("expected no BG launch within 10s throttle, got %d calls", len(bg.LaunchCalls))
	}
}

// pred-cmd-008: PrefixPredictor - 空文字、10秒超過 → BG 起動あり
func TestNewPrefixPredictor_EmptyPrefixBGRefresh(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return []cache.CacheEntry{{Path: "/app/key", StoreMode: "raw"}}, nil
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now().Add(-20 * time.Second)
		},
	}

	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	fn("")

	if len(bg.LaunchCalls) < 2 {
		t.Errorf("expected 2 BG launches for ps: and psa:, got %d", len(bg.LaunchCalls))
	}
}

// pred-cmd-009: PrefixPredictor - 空文字、片方が ErrCacheNotFound → 残りのキャッシュのみ返す
func TestNewPrefixPredictor_EmptyPrefixPartialCache(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return []cache.CacheEntry{{Path: "/app/key", StoreMode: "raw"}}, nil
			}
			return nil, cache.ErrCacheNotFound // psa はキャッシュなし
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now()
		},
	}

	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("")

	for _, c := range candidates {
		if !strings.HasPrefix(c, "ps:/") {
			t.Errorf("unexpected non-ps candidate: %s", c)
		}
	}
}

// pred-cmd-010: PrefixPredictor - 空文字、キャッシュエラー → パニックせず返る
func TestNewPrefixPredictor_EmptyCacheError(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return nil, fmt.Errorf("disk full")
		},
	}
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	// エラー時もパニックせず空リストが返る（nil は空スライスとして扱う）
	candidates := fn("")
	if len(candidates) != 0 {
		t.Errorf("expected empty list on cache error, got %v", candidates)
	}
}

// pred-cmd-011: PrefixPredictor - prefix="ps:/app" → フィルタリング済み候補
func TestNewPrefixPredictor_WithPrefix(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
		{Path: "/other/key", StoreMode: "raw"},
	}

	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "ps" {
				return psEntries, nil
			}
			return nil, cache.ErrCacheNotFound
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now().Add(-5 * time.Second)
		},
	}

	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("ps:/app")

	for _, c := range candidates {
		if !strings.HasPrefix(c, "ps:/app") {
			t.Errorf("unexpected candidate not matching ps:/app: %s", c)
		}
	}
}

// pred-cmd-012: PrefixPredictor - prefix="ps:/app"、10秒超過 → BG 起動あり
func TestNewPrefixPredictor_WithPrefixBGRefresh(t *testing.T) {
	psEntries := []cache.CacheEntry{
		{Path: "/app/key", StoreMode: "raw"},
	}
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return psEntries, nil
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now().Add(-20 * time.Second)
		},
	}
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	fn("ps:/app")

	if len(bg.LaunchCalls) == 0 {
		t.Error("expected BG launch for >10s throttle")
	}
}

// pred-cmd-013: PrefixPredictor - prefix="sm:..." → 空リスト
func TestNewPrefixPredictor_SecretManager(t *testing.T) {
	store := &MockStore{}
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("sm:my-secret")

	if len(candidates) != 0 {
		t.Errorf("expected empty list for sm:, got %v", candidates)
	}
}

// pred-cmd-014: PrefixPredictor - ErrCacheNotFound → 空リスト
func TestNewPrefixPredictor_CacheMiss(t *testing.T) {
	store := &MockStore{} // ReadFunc なし → ErrCacheNotFound
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 0 {
		t.Errorf("expected empty list on cache miss, got %v", candidates)
	}
}

// pred-cmd-015: PrefixPredictor - 非 ErrCacheNotFound エラー → 空リスト
func TestNewPrefixPredictor_CacheError(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return nil, fmt.Errorf("permission denied")
		},
	}
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 0 {
		t.Errorf("expected empty list on cache error, got %v", candidates)
	}
}

// pred-cmd-T4: RefPredictor - ErrCacheNotFound → 空リスト + BG 起動
func TestNewRefPredictor_CacheMiss_LaunchesBG(t *testing.T) {
	store := &MockStore{} // ReadFunc なし → ErrCacheNotFound
	bg := &MockBGLauncher{}
	fn := newRefPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 0 {
		t.Errorf("expected empty slice on cache miss, got %v", candidates)
	}
	if len(bg.LaunchCalls) == 0 {
		t.Error("expected BGLauncher to be called on ErrCacheNotFound")
	}
}

// pred-cmd-T5: PrefixPredictor - prefix 指定、ErrCacheNotFound → 空リスト + BG 起動
func TestNewPrefixPredictor_CacheMiss_LaunchesBG(t *testing.T) {
	store := &MockStore{} // ReadFunc なし → ErrCacheNotFound
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("ps:/app")

	if len(candidates) != 0 {
		t.Errorf("expected empty slice on cache miss, got %v", candidates)
	}
	if len(bg.LaunchCalls) == 0 {
		t.Error("expected BGLauncher to be called on ErrCacheNotFound")
	}
}

// pred-cmd-T6: PrefixPredictor - 空文字、片方が ErrCacheNotFound → 残りのキャッシュ返す + missing 側の BG 起動
func TestNewPrefixPredictor_EmptyPrefixPartialCache_LaunchesBG(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			if backendType == "psa" {
				return []cache.CacheEntry{{Path: "/app/advanced/KEY", StoreMode: "json"}}, nil
			}
			return nil, cache.ErrCacheNotFound // ps はキャッシュなし
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now() // 最近更新済み（スロットリング内）
		},
	}

	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("")

	// psa の候補のみ返る
	if len(candidates) == 0 {
		t.Fatal("expected non-empty candidates from psa")
	}
	for _, c := range candidates {
		if !strings.HasPrefix(c, "psa:/") {
			t.Errorf("unexpected non-psa candidate: %s", c)
		}
	}

	// ps の ErrCacheNotFound で BG 起動が発生していること
	if len(bg.LaunchCalls) == 0 {
		t.Error("expected BGLauncher to be called for missing ps cache")
	}
	// psa はキャッシュあり（スロットリング内）なので BG 起動は ps の 1 回のみ
	if len(bg.LaunchCalls) > 1 {
		t.Errorf("expected exactly 1 BG launch (for ps), got %d", len(bg.LaunchCalls))
	}
}

// pred-cmd-016: PrefixPredictor - 無効な prefix → 空リスト
func TestNewPrefixPredictor_InvalidPrefix(t *testing.T) {
	store := &MockStore{}
	bg := &MockBGLauncher{}
	fn := newPrefixPredictor(store, bg)
	candidates := fn("invalid-prefix")

	if len(candidates) != 0 {
		t.Errorf("expected empty list for invalid prefix, got %v", candidates)
	}
}

// pred-cmd-017: NewRefPredictor - complete.Predictor として動作する
func TestNewRefPredictor_AsPredictor(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return []cache.CacheEntry{{Path: "/app/key", StoreMode: "raw"}}, nil
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now()
		},
	}
	bg := &MockBGLauncher{}
	predictor := NewRefPredictor(store, bg)

	result := predictor.Predict(complete.Args{Last: "ps:/app"})
	if result == nil {
		t.Error("expected non-nil result")
	}
}

// pred-cmd-018: NewPrefixPredictor - complete.Predictor として動作する
func TestNewPrefixPredictor_AsPredictor(t *testing.T) {
	store := &MockStore{
		ReadFunc: func(backendType string) ([]cache.CacheEntry, error) {
			return []cache.CacheEntry{{Path: "/app/key", StoreMode: "raw"}}, nil
		},
		LastRefreshedAtFunc: func(backendType string) time.Time {
			return time.Now()
		},
	}
	bg := &MockBGLauncher{}
	predictor := NewPrefixPredictor(store, bg)

	result := predictor.Predict(complete.Args{Last: ""})
	if result == nil {
		t.Error("expected non-nil result")
	}
}
