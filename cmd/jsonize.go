package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	"github.com/youyo/bundr/internal/backend"
	"github.com/youyo/bundr/internal/jsonize"
)

// JsonizeCmd represents the "jsonize" subcommand.
type JsonizeCmd struct {
	Frompath  string  `required:"" predictor:"prefix" help:"Source prefix (e.g. ps:/app/prod/)"`
	To        *string `optional:"" predictor:"ref" name:"to" help:"Target ref to save JSON (omit to print to stdout)"`
	Store     *string `optional:"" enum:"raw,json" name:"store" help:"Storage mode for target (raw|json) [default: json]"`
	ValueType *string `optional:"" enum:"string,secure" name:"value-type" help:"Value type (string|secure) [default: string]"`
	Force     bool    `help:"Overwrite target if it already exists (save mode only)"`
	Compact   bool    `help:"Print compact JSON without indentation (stdout mode only)"`

	out io.Writer // for testing; nil means os.Stdout
}

// Run executes the jsonize command.
func (c *JsonizeCmd) Run(appCtx *Context) error {
	out := c.out
	if out == nil {
		out = os.Stdout
	}

	// 1. frompath パース + sm: チェック
	fromRef, err := backend.ParseRef(c.Frompath)
	if err != nil {
		return fmt.Errorf("jsonize command failed: invalid frompath ref: %w", err)
	}
	if fromRef.Type == backend.BackendTypeSM {
		return fmt.Errorf("jsonize command failed: --frompath sm: backend is not supported (use ps: or psa:)")
	}

	// 2. モード判定
	isStdoutMode := c.To == nil

	// 3. フラグ組み合わせバリデーション
	if err := c.validateFlags(isStdoutMode); err != nil {
		return err
	}

	// 4. [save モードのみ] target ref パース + 自己参照チェック
	var targetRef *backend.Ref
	if !isStdoutMode {
		ref, err := backend.ParseRef(*c.To)
		if err != nil {
			return fmt.Errorf("jsonize command failed: invalid target ref: %w", err)
		}
		targetRef = &ref

		fromBase := strings.TrimRight(fromRef.Path, "/") + "/"
		if strings.HasPrefix(targetRef.Path+"/", fromBase) || targetRef.Path == strings.TrimRight(fromRef.Path, "/") {
			return fmt.Errorf("jsonize command failed: target %q is within --frompath %q (self-reference not allowed)", *c.To, c.Frompath)
		}
	}

	// 5. frompath バックエンド作成
	fromBackend, err := appCtx.BackendFactory(fromRef.Type)
	if err != nil {
		return fmt.Errorf("jsonize command failed: create from backend: %w", err)
	}

	// 6. frompath からパラメータ一括取得
	entries, err := fromBackend.GetByPrefix(context.Background(), fromRef.Path, backend.GetByPrefixOptions{Recursive: true})
	if err != nil {
		return fmt.Errorf("jsonize command failed: get parameters: %w", err)
	}

	// 7. [save モードのみ, --force=false] target 存在チェック
	if !isStdoutMode && !c.Force {
		targetBackend, err := appCtx.BackendFactory(targetRef.Type)
		if err != nil {
			return fmt.Errorf("jsonize command failed: create target backend: %w", err)
		}
		_, err = targetBackend.Get(context.Background(), *c.To, backend.GetOptions{ForceRaw: true})
		if err == nil {
			return fmt.Errorf("jsonize command failed: target already exists: %s (use --force to overwrite)", *c.To)
		}
		if !isNotFound(err) {
			return fmt.Errorf("jsonize command failed: check target existence: %w", err)
		}
	}

	// 8. ParameterEntry → jsonize.Entry 変換
	jsonizeEntries := parameterEntriesToJsonizeEntries(entries, fromRef.Path)

	// Build は常にコンパクト JSON を返す。stdout モードではインデントを追加する。
	jsonBytes, err := jsonize.Build(jsonizeEntries, true)
	if err != nil {
		return fmt.Errorf("jsonize command failed: build json: %w", err)
	}

	// 9. stdout モード: インデント付き（または --compact）で out へ出力
	if isStdoutMode {
		var output []byte
		if c.Compact {
			output = jsonBytes
		} else {
			var v interface{}
			if err := json.Unmarshal(jsonBytes, &v); err != nil {
				return fmt.Errorf("jsonize command failed: unmarshal for indent: %w", err)
			}
			output, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				return fmt.Errorf("jsonize command failed: indent json: %w", err)
			}
		}
		fmt.Fprintln(out, string(output))
		return nil
	}

	// 10. save モード: target バックエンドへ Put
	store := "json"
	if c.Store != nil {
		store = *c.Store
	}
	valueType := "string"
	if c.ValueType != nil {
		valueType = *c.ValueType
	}

	targetBackend, err := appCtx.BackendFactory(targetRef.Type)
	if err != nil {
		return fmt.Errorf("jsonize command failed: create target backend: %w", err)
	}
	if err := targetBackend.Put(context.Background(), *c.To, backend.PutOptions{
		Value:     string(jsonBytes),
		StoreMode: store,
		ValueType: valueType,
	}); err != nil {
		return fmt.Errorf("jsonize command failed: put target: %w", err)
	}

	return nil
}

// validateFlags はモードに応じたフラグ組み合わせを検証する。
func (c *JsonizeCmd) validateFlags(isStdoutMode bool) error {
	if isStdoutMode {
		if c.Store != nil {
			return fmt.Errorf("jsonize command failed: --store is only valid with --to")
		}
		if c.ValueType != nil {
			return fmt.Errorf("jsonize command failed: --value-type is only valid with --to")
		}
		if c.Force {
			return fmt.Errorf("jsonize command failed: --force is only valid with --to")
		}
	} else {
		if c.Compact {
			return fmt.Errorf("jsonize command failed: --compact is only valid without --to")
		}
	}
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
