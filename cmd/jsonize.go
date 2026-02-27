package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/jsonize"
)

// JsonizeCmd represents the "jsonize" subcommand.
type JsonizeCmd struct {
	Target    string `arg:"" help:"Target ref to store the JSON (e.g. ps:/app/config, sm:app-config)"`
	Frompath  string `required:"" help:"Source prefix (e.g. ps:/app/prod/)"`
	Store     string `default:"json" enum:"raw,json" help:"Storage mode for target (raw|json)"`
	ValueType string `default:"string" enum:"string,secure" help:"Value type (string|secure)"`
	Force     bool   `help:"Overwrite target if it already exists"`
}

// Run executes the jsonize command.
func (c *JsonizeCmd) Run(appCtx *Context) error {
	// 1. frompath の ref をパース
	fromRef, err := backend.ParseRef(c.Frompath)
	if err != nil {
		return fmt.Errorf("jsonize command failed: invalid frompath ref: %w", err)
	}

	// 2. frompath に sm: は不可
	if fromRef.Type == backend.BackendTypeSM {
		return fmt.Errorf("jsonize command failed: --frompath sm: backend is not supported (use ps: or psa:)")
	}

	// 3. target の ref をパース
	targetRef, err := backend.ParseRef(c.Target)
	if err != nil {
		return fmt.Errorf("jsonize command failed: invalid target ref: %w", err)
	}

	// 4. frompath のバックエンドを作成
	fromBackend, err := appCtx.BackendFactory(fromRef.Type)
	if err != nil {
		return fmt.Errorf("jsonize command failed: create from backend: %w", err)
	}

	// 5. frompath からパラメータ一括取得
	entries, err := fromBackend.GetByPrefix(context.Background(), fromRef.Path, backend.GetByPrefixOptions{Recursive: true})
	if err != nil {
		return fmt.Errorf("jsonize command failed: get parameters: %w", err)
	}

	// 6. --force=false の場合: target の存在チェック
	if !c.Force {
		targetBackend, err := appCtx.BackendFactory(targetRef.Type)
		if err != nil {
			return fmt.Errorf("jsonize command failed: create target backend: %w", err)
		}
		_, err = targetBackend.Get(context.Background(), c.Target, backend.GetOptions{ForceRaw: true})
		if err == nil {
			// err == nil → target が存在する → 上書き禁止
			return fmt.Errorf("jsonize command failed: target already exists: %s (use --force to overwrite)", c.Target)
		}
		if !isNotFound(err) {
			// ParameterNotFound 以外のエラーは即時失敗（フェイルセーフ）
			return fmt.Errorf("jsonize command failed: check target existence: %w", err)
		}
		// isNotFound == true → target が存在しない → 続行
	}

	// 7. ParameterEntry → jsonize.Entry 変換
	jsonizeEntries := parameterEntriesToJsonizeEntries(entries, fromRef.Path)

	// 8. JSON ビルド
	jsonBytes, err := jsonize.Build(jsonizeEntries, true)
	if err != nil {
		return fmt.Errorf("jsonize command failed: build json: %w", err)
	}

	// 9. target バックエンドを作成して Put
	targetBackend, err := appCtx.BackendFactory(targetRef.Type)
	if err != nil {
		return fmt.Errorf("jsonize command failed: create target backend: %w", err)
	}

	if err := targetBackend.Put(context.Background(), c.Target, backend.PutOptions{
		Value:     string(jsonBytes),
		StoreMode: c.Store,
		ValueType: c.ValueType,
	}); err != nil {
		return fmt.Errorf("jsonize command failed: put target: %w", err)
	}

	fmt.Println("OK")
	return nil
}

// parameterEntriesToJsonizeEntries は ParameterEntry スライスを jsonize.Entry スライスに変換する。
func parameterEntriesToJsonizeEntries(entries []backend.ParameterEntry, fromPath string) []jsonize.Entry {
	base := strings.TrimRight(fromPath, "/") + "/"
	result := make([]jsonize.Entry, 0, len(entries))
	for _, e := range entries {
		relPath := strings.TrimPrefix(e.Path, base)
		// relPath が空文字の場合（パラメータがプレフィックス自身と一致）はスキップ
		if relPath == "" || relPath == e.Path {
			continue
		}
		result = append(result, jsonize.Entry{
			Path:      relPath,
			Value:     e.Value,
			StoreMode: e.StoreMode,
		})
	}
	return result
}

// isNotFound は "存在しない" エラーかどうかを判定する。
func isNotFound(err error) bool {
	var pnf *ssmtypes.ParameterNotFound
	var rnf *smtypes.ResourceNotFoundException
	if errors.As(err, &pnf) || errors.As(err, &rnf) {
		return true
	}
	// MockBackend のフォールバック
	return strings.Contains(err.Error(), "key not found")
}
