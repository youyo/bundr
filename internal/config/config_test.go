package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// 環境変数をクリーンにする
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_PROFILE", "")
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
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_PROFILE", "")
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
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_PROFILE", "")
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
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_PROFILE", "")
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
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_PROFILE", "")
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

func TestApplyCLIOverrides(t *testing.T) {
	t.Run("cfg-001: CLIフラグで空のベース設定を上書き", func(t *testing.T) {
		cfg := &Config{}
		ApplyCLIOverrides(cfg, "ap-northeast-1", "", "")
		if cfg.AWS.Region != "ap-northeast-1" {
			t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
		}
	})

	t.Run("cfg-002: CLIフラグがファイル設定より優先", func(t *testing.T) {
		cfg := &Config{AWS: AWSConfig{Region: "us-east-1"}}
		ApplyCLIOverrides(cfg, "ap-northeast-1", "", "")
		if cfg.AWS.Region != "ap-northeast-1" {
			t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
		}
	})

	t.Run("cfg-003: CLIフラグが空の場合はベース設定を保持", func(t *testing.T) {
		cfg := &Config{AWS: AWSConfig{Region: "us-east-1"}}
		ApplyCLIOverrides(cfg, "", "", "")
		if cfg.AWS.Region != "us-east-1" {
			t.Errorf("expected region %q, got %q", "us-east-1", cfg.AWS.Region)
		}
	})

	t.Run("cfg-004: profileの上書き", func(t *testing.T) {
		cfg := &Config{}
		ApplyCLIOverrides(cfg, "", "my-profile", "")
		if cfg.AWS.Profile != "my-profile" {
			t.Errorf("expected profile %q, got %q", "my-profile", cfg.AWS.Profile)
		}
	})

	t.Run("cfg-005: kmsKeyIDの上書き", func(t *testing.T) {
		cfg := &Config{}
		ApplyCLIOverrides(cfg, "", "", "arn:aws:kms:ap-northeast-1:123456789012:key/test")
		if cfg.AWS.KMSKeyID != "arn:aws:kms:ap-northeast-1:123456789012:key/test" {
			t.Errorf("expected kmsKeyID %q, got %q", "arn:aws:kms:ap-northeast-1:123456789012:key/test", cfg.AWS.KMSKeyID)
		}
	})

	t.Run("cfg-006: 全フラグ同時指定", func(t *testing.T) {
		cfg := &Config{}
		ApplyCLIOverrides(cfg, "ap-northeast-1", "prod", "key-1")
		if cfg.AWS.Region != "ap-northeast-1" {
			t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
		}
		if cfg.AWS.Profile != "prod" {
			t.Errorf("expected profile %q, got %q", "prod", cfg.AWS.Profile)
		}
		if cfg.AWS.KMSKeyID != "key-1" {
			t.Errorf("expected kmsKeyID %q, got %q", "key-1", cfg.AWS.KMSKeyID)
		}
	})

	t.Run("cfg-007: 全フラグが空文字のとき変更なし", func(t *testing.T) {
		cfg := &Config{AWS: AWSConfig{Region: "us-east-1", Profile: "existing", KMSKeyID: "existing-key"}}
		ApplyCLIOverrides(cfg, "", "", "")
		if cfg.AWS.Region != "us-east-1" {
			t.Errorf("expected region %q, got %q", "us-east-1", cfg.AWS.Region)
		}
		if cfg.AWS.Profile != "existing" {
			t.Errorf("expected profile %q, got %q", "existing", cfg.AWS.Profile)
		}
		if cfg.AWS.KMSKeyID != "existing-key" {
			t.Errorf("expected kmsKeyID %q, got %q", "existing-key", cfg.AWS.KMSKeyID)
		}
	})

	t.Run("cfg-008: regionのみ指定、他は空", func(t *testing.T) {
		cfg := &Config{AWS: AWSConfig{Profile: "existing-profile", KMSKeyID: "existing-key"}}
		ApplyCLIOverrides(cfg, "ap-northeast-1", "", "")
		if cfg.AWS.Region != "ap-northeast-1" {
			t.Errorf("expected region %q, got %q", "ap-northeast-1", cfg.AWS.Region)
		}
		if cfg.AWS.Profile != "existing-profile" {
			t.Errorf("expected profile %q, got %q", "existing-profile", cfg.AWS.Profile)
		}
		if cfg.AWS.KMSKeyID != "existing-key" {
			t.Errorf("expected kmsKeyID %q, got %q", "existing-key", cfg.AWS.KMSKeyID)
		}
	})
}

func TestAWSStandardEnvVars(t *testing.T) {
	allRelatedEnvVars := []string{
		"AWS_REGION", "AWS_PROFILE",
		"BUNDR_AWS_REGION", "BUNDR_AWS_PROFILE", "BUNDR_AWS_KMS_KEY_ID",
	}

	tests := []struct {
		name        string
		setEnv      map[string]string
		wantRegion  string
		wantProfile string
	}{
		{
			name:       "cfg-aws-01: AWS_REGION のみ",
			setEnv:     map[string]string{"AWS_REGION": "us-west-2"},
			wantRegion: "us-west-2",
		},
		{
			name:        "cfg-aws-02: AWS_PROFILE のみ",
			setEnv:      map[string]string{"AWS_PROFILE": "myprofile"},
			wantProfile: "myprofile",
		},
		{
			name: "cfg-aws-03: BUNDR_AWS_REGION が AWS_REGION より優先",
			setEnv: map[string]string{
				"AWS_REGION":       "us-east-1",
				"BUNDR_AWS_REGION": "ap-northeast-1",
			},
			wantRegion: "ap-northeast-1",
		},
		{
			name: "cfg-aws-04: BUNDR_AWS_PROFILE が AWS_PROFILE より優先",
			setEnv: map[string]string{
				"AWS_PROFILE":       "aws-profile",
				"BUNDR_AWS_PROFILE": "bundr-profile",
			},
			wantProfile: "bundr-profile",
		},
		{
			name:       "cfg-aws-05: BUNDR_AWS_REGION のみ（後方互換）",
			setEnv:     map[string]string{"BUNDR_AWS_REGION": "eu-west-1"},
			wantRegion: "eu-west-1",
		},
		{
			name: "cfg-aws-07: AWS_REGION が空文字 → BUNDR_AWS_REGION を使用",
			setEnv: map[string]string{
				"AWS_REGION":       "",
				"BUNDR_AWS_REGION": "ap-northeast-1",
			},
			wantRegion: "ap-northeast-1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, k := range allRelatedEnvVars {
				t.Setenv(k, "")
			}
			for k, v := range tc.setEnv {
				t.Setenv(k, v)
			}
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			if tc.wantRegion != "" && cfg.AWS.Region != tc.wantRegion {
				t.Errorf("Region: want %q, got %q", tc.wantRegion, cfg.AWS.Region)
			}
			if tc.wantProfile != "" && cfg.AWS.Profile != tc.wantProfile {
				t.Errorf("Profile: want %q, got %q", tc.wantProfile, cfg.AWS.Profile)
			}
		})
	}
}
