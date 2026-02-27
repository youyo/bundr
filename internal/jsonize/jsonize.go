package jsonize

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Entry はパス→値の単一エントリを表す。
type Entry struct {
	// Path は frompath プレフィックスを除去したサブパス。
	Path string
	// Value は raw 文字列値 (StoreMode が json の場合は JSON エンコード済み文字列)。
	Value string
	// StoreMode は "raw" または "json"。
	StoreMode string
}

// Build は entries を受け取り、ネスト JSON オブジェクトを構築して JSON バイト列を返す。
// パス区切りルール:
//   - まず "/" でパスを分割してネスト階層を構築
//   - さらに "_" で各セグメントを分割してネストを深める
//   - 同一キーへの競合（既存の map を string で上書きなど）はエラー
//
// 型変換ルール:
//   - StoreMode="json" の entry は JSON としてデコードしてからネストに組み込む
//   - StoreMode="raw" の entry の値は autoConvert で型変換を試みる
//
// autoConvert=true の場合:
//   - "true"/"false" → bool
//   - 整数文字列 → float64 (JSON の数値型)
//   - 小数文字列 → float64
//   - "null" → nil
//   - その他 → string のまま
func Build(entries []Entry, autoConvert bool) ([]byte, error) {
	root := map[string]interface{}{}

	for _, entry := range entries {
		parts := pathToParts(entry.Path)

		var value interface{}
		if entry.StoreMode == "json" {
			var jsonVal interface{}
			if err := json.Unmarshal([]byte(entry.Value), &jsonVal); err != nil {
				return nil, fmt.Errorf("invalid json value for path %q: %w", entry.Path, err)
			}
			value = jsonVal
		} else if autoConvert {
			value = autoConvertValue(entry.Value)
		} else {
			value = entry.Value
		}

		if err := setNested(root, parts, value); err != nil {
			return nil, err
		}
	}

	return json.Marshal(root)
}

// pathToParts は "/" と "_" でパスを分割して小文字化した parts スライスを返す。
// 例: "DB_HOST" → ["db", "host"]
// 例: "nested/DB_HOST" → ["nested", "db", "host"]
func pathToParts(path string) []string {
	segments := strings.Split(path, "/")
	var parts []string
	for _, seg := range segments {
		subParts := strings.Split(seg, "_")
		for _, p := range subParts {
			parts = append(parts, strings.ToLower(p))
		}
	}
	return parts
}

// autoConvertValue は raw 文字列を Go の型に変換する。
func autoConvertValue(raw string) interface{} {
	switch raw {
	case "null":
		return nil
	case "true":
		return true
	case "false":
		return false
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		// NaN/Inf は json.Marshal できないので文字列のまま返す
		if math.IsNaN(f) || math.IsInf(f, 0) {
			return raw
		}
		return f
	}
	return raw
}

// setNested は root マップの parts 階層に value を設定する。
// 競合がある場合はエラーを返す。
func setNested(root map[string]interface{}, parts []string, value interface{}) error {
	return setNestedWithPath(root, parts, value, nil)
}

// setNestedWithPath は再帰処理でフルパスを追跡しながらネストを構築する。
func setNestedWithPath(root map[string]interface{}, parts []string, value interface{}, currentPath []string) error {
	if len(parts) == 0 {
		return nil
	}

	key := parts[0]
	fullPath := append(append([]string{}, currentPath...), key)

	if len(parts) == 1 {
		if _, exists := root[key]; exists {
			return fmt.Errorf("conflict at key %q: key already set", strings.Join(fullPath, "."))
		}
		root[key] = value
		return nil
	}

	rest := parts[1:]
	child, exists := root[key]
	if !exists {
		child = map[string]interface{}{}
		root[key] = child
	}

	childMap, ok := child.(map[string]interface{})
	if !ok {
		// 既存が scalar なのに子を設定しようとしている → conflict
		return fmt.Errorf("conflict at key %q: cannot set child on a non-object", strings.Join(fullPath, "."))
	}

	return setNestedWithPath(childMap, rest, value, fullPath)
}
