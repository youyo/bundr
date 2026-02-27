package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// 環境変数をクリーンにする
	t.Setenv("BUNDR_AWS_REGION", "")
	t.Setenv("BUNDR_AWS_PROFILE", "")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg == nil {
		t.Fatal("Load() returned nil config")
	}

	// デフォルト値の確認
	if cfg.AWS.Region != "" {
		t.Errorf("expected empty region, got %q", cfg.AWS.Region)
	}
	if cfg.AWS.Profile != "" {
		t.Errorf("expected empty profile, got %q", cfg.AWS.Profile)
	}
	if cfg.AWS.KMSKeyID != "" {
		t.Errorf("expected empty kms_key_id, got %q", cfg.AWS.KMSKeyID)
	}
}

func TestLoadFromEnvVars(t *testing.T) {
	t.Setenv("BUNDR_AWS_REGION", "ap-northeast-1")
	t.Setenv("BUNDR_AWS_PROFILE", "myprofile")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "alias/mykey")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.AWS.Region != "ap-northeast-1" {
		t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
	}
	if cfg.AWS.Profile != "myprofile" {
		t.Errorf("expected profile %q, got %q", "myprofile", cfg.AWS.Profile)
	}
	if cfg.AWS.KMSKeyID != "alias/mykey" {
		t.Errorf("expected kms_key_id %q, got %q", "alias/mykey", cfg.AWS.KMSKeyID)
	}
}

func TestLoadFromProjectConfig(t *testing.T) {
	// 一時ディレクトリに .bundr.toml を作成
	tmpDir := t.TempDir()
	configContent := []byte(`[aws]
region = "us-west-2"
profile = "project-profile"
kms_key_id = "alias/project-key"
`)
	err := os.WriteFile(filepath.Join(tmpDir, ".bundr.toml"), configContent, 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// 環境変数をクリーン
	t.Setenv("BUNDR_AWS_REGION", "")
	t.Setenv("BUNDR_AWS_PROFILE", "")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "")

	cfg, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir() returned error: %v", err)
	}

	if cfg.AWS.Region != "us-west-2" {
		t.Errorf("expected region %q, got %q", "us-west-2", cfg.AWS.Region)
	}
	if cfg.AWS.Profile != "project-profile" {
		t.Errorf("expected profile %q, got %q", "project-profile", cfg.AWS.Profile)
	}
	if cfg.AWS.KMSKeyID != "alias/project-key" {
		t.Errorf("expected kms_key_id %q, got %q", "alias/project-key", cfg.AWS.KMSKeyID)
	}
}

func TestEnvVarsOverrideProjectConfig(t *testing.T) {
	// 一時ディレクトリに .bundr.toml を作成
	tmpDir := t.TempDir()
	configContent := []byte(`[aws]
region = "us-west-2"
profile = "project-profile"
`)
	err := os.WriteFile(filepath.Join(tmpDir, ".bundr.toml"), configContent, 0644)
	if err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	// 環境変数で region をオーバーライド
	t.Setenv("BUNDR_AWS_REGION", "eu-west-1")
	t.Setenv("BUNDR_AWS_PROFILE", "")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "")

	cfg, err := LoadFromDir(tmpDir)
	if err != nil {
		t.Fatalf("LoadFromDir() returned error: %v", err)
	}

	// 環境変数が優先
	if cfg.AWS.Region != "eu-west-1" {
		t.Errorf("expected region %q, got %q", "eu-west-1", cfg.AWS.Region)
	}
	// ファイルの値が残る
	if cfg.AWS.Profile != "project-profile" {
		t.Errorf("expected profile %q, got %q", "project-profile", cfg.AWS.Profile)
	}
}

func TestLoadFromGlobalConfig(t *testing.T) {
	// グローバル設定ディレクトリを一時ディレクトリに設定
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "bundr")
	err := os.MkdirAll(globalDir, 0755)
	if err != nil {
		t.Fatalf("failed to create global config dir: %v", err)
	}

	configContent := []byte(`[aws]
region = "ap-southeast-1"
`)
	err = os.WriteFile(filepath.Join(globalDir, "config.toml"), configContent, 0644)
	if err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	// 環境変数をクリーン
	t.Setenv("BUNDR_AWS_REGION", "")
	t.Setenv("BUNDR_AWS_PROFILE", "")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "")

	// プロジェクト設定なしのディレクトリ
	projectDir := t.TempDir()

	cfg, err := LoadWithGlobalDir(projectDir, globalDir)
	if err != nil {
		t.Fatalf("LoadWithGlobalDir() returned error: %v", err)
	}

	if cfg.AWS.Region != "ap-southeast-1" {
		t.Errorf("expected region %q, got %q", "ap-southeast-1", cfg.AWS.Region)
	}
}

func TestProjectConfigOverridesGlobalConfig(t *testing.T) {
	// グローバル設定
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global", "bundr")
	err := os.MkdirAll(globalDir, 0755)
	if err != nil {
		t.Fatalf("failed to create global config dir: %v", err)
	}
	err = os.WriteFile(filepath.Join(globalDir, "config.toml"), []byte(`[aws]
region = "us-east-1"
profile = "global-profile"
`), 0644)
	if err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	// プロジェクト設定
	projectDir := filepath.Join(tmpDir, "project")
	err = os.MkdirAll(projectDir, 0755)
	if err != nil {
		t.Fatalf("failed to create project dir: %v", err)
	}
	err = os.WriteFile(filepath.Join(projectDir, ".bundr.toml"), []byte(`[aws]
region = "ap-northeast-1"
`), 0644)
	if err != nil {
		t.Fatalf("failed to write project config: %v", err)
	}

	// 環境変数をクリーン
	t.Setenv("BUNDR_AWS_REGION", "")
	t.Setenv("BUNDR_AWS_PROFILE", "")
	t.Setenv("BUNDR_AWS_KMS_KEY_ID", "")

	cfg, err := LoadWithGlobalDir(projectDir, globalDir)
	if err != nil {
		t.Fatalf("LoadWithGlobalDir() returned error: %v", err)
	}

	// プロジェクト設定が優先
	if cfg.AWS.Region != "ap-northeast-1" {
		t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
	}
	// グローバル設定は profile のみ残る
	if cfg.AWS.Profile != "global-profile" {
		t.Errorf("expected profile %q, got %q", "global-profile", cfg.AWS.Profile)
	}
}
