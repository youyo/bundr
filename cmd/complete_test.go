package cmd

import (
	"testing"
)

// cmd-complete-001 / cmd-complete-002:
// CLI 構造体に Completion フィールド（CompletionCmd）が正しく定義され、
// go build が通ることを確認するコンパイルテストを行う。

// TestCLI_HasCompletionCmd は CLI 構造体に Completion フィールドが存在することを確認する。
func TestCLI_HasCompletionCmd(t *testing.T) {
	cli := &CLI{}
	// Completion フィールドのゼロ値が存在することを確認
	// (コンパイルが通ることが本質的なテスト)
	_ = cli.Completion
}

// TestCLI_HasCacheCmd は CLI 構造体に Cache フィールドが存在することを確認する。
func TestCLI_HasCacheCmd(t *testing.T) {
	cli := &CLI{}
	_ = cli.Cache
}

// TestContext_HasCacheStore は Context 構造体に CacheStore フィールドが存在することを確認する。
func TestContext_HasCacheStore(t *testing.T) {
	ctx := &Context{}
	// CacheStore フィールドが nil でも panic しないこと
	_ = ctx.CacheStore
}

// TestContext_HasBGLauncher は Context 構造体に BGLauncher フィールドが存在することを確認する。
func TestContext_HasBGLauncher(t *testing.T) {
	ctx := &Context{}
	_ = ctx.BGLauncher
}

// TestExecBGLauncher_Implements は ExecBGLauncher が BGLauncher インターフェースを実装することを確認する。
func TestExecBGLauncher_Implements(t *testing.T) {
	var _ BGLauncher = &ExecBGLauncher{}
}
