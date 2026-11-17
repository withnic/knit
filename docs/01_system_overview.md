01. システム概要 (System Overview)
1. 背景と目的
google/wire アーカイブに伴うドロップイン代替の提供
google/wire が公式にアーカイブされたことを受け、既存のGoプロジェクトが大きな修正コストをかけずに安全かつ円滑に移行できるよう、軽量なドロップイン代替となるDIコードジェネレータ（gen コマンド）を提供します。
AST依存解析基盤による結合テスト（Integration Test）の高速化
Cloud Spanner エミュレータ等の外部リソースを使用する重い結合テストの実行時間を短縮するため、git diff の変更箇所からAST（抽象構文木）の依存・参照関係を逆引きし、影響を受ける結合テストのみをピンポイントで抽出・実行するTest Impact Analysis機能（tia コマンド）を同一のAST解析基盤上で実現します。
ハイブリッド開発フローの実現
メモリ内で完結するSmallテストは標準の go test -short ./... で全件高速実行し、実行コストの大きいMediumテスト（DB結合テスト等）のみを本ツールでピンポイント抽出・実行する、効率的なハイブリッド開発フローを支えます。
2. コア機能とスコープ
2.1 DIコード生成機能 (knit gen)
既存の wire.go 構文と互換性のある最小限のコア機能のみをサポートし、シンプルなAST解析と高速なコード生成を実現します。

サポート対象機能
wire.Build(...): Injector関数の定義および依存関係グラフの自動解決
wire.NewSet(...): Provider群およびBind定義のグループ化・再利用構造のサポート
wire.Bind(new(Interface), new(*Impl)): インターフェース型と具象型の明確なバインディング
コンストラクタ関数（Provider）のサポート: 通常の NewXxx / ProvideXxx 関数に対応。戻り値シグネチャは以下の4パターンを厳密に処理（クリーンアップ関数の逆順実行を含む）
(T)
(T, error)
(T, func())
(T, func(), error)
依存グラフメタデータ出力: 解決したDI依存グラフを .knit/di-graph.json として同時出力し、TIA機能でのサブグラフ判定に活用
スコープ外（意図的に排除する機能）
wire.Struct: すべて明示的な NewXxx コンストラクタ関数で記述する運用とし、サポート対象外
wire.Value / wire.InterfaceValue: 1行の ProvideXxx 関数で容易に代替可能なため排除
wire.FieldsOf: 初期フェーズでは対象外とし、1行のGetter関数で代替（将来的な拡張余地のみ残す）
2.2 結合テスト影響範囲抽出機能 (knit tia)
変更の検知
git diff <base>...HEAD および未コミットの変更（working tree）から、変更された .go ファイルおよび該当行番号を取得します。
シード（Seed）要素の抽出
変更行に該当するトップレベル宣言（関数・メソッド、パッケージ変数 var、定数 const、構造体 type struct、インターフェース type interface）を解析の起点（Seed）として抽出します。
影響範囲の逆引きとハブ爆発の防止
リポジトリ全体のAST参照・呼び出し関係とDI依存グラフを組み合わせ、NewServer や SetupRouter 等の共通初期化関数を経由した全APIテストへの過剰波及（ハブ爆発）を防止しつつ、影響を受ける結合テスト（//go:build integration タグまたは *_integration_test.go）の TestXxx 関数のみを正確に抽出します。
3. CLIインターフェース仕様
3.1 gen コマンド
DIコードの生成および依存関係グラフメタデータの出力を行います。

knit gen [flags] [packages...]

フラグ
型
デフォルト値
説明
-out
 string
wire_gen.go
生成されるコードの出力ファイル名
-graph-out
 string
.knit/di-graph.json
TIA機能で使用するDI依存グラフJSONの出力先
-dry-run
 bool
false
ファイルへの書き込みを行わず、標準出力にコードを表示

3.2 tia コマンド
変更差分から影響を受ける結合テストを抽出し、必要に応じて実行します。

knit tia [flags]

フラグ
型
デフォルト値
説明
-base
 string
main
比較対象とするベースブランチ名
-tag
 string
integration
抽出対象とする結合テストのビルドタグ
-pattern
 string
_integration_test.go
抽出対象とする結合テストのファイル名サフィックス
-dirty
 bool
true
作業ディレクトリの未コミット変更を含めて解析するか
-exec
 bool
false
抽出した go test コマンドをその場で自動実行するか
-parallel-pkg
 int
1
go test -p に渡すパッケージ並列数（DB競合防止用）
-hub-exclude
 string
(空)
逆引き探索の打ち止め（Stop Node）とするハブ関数名パターン（例: NewServer,SetupRouter,Initialize）

4. 非機能要件
高速動作
tia コマンドは go/parser をベースとした軽量なAST走査を行うことで、中〜大規模リポジトリにおいても解析を1秒以内で完了させること。
決定論的出力 (Deterministic Output)
gen コマンドによって生成されるGoコードおよび tia コマンドによって出力されるテストリストは、Mapの走査順序等に依存せず、常にソートされた決定論的な結果を出力すること。

