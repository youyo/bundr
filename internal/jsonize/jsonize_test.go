package jsonize_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/youyo/bundr/internal/jsonize"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		id          string
		entries     []jsonize.Entry
		autoConvert bool
		wantJSON    string // JSON 比較は map 経由（キー順不定のため）
		wantErr     string // 空文字列ならエラーなし
	}{
		// 正常系 J-01 〜 J-17
		{
			id: "J-01",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "localhost", StoreMode: "raw"},
				{Path: "DB_PORT", Value: "5432", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"host":"localhost","port":5432}}`,
		},
		{
			id: "J-02",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "localhost", StoreMode: "raw"},
				{Path: "DB_PORT", Value: "5432", StoreMode: "raw"},
			},
			autoConvert: false,
			wantJSON:    `{"db":{"host":"localhost","port":"5432"}}`,
		},
		{
			id: "J-03",
			entries: []jsonize.Entry{
				{Path: "APP_NAME", Value: "myapp", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"app":{"name":"myapp"}}`,
		},
		{
			id: "J-04",
			entries: []jsonize.Entry{
				{Path: "KEY", Value: "value", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"key":"value"}`,
		},
		{
			id: "J-05",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "localhost", StoreMode: "raw"},
				{Path: "APP_NAME", Value: "myapp", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"host":"localhost"},"app":{"name":"myapp"}}`,
		},
		{
			id: "J-06",
			entries: []jsonize.Entry{
				{Path: "nested/DB_HOST", Value: "localhost", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"nested":{"db":{"host":"localhost"}}}`,
		},
		{
			id: "J-07",
			entries: []jsonize.Entry{
				{Path: "CONFIG", Value: `{"timeout":30,"enabled":true}`, StoreMode: "json"},
			},
			autoConvert: true,
			wantJSON:    `{"config":{"timeout":30,"enabled":true}}`,
		},
		{
			id: "J-08",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "localhost", StoreMode: "raw"},
				{Path: "CONFIG", Value: `{"db":{"port":5432}}`, StoreMode: "json"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"host":"localhost"},"config":{"db":{"port":5432}}}`,
		},
		{
			id: "J-09",
			entries: []jsonize.Entry{
				{Path: "DB_ENABLED", Value: "true", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"enabled":true}}`,
		},
		{
			id: "J-10",
			entries: []jsonize.Entry{
				{Path: "DB_ENABLED", Value: "false", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"enabled":false}}`,
		},
		{
			id: "J-11",
			entries: []jsonize.Entry{
				{Path: "VALUE", Value: "null", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"value":null}`,
		},
		{
			id: "J-12",
			entries: []jsonize.Entry{
				{Path: "PRICE", Value: "42.5", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"price":42.5}`,
		},
		{
			id: "J-13",
			entries: []jsonize.Entry{
				{Path: "COUNT", Value: "100", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"count":100}`,
		},
		{
			id: "J-14",
			entries: []jsonize.Entry{
				{Path: "TAGS", Value: `["a","b"]`, StoreMode: "json"},
			},
			autoConvert: true,
			wantJSON:    `{"tags":["a","b"]}`,
		},
		{
			id:          "J-15",
			entries:     []jsonize.Entry{},
			autoConvert: true,
			wantJSON:    `{}`,
		},
		{
			id: "J-16",
			entries: []jsonize.Entry{
				{Path: "KEY_A_B_C", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"key":{"a":{"b":{"c":"val"}}}}`,
		},
		{
			id: "J-17",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "localhost", StoreMode: "raw"},
				{Path: "DB_PORT", Value: "5432", StoreMode: "raw"},
				{Path: "DB_NAME", Value: "mydb", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"host":"localhost","port":5432,"name":"mydb"}}`,
		},
		// 異常系 JE-01 〜 JE-04
		{
			id: "JE-01",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "x", StoreMode: "raw"},
				{Path: "DB_HOST_SUB", Value: "y", StoreMode: "raw"},
			},
			autoConvert: true,
			wantErr:     `conflict at key "db.host"`,
		},
		{
			id: "JE-02",
			entries: []jsonize.Entry{
				{Path: "A", Value: "x", StoreMode: "raw"},
				{Path: "A_B", Value: "y", StoreMode: "raw"},
			},
			autoConvert: true,
			wantErr:     `conflict at key "a"`,
		},
		{
			id: "JE-03",
			entries: []jsonize.Entry{
				{Path: "CONFIG", Value: `{invalid json}`, StoreMode: "json"},
			},
			autoConvert: true,
			wantErr:     `invalid json value for path "CONFIG"`,
		},
		{
			id: "JE-04",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "x", StoreMode: "json"},
			},
			autoConvert: true,
			wantErr:     `invalid json value for path "DB_HOST"`,
		},
		// エッジケース JEC-01 〜 JEC-10
		{
			id: "JEC-01",
			entries: []jsonize.Entry{
				{Path: "HOST", Value: "localhost", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"host":"localhost"}`,
		},
		{
			// 連続アンダースコア: strings.Split の自然な動作に従い空文字列セグメントが入る
			id: "JEC-02",
			entries: []jsonize.Entry{
				{Path: "A__B", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"a":{"":{"b":"val"}}}`,
		},
		{
			// 先頭アンダースコア: 先頭に空文字列セグメントが入る
			id: "JEC-03",
			entries: []jsonize.Entry{
				{Path: "_KEY", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"":{"key":"val"}}`,
		},
		{
			id: "JEC-04",
			entries: []jsonize.Entry{
				{Path: "PORT", Value: "8080", StoreMode: "raw"},
			},
			autoConvert: false,
			wantJSON:    `{"port":"8080"}`,
		},
		{
			id: "JEC-05",
			entries: []jsonize.Entry{
				{Path: "DB_HOST", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db":{"host":"val"}}`,
		},
		{
			id: "JEC-06",
			entries: []jsonize.Entry{
				{Path: "A_B_C_D_E_F_G_H_I_J", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"a":{"b":{"c":{"d":{"e":{"f":{"g":{"h":{"i":{"j":"val"}}}}}}}}}}`,
		},
		{
			id: "JEC-07",
			entries: []jsonize.Entry{
				{Path: "KEY", Value: `"hello"`, StoreMode: "json"},
			},
			autoConvert: true,
			wantJSON:    `{"key":"hello"}`,
		},
		{
			id: "JEC-08",
			entries: []jsonize.Entry{
				{Path: "KEY", Value: `42`, StoreMode: "json"},
			},
			autoConvert: true,
			wantJSON:    `{"key":42}`,
		},
		{
			// スラッシュのみ区切り（アンダースコアなし）
			id: "JEC-09",
			entries: []jsonize.Entry{
				{Path: "app/config", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"app":{"config":"val"}}`,
		},
		{
			// ハイフンを含むキー: ハイフンは区切り文字ではない
			id: "JEC-10",
			entries: []jsonize.Entry{
				{Path: "DB-HOST", Value: "val", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"db-host":"val"}`,
		},
		// NaN/Inf テスト
		{
			id: "J-18",
			entries: []jsonize.Entry{
				{Path: "VALUE", Value: "NaN", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"value":"NaN"}`,
		},
		{
			id: "J-19",
			entries: []jsonize.Entry{
				{Path: "VALUE", Value: "Inf", StoreMode: "raw"},
			},
			autoConvert: true,
			wantJSON:    `{"value":"Inf"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.id, func(t *testing.T) {
			got, err := jsonize.Build(tc.entries, tc.autoConvert)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			// JSON 比較は map[string]interface{} 経由で行う（キー順不定のため）
			var gotMap, wantMap interface{}
			if err := json.Unmarshal(got, &gotMap); err != nil {
				t.Fatalf("unmarshal got: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.wantJSON), &wantMap); err != nil {
				t.Fatalf("unmarshal want: %v", err)
			}
			if !reflect.DeepEqual(gotMap, wantMap) {
				t.Errorf("got %s, want %s", got, tc.wantJSON)
			}
		})
	}
}
