package cache

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

const lockTimeout = 5 * time.Second

// withExclusiveLock はファイルへの排他ロックを取得して fn を実行する。
// ロック取得待ちタイムアウトは 5 秒。ロックファイルは lockPath に作成される。
func withExclusiveLock(lockPath string, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close()

	deadline := time.Now().Add(lockTimeout)
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("cache lock timeout: %w", err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}
