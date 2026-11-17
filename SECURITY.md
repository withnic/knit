# Security Policy

English | [日本語](#日本語)

## Supported versions

knit is maintained by an individual. Until v1.0.0, security fixes are made only in the latest release. Please upgrade to the latest version before reporting.

| Version | Supported |
|---|---|
| Latest release | ✅ |
| Older releases | ❌ |

## Reporting a vulnerability

**Please do not report security vulnerabilities through public GitHub issues, discussions or pull requests.**

Report them privately through GitHub instead:

1. Open the [Security tab](https://github.com/withnic/knit/security) of this repository.
2. Click **Report a vulnerability** ([direct link](https://github.com/withnic/knit/security/advisories/new)).

Please include as much of the following as you can:

- The affected version (`knit version`) and mode (`knit` or `wire`)
- Steps to reproduce, ideally a minimal example module
- The impact, and how an attacker could exploit it

## What to expect

This is a best-effort project maintained in spare time, so the following are goals rather than guarantees:

- An acknowledgement of your report within **7 days**, where possible
- Updates as the investigation progresses
- A fix released as soon as reasonably possible, followed by a published security advisory that credits you (unless you prefer to stay anonymous)

Please keep the details private until a fix has been released.

## Scope

In scope:

- The `knit` command (`cmd/knit`) and the code it generates
- The marker package `github.com/withnic/knit`
- The release artifacts published on GitHub Releases

Out of scope:

- Vulnerabilities in your own providers or application code
- Vulnerabilities in google/wire itself
- Issues in dependencies that do not affect knit (please report them upstream)

---

## 日本語

## サポート対象のバージョン

knit は個人でメンテナンスしています。v1.0.0 になるまでは、セキュリティ修正は最新リリースにだけ入れます。報告の前に最新版にアップデートしてください。

| バージョン | サポート |
|---|---|
| 最新リリース | ✅ |
| それより前のリリース | ❌ |

## 脆弱性の報告方法

**セキュリティ上の問題は、公開の Issue・Discussion・プルリクエストには書かないでください。**

代わりに、GitHub から非公開で報告してください。

1. このリポジトリの [Security タブ](https://github.com/withnic/knit/security) を開きます。
2. **Report a vulnerability** をクリックします（[直接リンク](https://github.com/withnic/knit/security/advisories/new)）。

できる範囲で、以下を含めてください。

- 該当するバージョン（`knit version`）とモード（`knit` または `wire`）
- 再現手順（できれば最小限のサンプルモジュール）
- 影響と、攻撃者がどう悪用できるか

## 対応について

空き時間でできる範囲で対応しているプロジェクトのため、以下は保証ではなく目安です。

- できる範囲で、**7 日以内**に受け付けの連絡をします
- 調査の進み具合に応じて連絡します
- できるだけ早く修正をリリースし、その後セキュリティアドバイザリを公開します。報告者のお名前を掲載します（匿名を希望される場合は掲載しません）

修正がリリースされるまでは、詳細を公開しないようお願いします。

## 対象範囲

対象:

- `knit` コマンド（`cmd/knit`）と、それが生成するコード
- マーカーパッケージ `github.com/withnic/knit`
- GitHub Releases で公開しているリリース物

対象外:

- ご自身のプロバイダやアプリケーションコードの脆弱性
- google/wire 自体の脆弱性
- knit に影響しない依存ライブラリの問題（依存元に報告してください）
