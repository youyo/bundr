package cache

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"
	"time"
)

// lock-001: withExclusiveLock — 正常ケース
func TestWithExclusiveLock_Normal(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	called := false
	err := withExclusiveLock(lockPath, func() error {
		called = true
		return nil
	})

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
}

// lock-002: withExclusiveLock — fn がエラーを返す場合、エラーが伝播しロックは解放される
func TestWithExclusiveLock_FnError(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	expectedErr := errors.New("fn error")
	err := withExclusiveLock(lockPath, func() error {
		return expectedErr
	})

	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}

	// ロックが解放されていることを確認: 別のロックが取得できる
	f, err2 := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err2 != nil {
		t.Fatalf("failed to open lock file: %v", err2)
	}
	defer f.Close()

	// 非ブロッキングで排他ロックを試みる
	lockErr := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if lockErr != nil {
		t.Fatalf("lock should be released after fn error, but got: %v", lockErr)
	}
	// 取得したロックを解放
	_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

// lock-003: 排他ロック中に別ゴルーチンが待機してから取得できる
func TestWithExclusiveLock_Concurrent(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "test.lock")

	var mu sync.Mutex
	var order []int

	ready := make(chan struct{})
	done := make(chan struct{})

	// ゴルーチン1: ロックを取得して少し待つ
	go func() {
		err := withExclusiveLock(lockPath, func() error {
			mu.Lock()
			order = append(order, 1)
			mu.Unlock()

			// ゴルーチン2に開始を通知
			close(ready)

			// 少し待つ（ゴルーチン2がロック待機になるよう）
			time.Sleep(50 * time.Millisecond)

			mu.Lock()
			order = append(order, 1) // 1がロック中を2回記録
			mu.Unlock()
			return nil
		})
		if err != nil {
			t.Errorf("goroutine 1: unexpected error: %v", err)
		}
		close(done)
	}()

	// ゴルーチン1がロックを取得するまで待つ
	<-ready

	// ゴルーチン2: ロックを待機してから取得
	err := withExclusiveLock(lockPath, func() error {
		mu.Lock()
		order = append(order, 2)
		mu.Unlock()
		return nil
	})
	if err != nil {
		t.Fatalf("goroutine 2: unexpected error: %v", err)
	}

	// ゴルーチン1の完了を待つ
	<-done

	// 順序を確認: [1, 1, 2] であるべき
	mu.Lock()
	defer mu.Unlock()
	if len(order) != 3 {
		t.Fatalf("expected 3 entries in order, got %d: %v", len(order), order)
	}
	if order[0] != 1 || order[1] != 1 || order[2] != 2 {
		t.Fatalf("unexpected order: %v (expected [1, 1, 2])", order)
	}
}
