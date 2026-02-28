---
title: jsonize バグ修正 — 複数 frompath と PutParameter タグ競合
project: bundr
author: planning-agent
created: 2026-03-01
status: Ready for Review
---

# jsonize バグ修正

## 概要

`bundr jsonize` コマンドに存在する 2 つのバグを修正する。

1. **バグ1**: 複数 `--frompath` を指定しても `{}` になる（末端パラメータの取得に失敗）
2. **バグ2**: `--to` フラグを使うと `PutParameter: tags and overwrite can't be used together` エラーが発生する

## 症状

```bash
# バグ1: 複数 frompath / 末端パラメータ → {}
$ bundr jsonize --frompath ps:/slacklens/preview/apiGateway.url \
               --frompath ps:/slacklens/preview/knowledgeBase.arn
{}

# バグ2: --to フラグ使用 → AWS API エラー
$ bundr jsonize --frompath ps:/slacklens/preview/apiGateway.url \
               --frompath ps:/slacklens/preview/knowledgeBase.arn \
               --to ps:/slacklens/preview/test
command failed: jsonize command failed: put target: ssm PutParameter:
  api error ValidationException: Invalid request: tags and overwrite can't be used together.
```

---

## バグ根本原因

### バグ1: `{}` になる問題

**原因 A — `Frompath string`（単一フィールド）**

```go
// cmd/jsonize.go:20
Frompath string `required:"" short:"f" name:"frompath" ...`
```

Kong は `[]string` でないと複数フラグを受け付けない。
`--frompath a --frompath b` を渡すと最後の値（`b`）のみが残る。

**原因 B — 末端パラメータが `GetByPrefix` で返らない**

`ps:/slacklens/preview/apiGateway.url` は末端パラメータ（プレフィックスではない）。
`GetByPrefix` は `normalizedPrefix = "/slacklens/preview/apiGateway.url/"` で
始まるパスを探すが、パラメータ自身は末尾スラッシュなしのため **マッチしない → 空リスト → `{}`**

### バグ2: tags and overwrite 競合

```go
// internal/backend/ps.go:73-79
input := &ssm.PutParameterInput{
    Overwrite: aws.Bool(true),  // 常に true
    Tags:      ssmTags,         // ← Overwrite=true と同時使用不可
}
```

AWS SSM API の制約: `Overwrite=true` + `Tags` の同時指定 → `ValidationException`

---

## 修正方針

### 修正1: `Frompath []string` への変更 + 末端パラメータ対応

**`cmd/jsonize.go`**

1. `Frompath string` → `Frompath []string`（Kong の `[]string` フラグは `sep` タグなしで複数フラグに対応。同リポジトリの `ExecCmd.From []string` と同じパターン）
2. バリデーション（自己参照チェック）を全 frompath に対してループ実行してから処理開始（バリデーションフェーズとして分離）
3. 各 frompath に対して `GetByPrefix` を実行し、エントリが空なら `Get` で単一パラメータとして取得
4. 複数 frompath の結果を統合して `jsonize.Build()` へ渡す

**末端パラメータのキー名と変換**

`ps:/slacklens/preview/apiGateway.url` → キー: `apiGateway.url`（`path.Base(ref.Path)` 相当）

`pathToParts` は `/` と `_` のみを区切り文字として扱う（`.` は区切り文字ではない）。
そのため `"apiGateway.url"` は小文字化して `"apigateway.url"` の単一キーになる。

```json
{"apigateway.url": "value"}
```

ネスト構造（`{"apigateway": {"url": "value"}}`）は **不要**。現行 `pathToParts` の仕様を踏襲する。

**末端パラメータの StoreMode 扱い**

`Backend.Get` は内部でタグを参照してデコード済みの値を返す。つまり `Get` の戻り値は
すでに「デコード後の文字列」であるため、`StoreMode: "raw"` で `jsonize.Entry` を作成する
（`Build` は raw モードで `autoConvert` による型変換のみ行う）。これは仕様として明記する。

**バックエンドの選択**

各 frompath ごとに `BackendFactory(fromRef.Type)` を呼ぶ（`sm:` は早期却下）。

**自己参照チェック（--to 使用時）**

全 frompath をループして、各 `fromRef.Path` が `targetRef.Path` の配下にないか確認。
エラーメッセージ: `"target %q overlaps with --frompath %q"`

**テストヘルパーのシグネチャ変更**

`setupJsonizeStdoutCmd(frompath string)` → `setupJsonizeStdoutCmd(frompaths ...string)` に変更。
既存の JS-01〜JS-05 系テストは引数を追加するだけで対応可能。

### 修正2: PutParameter から Tags を分離 → AddTagsToResource で設定

