# リポジトリ固有の指示

## 概要・構成

Misskey の通知を扱う Go 製の拡張 API サーバー。HTTP は Echo、永続化は DynamoDB を使う。

| パス | 役割 |
| --- | --- |
| `app/` | 起動処理、HTTP サーバー、ヘルスチェック |
| `notification/` | 通知の HTTP ハンドラー、サービス、ストアのインターフェース |
| `model/` | 通知モデルと ID 型 |
| `internal/db/dynamo/` | DynamoDB ストアとテーブル初期化 |
| `internal/infra/` | JWT 認証と共有シークレットの読み込み |
| `internal/ci/` | Dockerfile・イメージビルド workflow の検証テスト |
| `api/openapi.yaml` | 通知 API の仕様 |
| `misskey/` | 連携先の Misskey fork（Git サブモジュール） |

## 開発・検証

以下はリポジトリルートで実行する。Go の設定は [go.mod](go.mod)、
コンテナのビルド環境は [Dockerfile](Dockerfile) を参照する。

```bash
# Run the same Go tests as CI
go test ./app ./internal/... ./model ./notification

# Build the extension server
go build -o /tmp/miwkey-extension ./app

# Start after configuring authentication and AWS access
go run ./app

# Check a running server
/tmp/miwkey-extension healthcheck

# Build the extension container locally
docker build -t miwkey-extension:local .
```

Go 側の単体テストはストアのモックなどを使用する。サーバー起動や
DynamoDB を使う結合検証とは区別し、Misskey 側のテストは `misskey/` の手順に従う。
CI の実行内容は [.github/workflows/test.yml](.github/workflows/test.yml) を参照する。

## 起動設定・連携上の注意

- サーバーは `8080` 番ポートで待ち受ける。通知 API は `/api/v1` 配下で Bearer JWT 認証を使う。
- `AUTH_SECRET` を設定するか、`AUTH_SECRET_NAME` で Secrets Manager のシークレットを指定する。
  両方ある場合は `AUTH_SECRET` が優先される。DynamoDB と、使用する場合は Secrets Manager への
  AWS 認証・リージョン設定が必要。
- `DYNAMODB_NOTIFICATION_TABLE` の既定値は `Notifications`。
  `DYNAMODB_AUTO_CREATE=true` のときだけ、未作成のテーブルと GSI を作成する。
  ローカル検証用の接続先を確認して使い、本番ではこの変数を未設定にして AWS 側でテーブルを管理する。
- `GET /healthz` は認証なしで `204` を返す。通知データへのアクセスを追加しない。
  コンテナは distroless のため、ヘルスチェックはバイナリの `healthcheck` サブコマンドを使う。
- ログは stdout/stderr に出す。通知本文、DynamoDB のアイテム内容、JWT、Authorization ヘッダー、
  共有シークレットをログに含めない。

## Misskey サブモジュールと変更範囲

- `misskey/` は独立した Git リポジトリ。必要な場合は `git submodule update --init --recursive` で取得する。
  接続先は [.gitmodules](.gitmodules) を参照し、SSH ホスト別名 `github-miwkey` の設定とアクセス権を確認する。
- Misskey 側を変更する場合は、配下の指示を確認してそこで検証・コミットした後、
  親リポジトリでサブモジュールの参照コミットを更新する。ブランチ名だけで更新先を判断しない。
- このリポジトリのイメージビルド対象は拡張サーバー。
  Misskey 本体のイメージ公開は Misskey fork、AWS リソースとデプロイ構成は `miwkey-aws` が担当する。

## 仕様の参照先

- API を変更するときは [OpenAPI](api/openapi.yaml)、`notification/` の実装、`model/` の型、
  Misskey 側の呼び出しとの整合性を確認する。
- 起動設定、連携、ヘルスチェック、ログ、イメージ公開の設計は
  [Runtime Handoff Design](docs/superpowers/specs/2026-07-22-notification-extension-runtime-handoff-design.md) を参照する。
  実装状況や実行コマンドは、現在のコードと workflow でも確認する。

## コミット

このリポジトリでコミットを提案・実行する際は、`git-commit` スキルに加えて
[project-commit](docs/skills/project-commit/SKILL.md) を読み、適用してください。
コミットメッセージの形式は、このリポジトリのスキルと
[.gitmessage](.gitmessage) に従ってください。
