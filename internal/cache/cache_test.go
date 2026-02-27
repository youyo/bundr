package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// ---- 正常系テスト ----

// cache-001: Write → Read でエントリが読める
func TestFileStore_WriteRead(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())
	entries := []CacheEntry{
		{Path: "/app/prod/DB_HOST", StoreMode: "raw"},
		{Path: "/app/prod/DB_PORT", StoreMode: "raw"},
	}

	if err := store.Write("ps", entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(got))
	}
	for i, e := range entries {
		if got[i].Path != e.Path || got[i].StoreMode != e.StoreMode {
			t.Errorf("entry[%d]: expected %+v, got %+v", i, e, got[i])
		}
	}
}

// cache-002: Write 後のファイル内容が JSON スキーマに準拠している
func TestFileStore_Write_JSONSchema(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)
	entries := []CacheEntry{{Path: "/app/config", StoreMode: "json"}}

	if err := store.Write("ps", entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "ps.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var cf CacheFile
	if err := json.Unmarshal(data, &cf); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cf.SchemaVersion != SchemaVersion {
		t.Errorf("schema_version: expected %q, got %q", SchemaVersion, cf.SchemaVersion)
	}
	if cf.BackendType != "ps" {
		t.Errorf("backend_type: expected %q, got %q", "ps", cf.BackendType)
	}
	if cf.UpdatedAt.IsZero() {
		t.Error("updated_at should not be zero")
	}
	if cf.LastRefreshedAt.IsZero() {
		t.Error("last_refreshed_at should not be zero")
	}
}

// cache-003: キャッシュ不在で Read → ErrCacheNotFound
func TestFileStore_Read_NotFound(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())

	_, err := store.Read("ps")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != ErrCacheNotFound {
		t.Fatalf("expected ErrCacheNotFound, got %v", err)
	}
}

// cache-004: Write → {backendType}.json として保存される
func TestFileStore_Write_Filename(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	if err := store.Write("psa", []CacheEntry{{Path: "/app/x", StoreMode: "raw"}}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "psa.json")); err != nil {
		t.Fatalf("expected psa.json to exist: %v", err)
	}
}

// cache-005: ディレクトリが存在しない場合も自動作成して書き込める
func TestFileStore_Write_AutoCreateDir(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "subdir", "bundr")
	store := NewFileStoreWithDir(dir)

	err := store.Write("ps", []CacheEntry{{Path: "/app/x", StoreMode: "raw"}})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "ps.json")); statErr != nil {
		t.Fatalf("file not created: %v", statErr)
	}
}

// cache-006: entries が空スライスでも正常に書き込める
func TestFileStore_Write_EmptyEntries(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())

	if err := store.Write("ps", []CacheEntry{}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 entries, got %d", len(got))
	}
}

// cache-007: Write → LastRefreshedAt が記録されている
func TestFileStore_LastRefreshedAt_AfterWrite(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())
	before := time.Now().Add(-time.Second)

	if err := store.Write("ps", []CacheEntry{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	ts := store.LastRefreshedAt("ps")
	if ts.IsZero() {
		t.Fatal("LastRefreshedAt should not be zero after Write")
	}
	if ts.Before(before) {
		t.Errorf("LastRefreshedAt %v is before write time %v", ts, before)
	}
}

// ---- 異常系テスト ----

// cache-010: 破損 JSON ファイルを Read → エラーを返す（panic しない、ErrCacheNotFound とは区別）
func TestFileStore_Read_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	// 破損 JSON を書き込む
	if err := os.WriteFile(filepath.Join(dir, "ps.json"), []byte("{{broken json{{"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := store.Read("ps")
	if err == nil {
		t.Fatal("expected error for corrupted JSON, got nil")
	}
	if err == ErrCacheNotFound {
		t.Fatal("corrupted JSON should NOT return ErrCacheNotFound")
	}
}

// cache-011: 書き込み権限のないディレクトリで Write → error を返す
func TestFileStore_Write_NoPermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, skipping permission test")
	}
	dir := t.TempDir()
	// ディレクトリを読み取り専用にする
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755) //nolint:errcheck

	store := NewFileStoreWithDir(dir)
	err := store.Write("ps", []CacheEntry{})
	if err == nil {
		t.Fatal("expected error for read-only directory, got nil")
	}
}