**`internal/backend/ps.go`**

設計判断: 「新規パラメータか既存パラメータか」を事前に判定せず、常に2ステップ方式で統一する。
理由: 既存パラメータの確認に `Get` が必要になり、余分な API 呼び出しが増える。
2ステップ方式（PutParameter 後に AddTagsToResource）の方がシンプルで一貫性がある。

```go
// Step 1: Tags なしで PutParameter (Overwrite=true)
input := &ssm.PutParameterInput{
    Name:      aws.String(parsed.Path),
    Value:     aws.String(value),
    Type:      paramType,
    Overwrite: aws.Bool(true),
    // Tags フィールドを削除
}
// ...Tier/KMSKey の設定...
if _, err = b.client.PutParameter(ctx, input); err != nil {
    return fmt.Errorf("ssm PutParameter: %w", err)
}

// Step 2: AddTagsToResource でタグを設定
if _, err = b.client.AddTagsToResource(ctx, &ssm.AddTagsToResourceInput{
    ResourceType: ssmtypes.ResourceTypeForTaggingParameter,
    ResourceId:   aws.String(parsed.Path),
    Tags:         ssmTags,
}); err != nil {
    // パラメータは作成されたがタグが設定されていない状態を明示
    return fmt.Errorf("ssm AddTagsToResource failed (parameter was saved but tags are missing; run 'bundr put' again to retry): %w", err)
}
```

`AddTagsToResource` は既に `SSMClient` インターフェースに含まれているため、インターフェース変更は不要。

---

## 変更対象ファイル

| ファイル | 変更内容 |
|---------|---------|
| `cmd/jsonize.go` | `Frompath []string` 変更（sep タグなし）、複数 frompath バリデーション・ループ、末端パラメータ `Get` フォールバック、`path` パッケージのインポート追加 |
| `cmd/jsonize_test.go` | 複数 frompath テスト追加、末端パラメータテスト追加、テストヘルパーのシグネチャ変更 |
| `internal/backend/ps.go` | `PutParameter` から `Tags` 削除、`AddTagsToResource` 追加 |
| `internal/backend/ps_test.go` | `mockSSMClient` に `AddTagsToResource` コール記録を追加し、`PSBackend.Put` の2ステップ呼び出しを検証 |

> **注意**: `MockBackend`（`mock.go`）は `Backend` インターフェースを実装するもの。
> `AddTagsToResource` は `SSMClient` インターフェースに属する別の概念であり、
> `mock.go` への追加は**不要**。`ps_test.go` 専用の `mockSSMClient` で検証する。

---

## テスト設計書

### 正常系テストケース（追加分）

| ID | 入力 | 期待出力 | 備考 |
|----|------|---------|------|
| JS-06-multi-frompath-prefix | `--frompath ps:/a/app/ --frompath ps:/a/db/` (各プレフィックス配下に `host`, `port` が存在) | `{"host": "...", "port": ...}` | 複数プレフィックス統合 |
| JS-07-leaf-param | `--frompath ps:/a/b/host`（末端パラメータ） | `{"host": "value"}` | `path.Base` でキー名を `"host"` とする |
| JS-08-leaf-dotname | `--frompath ps:/a/apiGateway.url` | `{"apigateway.url": "value"}` | `.` は分割しない（単一キー） |
| JS-09-multi-leaf | `--frompath ps:/a/url --frompath ps:/a/arn` | `{"url": "val1", "arn": "val2"}` | 複数末端パラメータ |
| JSave-08-put-addtags | `--to` で保存（`mockSSMClient` で検証） | `PutParameter` 呼び出し後に `AddTagsToResource` が呼ばれる | `internal/backend/ps_test.go` でのみ検証 |

### 異常系テストケース（追加分）

| ID | 入力 | 期待エラー | 備考 |
|----|------|----------|------|
| JSaveE-08-multi-self-ref | `--frompath ps:/a/ --frompath ps:/b/ --to ps:/a/config` | "overlaps" エラー | 複数 frompath での自己参照チェック |
| JS-10-leaf-not-found | 存在しない末端パラメータ | "key not found" エラー | `Get` が失敗した場合のエラー伝搬 |

### `ps_test.go` 追加テスト設計

`mockSSMClient` に以下を追加:
```go
type mockSSMClient struct {
    putParameterCalls     []*ssm.PutParameterInput
    addTagsToResourceCalls []*ssm.AddTagsToResourceInput
    // ...
}
```

テストケース:
- `TestPSBackend_Put_TwoStep`: `PutParameter` が呼ばれた後に `AddTagsToResource` が呼ばれることを確認
- `TestPSBackend_Put_AddTagsFail_Error`: `AddTagsToResource` 失敗時に詳細なエラーメッセージが返ることを確認

