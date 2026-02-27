package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// SchemaVersion はキャッシュファイルのスキーマバージョン。
const SchemaVersion = "v1"

// ErrCacheNotFound はキャッシュが存在しない場合のエラー。
var ErrCacheNotFound = errors.New("cache not found")

// CacheFile はキャッシュファイル全体を表す。
type CacheFile struct {
	SchemaVersion   string       `json:"schema_version"`
	BackendType     string       `json:"backend_type"`
	UpdatedAt       time.Time    `json:"updated_at"`
	LastRefreshedAt time.Time    `json:"last_refreshed_at"`
	Entries         []CacheEntry `json:"entries"`
}

// CacheEntry はキャッシュ内の 1 エントリ（パスとメタデータ）。
// 値（secret）は絶対に保存しない。
type CacheEntry struct {
	Path      string `json:"path"`
	StoreMode string `json:"store_mode"`
}

// Store はキャッシュの読み書きインターフェース（テスト容易性のため）。
type Store interface {
	Read(backendType string) ([]CacheEntry, error)
	Write(backendType string, entries []CacheEntry) error
	// LastRefreshedAt は指定バックエンドの最終 BG 更新時刻を返す。
	// キャッシュが存在しない場合は zero time を返す。
	LastRefreshedAt(backendType string) time.Time
}

// FileStore は ~/.cache/bundr/ へのファイルベースの実装。
type FileStore struct {
	baseDir string
}

// xdgCacheDir は XDG Base Directory Specification に従ってキャッシュディレクトリを返す。
// $XDG_CACHE_HOME が絶対パスで設定されていればそれを使用し、
// 未設定または相対パスの場合は $HOME/.cache を返す。
// これにより macOS でも Linux と同様に ~/.cache/bundr/ が使われる。
func xdgCacheDir() (string, error) {
	if d := os.Getenv("XDG_CACHE_HOME"); d != "" && filepath.IsAbs(d) {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(home, ".cache"), nil
}

// NewFileStore はデフォルトキャッシュディレクトリを使用する FileStore を返す。
func NewFileStore() (*FileStore, error) {
	cacheDir, err := xdgCacheDir()
	if err != nil {
		return nil, fmt.Errorf("get user cache dir: %w", err)
	}
	return &FileStore{baseDir: filepath.Join(cacheDir, "bundr")}, nil
}

// NewFileStoreWithDir はテスト用にカスタムディレクトリを指定できる。
func NewFileStoreWithDir(dir string) *FileStore {
	return &FileStore{baseDir: dir}
}

// Read は指定バックエンドのキャッシュエントリを読み込む。
// キャッシュが存在しない場合は (nil, ErrCacheNotFound) を返す。
// ErrCacheNotFound 以外のエラー（ファイル破損・権限エラー等）も error として返す。
func (s *FileStore) Read(backendType string) ([]CacheEntry, error) {
	cf, err := s.readFile(backendType)
	if err != nil {
		return nil, err
	}
	return cf.Entries, nil
}

// Write はエントリをキャッシュファイルにアトミックに書き込む。
// ファイルロックを取得してから書き込み、ロック解放する。
func (s *FileStore) Write(backendType string, entries []CacheEntry) error {
	if err := os.MkdirAll(s.baseDir, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	target := filepath.Join(s.baseDir, backendType+".json")
	lockPath := target + ".lock"

	return withExclusiveLock(lockPath, func() error {
		tmp, err := os.CreateTemp(s.baseDir, ".tmp-"+backendType+"-")
		if err != nil {
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath) // 失敗時のクリーンアップ

		now := time.Now().UTC()
		cf := CacheFile{
			SchemaVersion:   SchemaVersion,
			BackendType:     backendType,
			UpdatedAt:       now,
			LastRefreshedAt: now,
			Entries:         entries,
		}
		if err := json.NewEncoder(tmp).Encode(cf); err != nil {
			tmp.Close()
			return fmt.Errorf("encode cache: %w", err)
		}
		if err := tmp.Close(); err != nil {
			return fmt.Errorf("close temp file: %w", err)
		}
		if err := os.Rename(tmpPath, target); err != nil {
			return fmt.Errorf("rename cache file: %w", err)
		}
		return nil
	})
}

// LastRefreshedAt は指定バックエンドのキャッシュから last_refreshed_at を返す。
// キャッシュが存在しない場合やエラーの場合は zero time を返す（エラーは無視）。
func (s *FileStore) LastRefreshedAt(backendType string) time.Time {
	cf, err := s.readFile(backendType)
	if err != nil {
		return time.Time{}
	}
	return cf.LastRefreshedAt
}

// NoopStore は何もしない Store 実装。
// キャッシュ初期化に失敗した場合（HOME 未設定など）のフォールバックとして使用し、
// 補完機能を無効にしつつ put/get/export 等の通常 CLI 動作を継続させる。
type NoopStore struct{}

// NewNoopStore は NoopStore を返す。
func NewNoopStore() *NoopStore { return &NoopStore{} }

// Read は常に ErrCacheNotFound を返す。
func (n *NoopStore) Read(_ string) ([]CacheEntry, error) { return nil, ErrCacheNotFound }

// Write は何もせずに成功を返す。
func (n *NoopStore) Write(_ string, _ []CacheEntry) error { return nil }

// LastRefreshedAt は常に zero time を返す。
func (n *NoopStore) LastRefreshedAt(_ string) time.Time { return time.Time{} }

// readFile は JSON キャッシュファイルを読み込んで CacheFile を返す内部ヘルパー。
func (s *FileStore) readFile(backendType string) (*CacheFile, error) {
	path := filepath.Join(s.baseDir, backendType+".json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheNotFound
		}
		return nil, fmt.Errorf("read cache file: %w", err)
	}

	var cf CacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse cache file: %w", err)
	}

	if cf.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("cache schema version mismatch: got %q, want %q", cf.SchemaVersion, SchemaVersion)
	}

	return &cf, nil
}
