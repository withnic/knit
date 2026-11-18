# Contributing to knit

English | [日本語](#日本語)

Thank you for your interest in knit! Bug reports, documentation fixes and pull requests are all welcome.

## Before you start

- **Bugs**: open an issue with the bug report template. A minimal example module that reproduces the problem helps a lot.
- **Features and larger changes**: please open an issue first, so we can agree on the approach before you spend time on code.
- **Security issues**: do not open a public issue. Follow [SECURITY.md](SECURITY.md).

### Project scope

knit deliberately supports a small subset of google/wire. The following will **not** be added; each has a simple alternative documented in the [README](README.md#not-supported):

- `wire.Struct`, `wire.Value`, `wire.InterfaceValue`, `wire.FieldsOf`
- Variadic, method or generic providers and injectors

The only external dependency allowed is `golang.org/x/tools/go/packages`. The marker package (`knit.go`) must have no dependencies at all.

## Development setup

You need Go 1.26 or later and `make`. Lint and vulnerability tools are installed automatically, at pinned versions, into `./bin`.

```sh
git clone https://github.com/<your-account>/knit.git
cd knit
make help       # list all tasks
make check      # go.mod tidy, formatting, lint, unit tests (what CI runs)
make test-all   # all tests including end-to-end tests
make fmt        # format code
```

Design documents are in [docs/](docs/) (Japanese). Please read them before changing how code is loaded, resolved or generated.

## Making changes

1. Fork the repository and create a branch from `main`.
2. Make your change, with tests.
   - Prefer table-driven tests.
   - Keep `go test -short ./...` under one second; tests that run `go` commands must skip in `-short` mode.
   - Generated code must be deterministic: never depend on map iteration order.
   - Keep the two modes (`knit` and `wire`) symmetric: modes may only differ in marker package, build tag and output file name.
3. If you intentionally change the generated code, update the golden files and review the diff:

   ```sh
   go test ./internal/di -run TestGenerateFile_Golden -update
   git diff internal/di/testdata/golden
   ```

4. Run `make check` and `make test-all`.
5. Update the documentation (README, docs/) if behavior changes.

## Commit messages

We use [Conventional Commits](https://www.conventionalcommits.org/). Release notes are generated from them.

```
feat: add support for X
fix: handle Y when Z
docs: clarify migration steps
```

Common types: `feat`, `fix`, `docs`, `test`, `refactor`, `build`, `ci`, `chore`. Mark breaking changes with `!` (for example `feat!:`) and describe them in a `BREAKING CHANGE:` footer.

## Pull requests

- Fill in the pull request template.
- All CI checks must pass, and a maintainer review is required before merging.
- Pull requests are squash-merged, so the PR title should also follow Conventional Commits.
- For first-time contributors, CI starts after a maintainer approves the run.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](LICENSE).

---

## 日本語

knit に興味を持っていただきありがとうございます。バグ報告、ドキュメントの修正、プルリクエスト、どれも歓迎します。

## 始める前に

- **バグ**: バグ報告テンプレートで Issue を作ってください。問題を再現できる最小限のサンプルモジュールがあると、とても助かります。
- **機能追加や大きな変更**: コードを書く前に Issue で相談してください。方針を先に合わせておくと、作業が無駄になりません。
- **セキュリティの問題**: 公開の Issue には書かず、[SECURITY.md](SECURITY.md) の手順で報告してください。

### プロジェクトの範囲

knit は google/wire の機能のうち、意図的に小さなサブセットだけをサポートしています。次の機能は**追加しません**。どれも [README](README.ja.md#サポートしない機能) に簡単な代わりの書き方があります。

- `wire.Struct`、`wire.Value`、`wire.InterfaceValue`、`wire.FieldsOf`
- 可変長引数・メソッド・ジェネリックのプロバイダや Injector

外部依存として使ってよいのは `golang.org/x/tools/go/packages` だけです。マーカーパッケージ（`knit.go`）は依存なしに保ちます。

## 開発環境

Go 1.26 以上と `make` が必要です。リンターと脆弱性スキャンのツールは、バージョンを固定して `./bin` に自動でインストールされます。

```sh
git clone https://github.com/<あなたのアカウント>/knit.git
cd knit
make help       # タスクの一覧
make check      # go.mod の tidy、フォーマット、リント、ユニットテスト（CI と同じ）
make test-all   # E2E を含む全テスト
make fmt        # コードの整形
```

設計ドキュメントは [docs/](docs/) にあります。コードの読み込み・解決・生成の仕組みを変える場合は、先に目を通してください。

## 変更の手順

1. リポジトリをフォークし、`main` からブランチを作ります。
2. テストと一緒に変更します。
   - テストはテーブル駆動で書いてください。
   - `go test -short ./...` は 1 秒以内に保ちます。`go` コマンドを実行するテストは `-short` のときにスキップしてください。
   - 生成コードは決定論的にします。map の走査順に依存しないでください。
   - 2 つのモード（`knit` と `wire`）の違いは、マーカーパッケージ・ビルドタグ・出力ファイル名だけに保ちます。
3. 生成コードを意図的に変えた場合は、ゴールデンファイルを更新して差分を確認します。

   ```sh
   go test ./internal/di -run TestGenerateFile_Golden -update
   git diff internal/di/testdata/golden
   ```

4. `make check` と `make test-all` を実行します。
5. 動作が変わる場合は、ドキュメント（README、docs/）も更新します。

## コミットメッセージ

[Conventional Commits](https://www.conventionalcommits.org/ja/) の形式を使います。リリースノートはここから作られます。

```
feat: add support for X
fix: handle Y when Z
docs: clarify migration steps
```

よく使う種類: `feat`、`fix`、`docs`、`test`、`refactor`、`build`、`ci`、`chore`。互換性を壊す変更には `!` を付け（例: `feat!:`）、`BREAKING CHANGE:` の行で内容を説明してください。コミットメッセージは英語で書いてください。

## プルリクエスト

- PR テンプレートを埋めてください。
- マージには、すべての CI の通過とメンテナのレビューが必要です。
- PR はスカッシュマージするので、PR のタイトルも Conventional Commits の形式にしてください。
- 初めて PR を出す方の場合、メンテナが承認してから CI が動きます。

## ライセンス

コントリビュートいただいた内容は、[MIT ライセンス](LICENSE)で提供されることに同意したものとみなします。