---

## 実装手順

### Step 1: `internal/backend/ps_test.go` を修正（Red 準備）

`mockSSMClient` に `AddTagsToResource` コール記録フィールドを追加。
`TestPSBackend_Put_TwoStep` テストを追加 → Red（現在は1ステップのため失敗）。

### Step 2: `ps.go` の Put メソッドを修正（バグ2修正・Green）

`PutParameter` から `Tags` を削除し、`AddTagsToResource` を後続で呼ぶ2ステップ実装に変更。
`TestPSBackend_Put_TwoStep` → Green。

### Step 3: `jsonize_test.go` に新テストを追加（Red 準備）

- `JS-06` 複数プレフィックステスト
- `JS-07` 末端パラメータテスト
- テストヘルパー `setupJsonizeStdoutCmd` を `...string` に変更

### Step 4: `jsonize.go` を修正（バグ1修正・Green）

`Frompath []string` に変更し、複数 frompath ループ・末端パラメータ `Get` フォールバックを実装。
`path` パッケージをインポートに追加。
全テスト → Green。

### Step 5: `go test ./...` で全テスト通過確認

---

## リスク評価

| リスク | 重大度 | 対策 |
|--------|--------|------|
| `AddTagsToResource` 失敗時のタグ欠落（Silent corruption） | **中** | エラーメッセージで「パラメータは作成されたがタグなし、再実行で解消可能」と明示。`bundr get` はタグなしパラメータを `raw` として扱うため、再実行するまで誤動作する可能性あり |
| Kong の `[]string` フラグ挙動 | 低 | `ExecCmd.From []string` と同じ `sep` なしパターンを使用。既存コードで動作確認済み |
| 末端パラメータの `Get` 失敗 | 低 | エラーを伝搬してユーザーに通知する |
| 複数 frompath の結果マージで競合 | 低 | `jsonize.Build()` の競合検出ロジックが既に実装済み → エラーを返す |
| `path.Base` の import 漏れ | 低 | `path.Base` は `path` パッケージ（`strings.Split` のみでも代替可能） |

---

## シーケンス図

```mermaid
sequenceDiagram
    participant U as User
    participant J as jsonize.Run()
    participant B as Backend
    participant A as AWS SSM

    U->>J: --frompath ps:/a/url --frompath ps:/b/arn

    note over J: バリデーションフェーズ（--to 使用時）
    note over J: 各 frompath が target の配下に入らないか確認

    loop 各 frompath
        J->>B: GetByPrefix(fromRef.Path)
        B->>A: GetParametersByPath
        A-->>B: entries

        alt エントリが空（末端パラメータ）
            J->>B: Get(fp)
            B->>A: GetParameter
            A-->>B: value (デコード済み)
            B-->>J: value
            J->>J: Entry{Path: path.Base(ref.Path), Value: val, StoreMode: "raw"}
        else エントリあり（プレフィックス）
            J->>J: parameterEntriesToJsonizeEntries(entries)
        end
    end

    J->>J: jsonize.Build(allEntries)

    alt stdout モード（--to なし）
        J->>U: JSON 出力 (インデント or --compact)
    else save モード（--to あり）
        J->>B: Put(targetRef, PutOptions{...})
        B->>A: PutParameter (Tags なし, Overwrite=true)
        alt PutParameter 失敗
            A-->>B: エラー
            B-->>J: "ssm PutParameter: ..."
            J-->>U: エラー終了
        else PutParameter 成功
            A-->>B: OK
            B->>A: AddTagsToResource (マネージドタグを設定)
            alt AddTagsToResource 失敗
                A-->>B: エラー
                B-->>J: "ssm AddTagsToResource failed (parameter was saved but tags are missing; ...)"
                J-->>U: エラー終了（パラメータは存在するがタグなし）
            else AddTagsToResource 成功
                A-->>B: OK
                B-->>J: nil
                J-->>U: 正常終了
            end
        end
    end
```

---

## チェックリスト

- [x] 観点1: 実装実現可能性（手順の抜け漏れなし、インポート追加・ヘルパー変更を明記）
- [x] 観点2: TDD テスト設計（`ps_test.go` を対象に追加、Red→Green 順序明記、MockSSMClient と MockBackend の役割を分離）
- [x] 観点3: アーキテクチャ整合性（`ExecCmd.From` と同じ `[]string` パターン、Backend インターフェース変更なし）
- [x] 観点4: リスク評価（タグ欠落リスクを「中」に格上げ、エラーメッセージ詳細化）
- [x] 観点5: シーケンス図（エラーフロー・末端パラメータ分岐・AddTagsToResource 失敗ケースを図示）
