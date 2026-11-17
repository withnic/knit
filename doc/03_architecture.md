03. アーキテクチャ設計 (Architecture)
本ドキュメントでは、knit システムの全体構造、内部モジュールのレイアウト、およびコア機能（DIコード生成パイプライン・結合テスト逆引きパイプライン）の処理フローと詳細設計について記述します。


1. ディレクトリ構成
システムは明確に分離された責務を持つパッケージ群で構成されています。以下にプロジェクトの完全なディレクトリ構造と各ファイルの役割を示します。


knit/
├── cmd/
│   └── knit/
│       └── main.go              # CLIエントリーポイント (gen / tia サブコマンド)
├── internal/
│   ├── di/                      # DIコード生成エンジン (Wire代替)
│   │   ├── loader.go            # golang.org/x/tools/go/packages によるAST・型ロード
│   │   ├── resolver.go          # 依存グラフ構築・トポロジカルソート・循環参照検出
│   │   ├── generator.go         # go/format を使った決定論的コード生成 (wire_gen.go)
│   │   └── graph_export.go      # .knit/di-graph.json 出力
│   ├── tia/                     # 結合テスト影響範囲解析エンジン
│   │   ├── diff.go              # git diff --unified=0 パースと変更行抽出
│   │   ├── scanner.go           # go/parser 並列AST走査とシンボル・参照抽出
│   │   ├── analyzer.go          # 逆引きBFS探索・ハブ除外・DIサブグラフ絞り込み
│   │   └── runner.go            # go test コマンド組み立てと実行
│   └── shared/
│       └── graphschema/         # di-graph.json の共有型定義
├── docs/
│   ├── 01_system_overview.md
│   ├── 02_data_models.md
│   ├── 03_architecture.md
│   └── 04_guidelines.md
├── CLAUDE.md
└── go.mod
パッケージ構成概要
パッケージパス
主要機能・役割
cmd/knit
CLIコマンドのパーサーおよび各サブコマンドの初期実行制御を行うエントリーポイント。
internal/di
AST解析、型解釈、依存関係グラフの構築、コード生成、グラフJSONのエクスポートを担うコアモジュール。
internal/tia
Git差分解析、AST走査によるシンボル参照の抽出、逆引きBFSグラフ探索、テスト実行を担当。
internal/shared/graphschema
DI生成ステージとTIA解析ステージ間を橋渡しするデータ構造モデル定義。



2. DIコード生成パイプライン (gen)
gen パッケージは、Google Wire 互換の宣言的設定からコンパイル安全な依存注入コードを決定論的に生成します。
パイプライン処理ステップ
ステップ
コンポーネント
処理内容および詳細仕様
1. パッケージロード
 loader.go
golang.org/x/tools/go/packages を使用し、NeedName | NeedFiles | NeedSyntax | NeedTypes | NeedTypesInfo のフラグで要求パッケージ群を読み込む。wire.Build / wire.NewSet / wire.Bind の呼び出し箇所をASTレベルで特定し、完全な型情報を構築する。
2. セット平坦化とバインド解決
 resolver.go
ネストされた wire.NewSet を再帰的に展開し、フラットなプロバイダー一覧を作成する。wire.Bind 宣言を解析し、インターフェース型から具象型実装へのプロキシマッピングテーブルを生成する。
3. トポロジカルソートと循環検出
 resolver.go
Injector 関数の戻り値型を出発点とし、必要なプロバイダーの引数型を再帰的に解釈して有向グラフ（DAG）を構築する。Kahnのアルゴリズム等でトポロジカルソートを実施。依存関係の循環または未解決型が存在する場合、問題となる型パスを明示した詳細なエラーを発行する。
4. コード生成と整形
 generator.go
変数名の衝突を連番サフィックス（例: v1, v2）で回避。リソース初期化時のクリーンアップ関数（func()）を生成し、エラー発生時には LIFO（後入れ先出し）順で実行される後始末処理を付与する。最後に go/format を介して整形された wire_gen.go を書き出す。



