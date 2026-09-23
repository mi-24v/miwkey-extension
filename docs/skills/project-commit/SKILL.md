---
name: project-commit
description: Use when proposing or creating commits in the miwkey-extension repository.
---

# Project Commit

このリポジトリのコミットメッセージとコマンド提示に関する規約。
`git-commit` スキルを補足し、メッセージ形式には以下を適用する。

## メッセージ

- Conventional Commits の `<type>(<scope>): <summary>` 形式を使う。
- summary は英語の命令形で、72文字以内にする。
- リポジトリルートの [.gitmessage](../../../.gitmessage) を読み、
  `Why` / `What` / `How` / `Tests` / `Notes` の全項目を具体的に記入する。
  本文は省略しない。特記事項がない場合も `Notes` にその旨を書く。
- `Tests` には実行した検証コマンドと結果を記載する。
  テストを実行していない場合は、その理由を明記する。

## コマンドの提示

コミットを提案する際は、実際に使うファイル名と記入済みのメッセージを含め、
そのまま実行できるコマンドを提示する。

- `git status` と `git diff --stat` を含める。
- `git commit -am ...` または対象ファイルを明示した
  `git add ...` と `git commit ...` の手順を含める。
- 新規ファイルは `git commit -am` では追加されないため、
  `git add` を使う。本文の改行を保持するため、`git commit -F` で
  メッセージファイルまたは引用付き heredoc を渡せる。
