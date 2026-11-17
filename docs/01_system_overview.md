# 01. システム概要 (System Overview)

## 1. 背景と目的

### 独自の DI コードジェネレータと google/wire 互換モード

google/wire が公式にアーカイブされたことを受け、wire と同じ宣言スタイル（Injector 関数に `Build(...)` を書き、プロバイダを列挙する）で使える軽量な DI コードジェネレータ **knit** を提供します。

- **knit モード（デフォルト）**: knit 自身のマーカーパッケージ `github.com/withnic/knit`（`knit.Build` / `knit.NewSet` / `knit.Bind`）で宣言し、`knit_gen.go` を生成します。google/wire への依存は不要です。
- **wire モード（`-mode=wire`）**: google/wire と同じ入出力（`wire.Build`、`//go:build wireinject`、`wire_gen.go`）のドロップイン代替です。既存の wire プロジェクトを修正なしでそのまま扱えます。

両モードの生成ロジックは共通で、違いは「マーカーパッケージ」「ビルドタグ」「出力ファイル名」だけです。

### 依存グラフのメタデータ出力

コード生成と同時に、各 Injector がどのプロバイダ・型に依存しているかを `.knit/di-graph.json` として出力します。このファイルはバージョン付きの安定した出力であり、テスト影響範囲解析などの外部ツールが DI の依存関係を利用するための入力になります。

## 2. コア機能とスコープ

google/wire の構文と互換性のある最小限のコア機能のみをサポートし、シンプルな解析と高速なコード生成を実現します。

### 2.1 モード

| | knit モード（デフォルト） | wire モード（`-mode=wire`） |
|---|---|---|
| マーカーパッケージ | `github.com/withnic/knit` | `github.com/google/wire` |
| 宣言 | `knit.Build` / `knit.NewSet` / `knit.Bind` | `wire.Build` / `wire.NewSet` / `wire.Bind` |
| Injector ファイルのビルドタグ | `//go:build knitinject` | `//go:build wireinject` |
| 生成ファイル | `knit_gen.go`（`//go:build !knitinject`） | `wire_gen.go`（`//go:build !wireinject`） |

各モードは自分のマーカーパッケージの呼び出しだけを Injector として認識します（もう一方のモードの宣言は無視します）。

knit モードの宣言例:

```go
//go:build knitinject

package di

import "github.com/withnic/knit"

var StoreSet = knit.NewSet(NewSQLStore, knit.Bind(new(Store), new(*SQLStore)))

func InitializeServer(ctx context.Context) (*Server, func(), error) {
	panic(knit.Build(StoreSet, NewServer))
}
```

### 2.2 サポート対象

以下、`Build` などはどちらのモードのマーカーパッケージでも同じ意味です。

| 機能 | 内容 |
|---|---|
| `Build(...)` | Injector 関数の定義と依存関係グラフの自動解決 |
| `NewSet(...)` | プロバイダ群と Bind 定義のグループ化・再利用（ネスト、別パッケージのセット変数参照を含む） |
| `Bind(new(Iface), new(*Impl))` | インターフェース型と具象型のバインディング |
| プロバイダ関数 | 通常の `NewXxx` / `ProvideXxx` 関数。戻り値は `(T)`、`(T, error)`、`(T, func())`、`(T, func(), error)` の 4 パターンを厳密に扱う |
| Injector の引数 | Injector 自身の引数（例: `ctx context.Context`）を依存の供給源として扱う |
| クリーンアップ | プロバイダのクリーンアップ関数を集約し、途中でエラーが発生した場合は成功済みのリソースを LIFO（後入れ先出し）で破棄する |
| 依存グラフ出力 | 解決した依存グラフを `.knit/di-graph.json` に出力する |

### 2.3 スコープ外（意図的に排除する機能）

以下は実装しません。wire モードで検出した場合は、分かりやすいエラーで停止します（knit のマーカーパッケージにはそもそも存在しません）。

| 機能 | 代替手段 |
|---|---|
| `wire.Struct` | 明示的な `NewXxx` コンストラクタ関数を書く |
| `wire.Value` / `wire.InterfaceValue` | 1 行の `ProvideXxx` 関数を書く |
| `wire.FieldsOf` | 1 行の Getter 関数を書く |
| 可変長引数（`...T`）を持つプロバイダ・Injector | 明示的なスライス型（`[]T`）や専用プロバイダに置き換える |
| メソッドの Injector・プロバイダ、ジェネリックなプロバイダ・Injector、1 関数内の複数の `Build` | 通常の関数に書き換える |

## 3. CLI インターフェース仕様

```
knit gen [flags] [packages...]
knit version
```

`gen` はパッケージパターン（省略時 `./...`）に含まれる、選択したモードのすべての Injector を解決し、パッケージごとに 1 ファイルを生成します。全 Injector のエラーをまとめて報告します。内容が変わらないファイルは書き換えません。

| フラグ | 型 | デフォルト値 | 説明 |
|---|---|---|---|
| `-mode` | string | `knit` | 宣言の方言。`knit` または `wire`（§2.1） |
| `-out` | string | モードに応じて `knit_gen.go` / `wire_gen.go` | 各パッケージディレクトリに生成するファイル名（ディレクトリを含まない `.go` ファイル名） |
| `-graph-out` | string | `.knit/di-graph.json` | 依存グラフ JSON の出力先。空文字で出力しない |
| `-dry-run` | bool | `false` | ファイルを書き込まず、生成コードを標準出力に表示する（`// ==> パス <==` 区切り） |

書き込んだファイルの一覧は標準エラーに出力します。Injector が 1 つも見つからない場合は、探したマーカーパッケージ・ビルドタグと、もう一方のモードを示すエラーを返します（モードの指定忘れに気付けるように）。

### 終了コード

| コード | 意味 |
|---|---|
| 0 | 成功 |
| 1 | 解析・解決・生成・書き込みのエラー |
| 2 | 未知のコマンド、フラグの誤り（未知の `-mode` を含む） |

## 4. 非機能要件

- **決定論的出力 (Deterministic Output)**: 生成コードと依存グラフは map の走査順序などに依存せず、同じ入力から常にバイト単位で同一の結果を出力すること（`generated_at` を除く）。
- **コンパイル可能性**: 生成コードは `go/format` で整形済みであり、そのままコンパイル・実行できること。
- **依存の最小化**: 外部依存は `golang.org/x/tools/go/packages` のみとすること。