// cache-012: スキーマバージョン不一致のキャッシュを Read → ErrCacheNotFound 相当のエラー
func TestFileStore_Read_SchemaMismatch(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	// 別バージョンのキャッシュを書き込む
	cf := CacheFile{
		SchemaVersion:   "v999",
		BackendType:     "ps",
		UpdatedAt:       time.Now(),
		LastRefreshedAt: time.Now(),
		Entries:         []CacheEntry{},
	}
	data, _ := json.Marshal(cf)
	if err := os.WriteFile(filepath.Join(dir, "ps.json"), data, 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := store.Read("ps")
	if err == nil {
		t.Fatal("expected error for schema mismatch, got nil")
	}
	// ErrCacheNotFound または専用エラーどちらでも可
}

// cache-013: 破損ファイルを Read した場合、エラーを返す（補完継続できる）
func TestFileStore_Read_CorruptedFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	if err := os.WriteFile(filepath.Join(dir, "ps.json"), []byte("not json at all"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := store.Read("ps")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// ---- エッジケーステスト ----

// cache-020: 並行 Write x 5 ゴルーチン → ファイル破損なし
func TestFileStore_Write_Concurrent(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())
	const workers = 5
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		i := i
		go func() {
			defer wg.Done()
			entries := []CacheEntry{
				{Path: strings.Repeat("/app/path", i+1), StoreMode: "raw"},
			}
			if err := store.Write("ps", entries); err != nil {
				t.Errorf("worker %d: Write: %v", i, err)
			}
		}()
	}
	wg.Wait()

	// 読み込んでファイル破損がないことを確認
	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read after concurrent Write: %v", err)
	}
	// 最後の Write のエントリが入っているはず
	if len(got) == 0 {
		t.Fatal("expected non-empty entries after concurrent Write")
	}
}

// cache-021: 非常に長いパス（1024文字）でも正常に書き込み・読み込みできる
func TestFileStore_Write_LongPath(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())
	longPath := "/" + strings.Repeat("a", 1020)
	entries := []CacheEntry{{Path: longPath, StoreMode: "raw"}}

	if err := store.Write("ps", entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 || got[0].Path != longPath {
		t.Errorf("unexpected entries: %+v", got)
	}
}

// cache-022: エントリ数 10000 件でも正常に処理できる
func TestFileStore_Write_ManyEntries(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())
	entries := make([]CacheEntry, 10000)
	for i := range entries {
		entries[i] = CacheEntry{Path: strings.Repeat("/app/", 1) + string(rune('a'+i%26)), StoreMode: "raw"}
	}

	if err := store.Write("ps", entries); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 10000 {
		t.Errorf("expected 10000 entries, got %d", len(got))
	}
}

// ---- スロットリングテスト ----

// cache-030: Write 後 5 秒以内に LastRefreshedAt が 10 秒未満
func TestFileStore_LastRefreshedAt_Within10Seconds(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())

	if err := store.Write("ps", []CacheEntry{}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	ts := store.LastRefreshedAt("ps")
	age := time.Since(ts)
	if age >= 10*time.Second {
		t.Errorf("LastRefreshedAt is %v old, expected < 10s", age)
	}
}

// LastRefreshedAt: キャッシュが存在しない場合は zero time を返す
func TestFileStore_LastRefreshedAt_NotFound(t *testing.T) {
	store := NewFileStoreWithDir(t.TempDir())

	ts := store.LastRefreshedAt("ps")
	if !ts.IsZero() {
		t.Errorf("expected zero time for non-existent cache, got %v", ts)
	}
}

// NewFileStore のデフォルトディレクトリが設定されること
func TestNewFileStore(t *testing.T) {
	s, err := NewFileStore()
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil FileStore")
	}
}

// ファイルが存在するが読み取り権限がない場合、ErrCacheNotFound とは区別されたエラーを返す
func TestFileStore_Read_NoReadPermission(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("running as root, skipping permission test")
	}
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	// 有効な JSON ファイルを作成してから権限を除去
	cacheFile := filepath.Join(dir, "ps.json")
	if err := os.WriteFile(cacheFile, []byte(`{"schema_version":"v1","backend_type":"ps","entries":[]}`), 0o000); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := store.Read("ps")
	if err == nil {
		t.Fatal("expected error for unreadable file, got nil")
	}
	if err == ErrCacheNotFound {
		t.Fatal("unreadable file should NOT return ErrCacheNotFound")
	}
}

