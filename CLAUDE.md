# CLAUDE.md

このファイルは、Claude Code が本リポジトリ（knit）で開発・設計を行う際の動作指針とルールを定義します。

## 1. プロジェクト概要

knit は、アーカイブされた google/wire の軽量なドロップイン代替となる Go 製の DI コードジェネレータです。

- `knit gen`: `wire.Build`、`wire.NewSet`、`wire.Bind`、通常の `NewXxx` 関数とクリーンアップ関数に対応した `wire_gen.go` を生成し、依存グラフ `.knit/di-graph.json` も出力します。
- `knit version`: バージョンを表示します。

## 2. 必須参照ドキュメント

実装・修正を行う前に、必ず `docs/` 配下の設計ドキュメントを確認すること。

| ドキュメント | 内容 |
|---|---|
| `docs/01_system_overview.md` | コア機能、サポート対象・対象外のスコープ、CLI 仕様 |
| `docs/02_data_models.md` | ProviderNode、InjectorSpec、TypeKey、di-graph.json の構造定義 |
| `docs/03_architecture.md` | パッケージ構成、コード生成パイプラインと生成コードの詳細 |
| `docs/04_guidelines.md` | コーディング規約、決定論的出力、テスト戦略、禁止事項、テストマトリクス |

## 3. ビルド・テストコマンド

開発操作はすべて Makefile 経由で実行する（`make help` で一覧表示）。ツール類（golangci-lint、govulncheck）はバージョン固定で `./bin` に自動インストールされる。

```sh
make test       # ユニットテスト（go test -short ./...）
make test-all   # E2E を含む全テスト（go test ./...）
make fmt        # フォーマット（gofumpt + goimports）
make lint       # golangci-lint（.golangci.yml）
make check      # CI 相当の一括チェック（tidy・フォーマット・リント・ユニットテスト）
make vuln       # 脆弱性スキャン
make cover      # カバレッジ
make build      # bin/knit のビルド
```

## 4. 開発・実装ルール

- **スコープ厳守**: `wire.Struct` や `wire.Value` などの対象外機能は実装しないこと。
- **依存の制限**: 外部依存は `golang.org/x/tools/go/packages` のみ。
- **検証の徹底**: コードを変更したら必ず `make check` を実行し、フォーマット・リント・ユニットテストがすべてパスすることを確認してから完了報告を行うこと。生成コードに影響する変更では `make test-all` も実行すること。
- **ゴールデンファイル**: 生成コードを意図的に変えた場合は `-update` でゴールデンファイルを更新し、差分を確認すること。
- **ワークスペース管理**: タスク進行メモや設計検討の作業ファイルはメインリポジトリにコミットせず、外部ワークスペース（`.ai-workspace/`）で管理すること。
