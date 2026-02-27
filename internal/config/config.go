package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config はアプリケーション全体の設定を保持する。
type Config struct {
	AWS AWSConfig `mapstructure:"aws"`
}

// AWSConfig は AWS 関連の設定を保持する。
type AWSConfig struct {
	Region   string `mapstructure:"region"`
	Profile  string `mapstructure:"profile"`
	KMSKeyID string `mapstructure:"kms_key_id"`
}

// Load はカレントディレクトリとデフォルトのグローバル設定を読み込む。
// 優先順位: env vars > .bundr.toml (カレントディレクトリ) > ~/.config/bundr/config.toml
func Load() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return LoadFromDir(cwd)
	}

	globalDir := filepath.Join(homeDir, ".config", "bundr")
	return LoadWithGlobalDir(cwd, globalDir)
}

// LoadFromDir は指定ディレクトリの .bundr.toml と環境変数から設定を読み込む。
func LoadFromDir(dir string) (*Config, error) {
	return LoadWithGlobalDir(dir, "")
}

// LoadWithGlobalDir はグローバル設定とプロジェクト設定を読み込む。
// 優先順位: env vars > プロジェクト設定 (.bundr.toml) > グローバル設定 (config.toml)
func LoadWithGlobalDir(projectDir, globalDir string) (*Config, error) {
	cfg := &Config{}

	// 1. グローバル設定 (最低優先)
	if globalDir != "" {
		_ = loadTOML(globalDir, "config", cfg)
	}

	// 2. プロジェクト設定 (グローバルより優先)
	_ = loadTOML(projectDir, ".bundr", cfg)

	// 3. 環境変数 (最高優先)
	applyEnvOverrides(cfg)

	return cfg, nil
}

// loadTOML は指定ディレクトリから TOML ファイルを読み込んで cfg にマージする。
// 既存の非空値はファイルの値で上書きされる。
func loadTOML(dir, name string, cfg *Config) error {
	v := viper.New()
	v.SetConfigName(name)
	v.SetConfigType("toml")
	v.AddConfigPath(dir)

	if err := v.ReadInConfig(); err != nil {
		return err
	}

	var fileCfg Config
	if err := v.Unmarshal(&fileCfg); err != nil {
		return err
	}

	// ファイルに設定されている値のみマージ
	if fileCfg.AWS.Region != "" {
		cfg.AWS.Region = fileCfg.AWS.Region
	}
	if fileCfg.AWS.Profile != "" {
		cfg.AWS.Profile = fileCfg.AWS.Profile
	}
	if fileCfg.AWS.KMSKeyID != "" {
		cfg.AWS.KMSKeyID = fileCfg.AWS.KMSKeyID
	}

	return nil
}

// ApplyCLIOverrides は CLI フラグの値で設定をオーバーライドする。
// 空文字の引数は無視される（既存設定を保持）。
func ApplyCLIOverrides(cfg *Config, region, profile, kmsKeyID string) {
	if region != "" {
		cfg.AWS.Region = region
	}
	if profile != "" {
		cfg.AWS.Profile = profile
	}
	if kmsKeyID != "" {
		cfg.AWS.KMSKeyID = kmsKeyID
	}
}

// applyEnvOverrides は環境変数の値で設定をオーバーライドする。
// 優先順位: BUNDR_* > AWS_*（標準 AWS 環境変数はフォールバック）
func applyEnvOverrides(cfg *Config) {
	// 1. 標準 AWS 環境変数（フォールバック）
	if v := os.Getenv("AWS_REGION"); v != "" {
		cfg.AWS.Region = v
	}
	if v := os.Getenv("AWS_PROFILE"); v != "" {
		cfg.AWS.Profile = v
	}
	// 2. BUNDR_* 環境変数（AWS_* より優先）
	if v := os.Getenv("BUNDR_AWS_REGION"); v != "" {
		cfg.AWS.Region = v
	}
	if v := os.Getenv("BUNDR_AWS_PROFILE"); v != "" {
		cfg.AWS.Profile = v
	}
	if v := os.Getenv("BUNDR_AWS_KMS_KEY_ID"); v != "" {
		cfg.AWS.KMSKeyID = v
	}
}