3. 結合テスト逆引きパイプライン (tia)
tia パッケージは、ソースコードの差分から影響を受ける最小限の結合テスト群を特定し、効率的に実行するエンジンです。
パイプライン処理ステップ
ステップ
コンポーネント
処理内容および詳細仕様
1. 差分抽出
diff.go
git diff --unified=0 <base>...HEAD および git diff --unified=0 HEAD コマンドを実行。変更が発生したファイルとその行番号区間（DiffRange）を正規表現等で抽出・パースする。
2. 軽量並列AST走査
scanner.go
完全な型チェックを行わず、高速な go/parser（parser.ParseComments）を用いて全 .go ファイルを並列走査。トップレベル宣言（関数、メソッド、変数、定数、構造体、インターフェース）および各内部の識別子参照関係を迅速に抽出する。
3. シード特定と逆引きBFS探索
analyzer.go
シード特定: 変更行区間に含まれる宣言を探索開始点（Seed）とする。ファイル先頭やパッケージ記述などトップレベル変更の場合は該当ファイルの全宣言をシード化する。ハブ除外: NewServer や SetupRouter などの汎用初期化関数をハブノード（IsHub == true）として認識し、探索を停止することで影響範囲の無制限な波及（ハブ爆発）を防止する。インターフェース補完: メソッド名（BaseName）をキーとした呼び出し探索と、di-graph.json 内のサブグラフ情報を突き合わせ、動的呼び出しの漏れを抑止する。
4. テスト生成・実行
runner.go
解析結果から IsTest && IsIntegration の基準を満たすノードのみをフィルタリングし、パッケージごとに集約。go test -tags=integration -p 1 -run "^(TestA|TestB)$" ./pkg/... 形式の実行コマンドを組み立て、サブプロセスとしてテストを実行する。



4. 詳細設計仕様 (Design Review 追加事項)
4.1. gen パイプラインの実装詳細
packages.Load のオプションとビルドタグ制御
loader.go において Go パッケージ情報をロードする際、BuildFlags に []string{"-tags=wireinject"} を明示的に指定する必要があります。これにより、インジェクター宣言を含むソースファイルが正しくビルド対象として解釈されます。また、すでに生成済みの wire_gen.go が存在する場合は、生成コード内に設定された //go:build !wireinject ディレクティブによって自動的に読み込み対象から除外され、シンボルの重複やビルドタグの衝突が防止されます。

コード生成テンプレートとクリーンアップ処理のパターン
generator.go で生成されるインジェクター関数コードは、以下の仕様に従って構築されます。
インポートエイリアスの自動採番: 同一パッケージ名や異なるディレクトリからの同名パッケージが複数存在する場合、pkg1, pkg2 などの連番エイリアスを自動付与して衝突を回避します。
変数名スコープ管理: 戻り値や中間変数の識別子衝突を防ぐため、型ごとにスコープ内で唯一の変数名（例: v1, v2）を連番で生成します。
エラー時の LIFO クリーンアップ構造: 複数リソースの初期化途中でエラーが発生した場合、それまでに正常初期化されたリソースのみを逆順（LIFO）で破棄する cleanups []func() パターンを出力します。

func InitializeServer() (*Server, func(), error) {
    var cleanups []func()
    cleanup := func() {
        for i := len(cleanups) - 1; i >= 0; i-- {
            cleanups[i]()
        }
    }
    v1, cleanupV1, err := db.NewDatabase()
    if err != nil {
        return nil, nil, err
    }
    cleanups = append(cleanups, cleanupV1)
    v2, cleanupV2, err := logger.NewLogger()
    if err != nil {
        cleanup()
        return nil, nil, err
    }
    cleanups = append(cleanups, cleanupV2)
    server := &Server{DB: v1, Logger: v2}
    return server, cleanup, nil
}

4.2. tia パイプラインの実装詳細
ハブ爆発防止と影響集合の保持
逆引き BFS 探索中に NewServer, SetupRouter, Initialize* などのハブ関数・共通初期化ノードに到達した場合、そこからの親方向への無制限な探索波及を打ち止めます。ただし、打ち止め地点に達するまでに収集した「影響シンボル・型・ファイル集合」はそのまま保持され、次ステップの判定に入力として渡されます。

サブグラフ交差判定アルゴリズム (AND判定)
到達した影響シンボル集合と、.knit/di-graph.json に記録された各 Injector の依存ツリー（サブグラフ）との論理積（AND判定）を取ることで、実際に影響を受けるハンドラや Injector を精度高く割り出します。

for each injector in digraph.Injectors:
    affectedNodes = intersect(affectedSymbolsSet, injector.DependencyTreeNodes)
    if len(affectedNodes) > 0:
        markAsTargetIntegrationTest(injector.TargetTestCases)

同名メソッドの偽陽性フィルタリング
全プロジェクトを走査する際、異なる型に定義された同名メソッド（例: UserStore.Save() と OrderStore.Save()）による間違った依存接続（偽陽性）を防ぐため、各ファイルの AST インポート情報（ImportMap）を参照し、パッケージパスおよび型名の完全一致を確認してからグラフのエッジを接続します。

テスト関連ファイルおよび外部リソース変更時のフォールバック仕様
*_test.go の直接変更: 変更されたテストファイル内の該当テスト関数および、そのファイルが属するパッケージ内の影響テストを直接実行対象とします。
テストヘルパーの変更: ヘルパー関数を呼び出している全テストスイートを自動抽出して実行対象に追加します。
.sql / マイグレーションファイルの変更: 静的解析のみでは影響スコープの限定が困難なため、フォールバックとして全結合テスト（-tags=integration）を一括実行します。
