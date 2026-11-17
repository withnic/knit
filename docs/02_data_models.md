# 02. データモデル定義 (Data Models)

knit はデータベースを持たず、Go の AST・型情報から抽出するメモリ上の構造と、出力成果物である依存グラフ JSON をデータモデルとします。

## 1. DI コード生成エンジン用データモデル (`internal/di/model.go`)

### 1.1 ProviderNode

コンストラクタ関数（プロバイダ）1 つを表します。

| フィールド | 型 | 説明 |
|---|---|---|
| `ID` | string | 一意識別子（例: `github.com/org/repo/internal/repo.NewUserRepository`） |
| `PkgPath` | string | パッケージの import パス |
| `PkgName` | string | パッケージ名 |
| `FuncName` | string | 関数名 |
| `Params` | []ParamSpec | 引数リスト（型 `TypeKey` と変数名ヒント） |
| `OutputType` | TypeKey | 提供する型（第 1 戻り値） |
| `HasCleanup` | bool | 第 2 戻り値に `func()` クリーンアップ関数を持つか |
| `HasError` | bool | 最終戻り値に `error` を持つか |

### 1.2 BindingSpec

`Bind(new(Iface), new(Impl))`（`knit.Bind` / `wire.Bind`）によるインターフェースと具象型の対応です。

| フィールド | 型 | 説明 |
|---|---|---|
| `InterfaceType` | TypeKey | インターフェースの完全修飾型名 |
| `ConcreteType` | TypeKey | 実装する具象型の完全修飾型名（ポインタ型を含む） |

### 1.3 InjectorSpec

`Build(...)`（`knit.Build` / `wire.Build`）を含む Injector 関数の定義です。

| フィールド | 型 | 説明 |
|---|---|---|
| `FuncName` | string | 生成する関数名（例: `InitializeUserHandler`） |
| `PkgPath` / `PkgName` | string | 出力先パッケージの import パスとパッケージ名 |
| `FilePath` | string | 宣言元のファイル（`knit.go`、`wire.go` など）のパス |
| `Params` | []ParamSpec | Injector 自身が受け取る引数（例: `ctx context.Context`）。同じ型の引数の重複はエラー |
| `ReturnType` | TypeKey | 構築対象のルート型 |
| `HasCleanup` | bool | Injector がクリーンアップ関数を返すか |
| `HasError` | bool | Injector が `error` を返すか |
| `Providers` | []ProviderNode | `NewSet` を平坦化した有効なプロバイダ一覧（ID 順） |
| `Bindings` | []BindingSpec | 有効な Bind 一覧（InterfaceType 順） |
| `Func` | *types.Func | Loader が設定する型チェック済みの Injector 宣言。コード生成時のシグネチャ・import・ゼロ値の描画に使う（手組みの Spec では nil） |

### 1.4 TypeKey 仕様と正規化ルール

`TypeKey` は、Go の型を一意かつ厳密に識別する完全修飾型名の文字列です。

- `github.com/org/repo/internal/service.UserService`（インターフェース・値型）
- `*github.com/org/repo/internal/user.ID`（ポインタ型）

正規化ルール:

- **ポインタ型と値型の区別**: 先頭の `*` の有無を保持し、別のキーとして扱う。
- **同名型の衝突防止**: パッケージの完全な import パスを含め、別パッケージの同名型（`pkga.Config` と `pkgb.Config`）を区別する。
- **型エイリアス**: トップレベルのエイリアスは実体の型に解決する。
- **可変長引数の拒否**: プロバイダ・Injector の可変長引数（`...T`）は TypeKey にせず、検証エラーとする。

### 1.5 解決結果 (`internal/di/resolver.go`)

| 型 | 説明 |
|---|---|
| `ResolvedInjector` | Injector の依存計画。`Steps`（トポロジカル順のプロバイダ呼び出し）と `Root`（返す値） |
| `Step` | 1 回のプロバイダ呼び出し。`Args` は各引数に渡す値 |
| `ValueRef` | 値の供給元。`Kind` が `FromParam`（Injector 引数）または `FromProvider`（先行ステップ）、`BoundFrom` は Bind 元の具象型 |
| `ResolveError` | 解決エラー。`Injector`、`Kind`（provider not found / duplicate provider / circular dependency / cleanup・error の不足）、`Chain`（ルート型から失敗した型までの依存チェーン） |

### 1.6 Mode (`internal/di/mode.go`)

宣言の方言を表します。ロード時のマーカーパッケージ・ビルドタグと、生成ファイルのビルド制約・ファイル名を決めます。ゼロ値は `KnitMode` として扱います。

| フィールド | 型 | 説明 | `KnitMode` | `WireMode` |
|---|---|---|---|---|
| `Name` | string | モード名（`-mode` の値、エラーメッセージ上の表記） | `knit` | `wire` |
| `MarkerPkgPath` | string | `Build` / `NewSet` / `Bind` を提供するパッケージ | `github.com/withnic/knit` | `github.com/google/wire` |
| `InjectTag` | string | Injector ファイルのビルドタグ（生成ファイルはその否定） | `knitinject` | `wireinject` |
| `OutFile` | string | デフォルトの生成ファイル名 | `knit_gen.go` | `wire_gen.go` |

## 2. 依存グラフ JSON スキーマ (`.knit/di-graph.json`)

`knit gen` が出力するファイルです（モードによらず同じ形式）。各 Injector がどの型・プロバイダに依存しているかを記録します。外部ツール向けの安定した出力であり、フォーマットを変更する場合は `version` を上げます。型定義は `internal/graphschema` にあります。

```json
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
```

| フィールド | 説明 |
|---|---|
| `version` | スキーマバージョン（現在 `"1.0"`）。読み込み時に一致しないものは拒否する |
| `generated_at` | 生成時刻（RFC 3339） |
| `injectors` | Injector ごとのサブグラフ。`pkg_path`、`name` の順でソート |
| `injectors[].name` / `pkg_path` | Injector 関数名と、それを宣言するパッケージの import パス |
| `injectors[].root_type` | Injector が返す型（TypeKey） |
| `dependencies` | 構築される型の一覧。`type`、`provider` の順でソート |
| `dependencies[].type` | 供給される型（TypeKey） |
| `dependencies[].provider` | その型を提供するプロバイダ関数の ID |
| `dependencies[].pkg_path` | プロバイダのパッケージの import パス |
| `dependencies[].pkg_dir` | プロバイダのパッケージのモジュールルートからの相対ディレクトリ（`./internal/handler`、ルートは `.`、モジュール外は空） |
| `dependencies[].bound_from` | `Bind` によって満たされたインターフェースの場合、束縛元の具象型（省略可） |

`dependencies` には、プロバイダ呼び出しごとのエントリに加え、`Bind` 経由で満たされたインターフェースごとに `bound_from` 付きのエントリが追加されます（Injector 引数から供給される型は含みません）。
