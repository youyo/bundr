---
title: README sm: バックエンドサポート追記
project: bundr
author: planning-agent
created: 2026-03-02
status: Draft
---

# README sm: バックエンドサポート追記

## Context

v0.4.4 リリース（コミット `cb755e5`）にて `bundr ls sm:` / `bundr cache refresh sm:` および sm: 補完サポートが実装された。しかし、README.md（英語版）と README.ja.md（日本語版）にはまだ ps: の例しか記載されておらず、sm: ユーザーが機能を発見できない状態。本タスクでこのギャップを解消する。

## スコープ

### 実装範囲
- `README.md`（英語版）— ls / cache セクションに sm: 例を追加
- `README.ja.md`（日本語版）— 同様に sm: 例を追加

### スコープ外
- コード変更なし
- Ref syntax テーブル変更なし（sm: はすでに記載あり）
- Command reference の仕様表変更なし

## 変更対象ファイル

| ファイル | 変更箇所 |
|--------|---------|
| `README.md` | Recipes > ls セクション（lines 130-148）に sm: 例 2-3 行追加 |
| `README.md` | Recipes > cache セクション（lines 260-268）に sm: 例 1-2 行追加 |
| `README.ja.md` | コマンドリファレンス > bundr ls セクション（lines 135-146）に sm: 例追加 |
| `README.ja.md` | コマンドリファレンス > bundr cache セクション（lines 217-223）に sm: 例追加 |

## 具体的な変更内容

### README.md — ls セクション

**現在:**
```markdown
### ls

List all parameter paths under a prefix:
bundr ls ps:/app/

List only the immediate children (no recursion):
bundr ls ps:/app/ --no-recursive

Count parameters:
bundr ls ps:/app/ | wc -l
```

**変更後（sm: 例を追加）:**
```markdown
### ls

List all parameter paths under a prefix:
```sh
bundr ls ps:/app/
bundr ls sm:myapp/          # Secrets Manager prefix
bundr ls sm:                 # all secrets
```

List only the immediate children (no recursion):
```sh
bundr ls ps:/app/ --no-recursive
bundr ls sm:myapp/ --no-recursive
```

Count parameters:
```sh
bundr ls ps:/app/ | wc -l
```
```

### README.md — cache セクション

**現在:**
```markdown
Refresh the cache manually after adding new parameters:

bundr cache refresh
```

**変更後（sm: 例と prefix 引数を追加）:**
```markdown
Refresh the cache manually after adding new parameters:

```sh
bundr cache refresh                  # refresh all backends
bundr cache refresh ps:/app/         # refresh a specific Parameter Store prefix
bundr cache refresh sm:              # refresh all Secrets Manager secrets
```
```

### README.ja.md — ls セクション

**現在:**
```markdown
bundr ls ps:/app/
bundr ls ps:/app/ --no-recursive  # サブディレクトリを展開しない
```

**変更後:**
```markdown
bundr ls ps:/app/
bundr ls sm:myapp/             # Secrets Manager のプレフィックス
bundr ls sm:                   # Secrets Manager の全シークレット
bundr ls ps:/app/ --no-recursive  # サブディレクトリを展開しない
```

### README.ja.md — cache セクション

**現在:**
```markdown
bundr cache refresh
```

**変更後:**
```markdown
bundr cache refresh                 # 全バックエンドを更新
bundr cache refresh ps:/app/        # 特定の Parameter Store プレフィックスを更新
bundr cache refresh sm:             # Secrets Manager の全シークレットを更新
```

## チェックリスト（27項目セルフレビュー）

### 観点1: 実装実現可能性（5項目）
- [x] 手順の抜け漏れがないか — 4ファイル全て対象
- [x] 各ステップが具体的か — 変更箇所を行番号付きで明示
- [x] 依存関係が明示されているか — ドキュメント変更のみ、依存なし
- [x] 変更対象ファイルが網羅されているか — README.md + README.ja.md
- [x] 影響範囲が正確か — コード変更なし、副作用なし

### 観点2: TDDテスト設計（N/A）
- ドキュメント変更のみ。テストは不要（N/A）

### 観点3: アーキテクチャ整合性（5項目）
- [x] 既存の命名規則に従っているか — 既存例と同一フォーマット使用
- [x] 設計パターンが一貫しているか — コードブロック形式を踏襲
- [x] モジュール分割が適切か — ドキュメント変更のみ
- [x] 依存方向が正しいか — 影響なし
- [x] 類似機能との統一性があるか — 既存の ps: 例と同じスタイル

### 観点4: リスク評価（6項目）
- [x] リスクが特定されているか — 内容の誤りリスクのみ
- [x] 対策が具体的か — 実装コードと照合して内容を確認
- [x] フェイルセーフ — ドキュメント変更のみ（コード実行なし）
- [x] パフォーマンス影響 — なし
- [x] セキュリティ観点 — なし
- [x] ロールバック — git revert で簡単に戻せる

### 観点5: シーケンス図（N/A）
- ドキュメント変更のみ。図は不要（N/A）

## 検証方法

変更後に以下を確認:
1. `cat README.md | grep -A 10 "### ls"` — sm: 例が含まれること
2. `cat README.md | grep -A 10 "### cache"` — sm: 例が含まれること
3. `cat README.ja.md | grep "sm:"` — 日本語版にも sm: 例があること
4. レンダリング確認: `gh browse README.md`（任意）または VSCode プレビュー
