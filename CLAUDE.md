CLAUDE.md
このファイルは、Claude Code が本リポジトリ（knit）で開発・設計を行う際の動作指針とルールを定義します。
1. プロジェクト概要
knit は以下の2つの機能を提供するGo製CLIツールです。

gen サブコマンド
アーカイブされた google/wire の軽量ドロップイン代替DIコードジェネレータ。
wire.Build, wire.NewSet, wire.Bind, 通常の NewXxx 関数＋クリーンアップ関数に対応し、依存グラフJSONも出力します。
tia サブコマンド
git diff で変更された関数・変数・定数・構造体・インターフェースを起点にASTの参照関係を逆引きし、ハブ関数（NewServer 等）による過剰波及を防ぎつつ、影響を受ける結合テスト（//go:build integration / *_integration_test.go）のみを抽出・実行するTest Impact Analysisツール。
2. 必須参照ドキュメント
実装・修正を行う前に、必ず docs/ 配下の設計ドキュメントを確認すること。

ドキュメントパス
概要・確認内容
docs/01_system_overview.md
コア機能、サポート対象／非対象のスコープ、CLIフラグ仕様
docs/02_data_models.md
ProviderNode, InjectorSpec, di-graph.json, SymbolNode の構造定義
docs/03_architecture.md
パッケージ構成、コード生成およびAST逆引きパイプラインのアルゴリズム
docs/04_guidelines.md
コーディング規約、決定論的出力ルール、テスト戦略、禁止事項

3. ビルド・テストコマンド
開発操作はすべて Makefile 経由で実行する（`make help` で一覧表示）。ツール類（golangci-lint, govulncheck）はバージョン固定で ./bin に自動インストールされる。

全ユニットテスト実行（高速）
make test          # go test -short ./...
結合テスト含む全テスト実行
make test-all      # go test ./...
フォーマット / リント
make fmt           # gofumpt + goimports
make lint          # golangci-lint（.golangci.yml）
CI 相当の一括チェック（tidy・フォーマット・リント・ユニットテスト）
make check
脆弱性スキャン / カバレッジ
make vuln
make cover
バイナリビルド
make build         # bin/knit
4. 開発・実装ルール
スコープ厳守
wire.Struct や wire.Value などの対象外機能は実装しないこと。
検証の徹底
コードを変更したら必ず make check を実行し、フォーマット・リント・ユニットテストがすべてパスすることを確認してから完了報告を行うこと。
ワークスペース管理
タスク進行メモや設計検討の作業ファイルはメインリポジトリにコミットせず、外部ワークスペース（.ai-workspace/）で管理すること。
5. 実装ロードマップ
開発は以下の4つのフェーズに分けて進められます。各フェーズの詳細な要件やガイドラインについては docs/04_guidelines.md を参照してください。

Phase 1: コア解析 Engine & Wire サポート
AST解析、wire.Build / wire.NewSet / wire.Bind の抽出、依存グラフ構築基盤の実装。
Phase 2: コード生成 & di-graph.json 出力
依存関係に基づく最適ソート、決定論的なGoコード生成、および di-graph.json の出力処理の実装。
Phase 3: TIA Engine & 影響テスト抽出
git diff 解析、AST参照関係の逆引きパイプライン、ハブ関数検出および影響を受ける結合テストの特定処理の実装。
Phase 4: CLI インテグレーション & 検証
gen / tia サブコマンドの統合、エラーハンドリングの強化、統合テストと全体検証の実施。

