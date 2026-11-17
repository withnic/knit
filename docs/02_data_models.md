02. データモデル定義 (Data Models)
本システムはデータベースを持たず、GoのAST・型情報から抽出するメモリ上のグラフ構造と、中間成果物であるJSONスキーマをデータモデルとして定義します。


1. DIコード生成エンジン用データモデル (internal/di/model.go)
DIコード生成エンジンは、Goソースコードの構文解析および型解析を行い、依存関係の構築に必要なメタデータをメモリ上に保持します。
1.1 ProviderNode
コンストラクタ関数（Provider）1つを表すノード構造体です。

フィールド名
型
説明
ID
 string
一意識別子（例: github.com/org/repo/internal/repo.NewUserRepository）
PkgPath
 string
パッケージのインポートパス
PkgName
 string
パッケージ名
FuncName
 string
コンストラクタ関数名
Params
 []ParamSpec
引数リスト（型情報 TypeKey と変数名ヒント）
OutputType
 TypeKey
提供する主要な戻り値の型
HasCleanup
 bool
第2戻り値に func() クリーンアップ関数を持つか
HasError
 bool
最終戻り値に error を持つか

1.2 BindingSpec
wire.Bind(new(Iface), new(Impl)) によって定義されるインターフェースと具象型の対応関係を表します。

フィールド名
型
説明
InterfaceType
TypeKey
インターフェースの完全修飾型名
ConcreteType
TypeKey
実装する具象型の完全修飾型名（ポインタ型含む）

1.3 InjectorSpec
wire.Build(...) を含むInjector関数の定義を表します。

フィールド名
型
説明
FuncName
 string
生成する関数名（例: InitializeUserHandler）
PkgPath
 string
出力先パッケージパス
FilePath
 string
元の wire.go ファイルパス
Params
 []ParamSpec
Injector自身が外部から受け取る引数（例: ctx context.Context, db *spanner.Client）
ReturnType
 TypeKey
構築対象のルート型
HasCleanup
 bool
Injectorがクリーンアップ関数を返すか
HasError
 bool
Injectorが error を返すか
Providers
 []ProviderNode
wire.NewSet を平坦化（Flatten）して展開済みの有効Provider一覧
Bindings
 []BindingSpec
有効なBind一覧



2. 共有依存グラフJSONスキーマ (.knit/di-graph.json)
gen コマンドが出力し、tia コマンドが読み込む中間JSONファイルです。各Injector（Handlerやバッチ単位のサブグラフ）がどの型・コンストラクタに依存しているかを記録します。


{
  "version": "1.0",
  "generated_at": "2026-11-17T21:00:00+09:00",
  "injectors": [
    {
      "name": "InitializeUserHandler",
      "pkg_path": "github.com/org/repo/internal/di",
      "root_type": "*github.com/org/repo/internal/handler.UserHandler",
      "dependencies": [
        {
          "pkg_path": "github.com/org/repo/internal/handler",
          "pkg_dir": "./internal/handler",
          "type": "*github.com/org/repo/internal/handler.UserHandler",
          "provider": "github.com/org/repo/internal/handler.NewUserHandler"
        },
        {
          "pkg_path": "github.com/org/repo/internal/service",
          "pkg_dir": "./internal/service",
          "type": "github.com/org/repo/internal/service.UserService",
          "provider": "github.com/org/repo/internal/service.NewUserService",
          "bound_from": "*github.com/org/repo/internal/service.userServiceImpl"
        }
      ]
    }
  ]
}


3. テスト影響範囲解析用データモデル (internal/tia/model.go)
コード差分から影響を受けるテストを効率的に逆引き・特定するためのデータ構造です。
3.1 SymbolNode
AST走査（go/parser）によって抽出されるトップレベル宣言のノードです。

フィールド名
型
説明
ID
 string
一意識別子（例: ./internal/service.(*userServiceImpl).Create）
Kind
 SymbolKind
Func / Method / Var / Const / Struct / Interface
BaseName
 string
識別子のベース名（例: Create, MaxRetry, UserRepository）
ReceiverType
 string
メソッドの場合のレシーバ型名（例: *userServiceImpl）
PkgDir
 string
リポジトリルートからの相対ディレクトリパス（例: ./internal/service）
FilePath
 string
リポジトリルートからの相対ファイルパス
StartLine
 int
ASTノード開始行
EndLine
 int
ASTノード終了行
IsTest
 bool
*_test.go 内の TestXxx 関数であるか
IsIntegration
 bool
結合テスト対象ファイル（//go:build integration または *_integration_test.go）であるか
IsHub
 bool
ルーターやDI初期化など逆引き打ち止め対象のハブ関数であるか
Refs
 []string
この宣言内部で参照・呼び出ししている外部識別子（BaseName）のリスト

3.2 DiffRange
git diff --unified=0 から抽出されるファイルごとの変更行区間です。

フィールド名
型
説明
FilePath
 string
変更された .go ファイルの相対パス
StartLine
 int
変更開始行
EndLine
 int
変更終了行
3.3 ImportMap & RepoIndex
テスト影響範囲解析エンジンがプロジェクト全体の依存解析およびモジュール境界判定を行うための補足構造体です。
ImportMap:
ソースファイルごとのインポートマップ（map[string]string）を保持します。パッケージエイリアスまたはパッケージ短縮名から、対応するフルインポートパスへのマッピングを行います。
フィールド名
型
説明
FilePath
string
対象のGoファイルパス
Imports
map[string]string
エイリアス/短縮名 -> フルインポートパス

RepoIndex:
リポジトリ全体レベルのメタデータおよび go.mod の情報を統合管理する構造体です。
フィールド名
型
説明
ModulePath
string
go.mod で定義されたルートモジュールパス
Symbols
[]SymbolNode
全パッケージのシンボルノード一覧
FileImports
map[string]map[string]string
ファイルパスごとの ImportMap 索引





1.4 TypeKey 仕様と正規化ルール
TypeKey は、Go言語における型が一意かつ厳密に識別できるよう定義された完全修飾型名文字列です。
完全修飾フォーマット例:
github.com/org/repo/internal/service.UserService （インターフェース・値型）
*github.com/org/repo/internal/user.ID （ポインタ型）
正規化ルール (Normalization Rules):
ポインタ型と値型の厳密な区別: 先頭の * の有無を保持し、値型とポインタ型を異なるキーとして厳密に区別します。
同名型の衝突防止: パッケージのフルインポートパス（例: github.com/org/repo/internal/user）を含めることで、別パッケージにおける同名型の名称衝突を回避します。
可変長引数の拒否: コンストラクタや Injector の引数における可変長引数（...T）は TypeKey として許可されず、明示的なスライス型（[]T）への展開またはバリデーションエラーとして処理されます。