// ---- セキュリティテスト ----

// セキュリティ要件: キャッシュファイルに秘密情報（値）が保存されないことを確認
// CacheEntry は Path と StoreMode のみを持ち、Value フィールドは存在しない設計。
func TestFileStore_Write_DoesNotStoreSecretValues(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	// センシティブな名前のパスを含むエントリ（値は含まれない）
	entries := []CacheEntry{
		{Path: "/app/prod/DB_PASSWORD", StoreMode: "raw"},
		{Path: "/app/prod/API_SECRET", StoreMode: "raw"},
	}

	if err := store.Write("ps", entries); err != nil {
		t.Fatalf("Write: %v", err)
	}

	// JSON ファイルを直接確認して "value" キーがないことをアサート
	data, err := os.ReadFile(filepath.Join(dir, "ps.json"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	entriesRaw, ok := raw["entries"].([]interface{})
	if !ok {
		t.Fatal("entries not found in JSON")
	}

	// 各エントリに "value" フィールドがないことを確認
	for i, e := range entriesRaw {
		m, ok := e.(map[string]interface{})
		if !ok {
			t.Errorf("entry[%d] is not an object", i)
			continue
		}
		if _, hasValue := m["value"]; hasValue {
			t.Errorf("entry[%d] contains 'value' field — secret values must never be cached", i)
		}
		// path と store_mode のみが存在するはず
		if _, hasPath := m["path"]; !hasPath {
			t.Errorf("entry[%d] missing 'path' field", i)
		}
		if _, hasStoreMode := m["store_mode"]; !hasStoreMode {
			t.Errorf("entry[%d] missing 'store_mode' field", i)
		}
	}
}

// ---- NoopStore テスト ----

// NoopStore: NewNoopStore が非 nil を返す
func TestNoopStore_New(t *testing.T) {
	s := NewNoopStore()
	if s == nil {
		t.Fatal("expected non-nil NoopStore")
	}
}

// NoopStore: Read は常に ErrCacheNotFound を返す
func TestNoopStore_Read(t *testing.T) {
	s := NewNoopStore()
	_, err := s.Read("ps")
	if err != ErrCacheNotFound {
		t.Errorf("expected ErrCacheNotFound, got %v", err)
	}
}

// NoopStore: Write は常に nil を返す
func TestNoopStore_Write(t *testing.T) {
	s := NewNoopStore()
	err := s.Write("ps", []CacheEntry{{Path: "/app/key", StoreMode: "raw"}})
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// NoopStore: LastRefreshedAt は常に zero time を返す
func TestNoopStore_LastRefreshedAt(t *testing.T) {
	s := NewNoopStore()
	ts := s.LastRefreshedAt("ps")
	if !ts.IsZero() {
		t.Errorf("expected zero time, got %v", ts)
	}
}

// NoopStore: Store インターフェースを実装している
func TestNoopStore_ImplementsStore(t *testing.T) {
	var _ Store = &NoopStore{}
}

// Write ではアトミック更新（一時ファイル経由）が行われることを確認
func TestFileStore_Write_IsAtomic(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStoreWithDir(dir)

	// 1回目の書き込み
	entries1 := []CacheEntry{{Path: "/app/v1/key", StoreMode: "raw"}}
	if err := store.Write("ps", entries1); err != nil {
		t.Fatalf("Write 1: %v", err)
	}

	// 2回目の書き込み（上書き）
	entries2 := []CacheEntry{{Path: "/app/v2/key", StoreMode: "json"}}
	if err := store.Write("ps", entries2); err != nil {
		t.Fatalf("Write 2: %v", err)
	}

	// 最終的に 2 回目の内容が読めること
	got, err := store.Read("ps")
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 || got[0].Path != "/app/v2/key" {
		t.Errorf("expected v2 entry after overwrite, got %+v", got)
	}

	// 一時ファイルが残っていないことを確認
	globPattern := filepath.Join(dir, ".tmp-ps-*")
	matches, _ := filepath.Glob(globPattern)
	if len(matches) > 0 {
		t.Errorf("temporary files left behind: %v", matches)
	}
}
