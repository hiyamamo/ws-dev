# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## リポジトリ概要

このリポジトリは **ws-dev CLI** の開発リポジトリ。Go 製の単一バイナリで、同一リポジトリを複数クローンして並列開発するワークフローを提供する。

- `cmd/ws-dev/` — エントリポイント（cobra ルート）
- `internal/` — 各機能モジュール（下記）

## ビルドとテスト

```bash
mise exec -- go build -o ws-dev ./cmd/ws-dev
mise exec -- go test ./...
mise exec -- go test ./internal/mcp -run TestSearchLog     # 単体テスト指定
```

Go は `mise` 経由で管理（`mise use -g go@latest` でインストール済み）。`golangci-lint` / `goreleaser` も `.mise.toml` に固定されているので `mise install` で揃う。

## Lint / Format

`golangci-lint` に fmt/lint の両方を任せている（`go fmt` / `gofumpt` 単体は使わない）。Makefile 経由が基本：

```bash
make fmt        # golangci-lint fmt ./...
make lint       # golangci-lint run ./...
make vet        # go vet ./...
make check      # fmt + vet + lint + test を一括
```

コード変更後は `make check` を通してから commit する。設定は `.golangci.yml`。

## リリース

GoReleaser + GitHub Actions で配布。`v*` タグを push すると `.github/workflows/release.yml` が走り、`.goreleaser.yaml` の定義に従って linux/darwin × amd64/arm64 のバイナリと checksum が GitHub Releases に上がる。

- Windows はビルド対象外（`procman` が `syscall.Setpgid` / `syscall.Kill` に依存するため）
- バージョン情報は ldflags で `internal/cmd.{version,commit,date}` に注入され、`ws-dev version` で確認できる

ローカル検証：

```bash
make release-check      # .goreleaser.yaml の構文チェック
make release-snapshot   # タグなしで dist/ にクロスビルド（動作確認用）
make clean              # dist/ と ws-dev バイナリを削除
```

リリース手順：

```bash
# main が最新の状態で
git tag v0.1.0
git push origin v0.1.0
# → Actions が走って Releases にアーティファクトが上がる
```

別PCでの取得：

```bash
gh release download v0.1.0 -R hiyamamo/ws-dev -p 'ws-dev_*_linux_amd64.tar.gz'
tar xzf ws-dev_*_linux_amd64.tar.gz && sudo mv ws-dev /usr/local/bin/
```

## パッケージ構成と責務

| パッケージ | 責務 |
|------------|------|
| `internal/cmd` | cobra サブコマンド定義（init/clone/link/unlink/server/logs/run/mcp） |
| `internal/config` | `ws-dev.yml` のパース、`RepoName()` は URL から basename 抽出 |
| `internal/workspace` | `ws-dev.yml` 検出（cwd → 親へ遡上）、`RepoDir(label)` など |
| `internal/links` | `links/` → `repos/<repo>-<label>/` へ相対シムリンク作成。既存ディレクトリは置換 |
| `internal/tasks` | `exec_wrapper` を前置したコマンドを stdio 継承で実行 |
| `internal/procman` | Procfile 相当の並列プロセス管理。`{{.Label}}` 等を text/template で展開、各プロセスを setpgid で独立 pgid に配置、SIGTERM/SIGKILL でクリーンアップ |
| `internal/mcp` | stdio JSON-RPC の MCP サーバー実装（`list_logs` / `tail_log` / `truncate_log` / `search_log`） |

## 重要な設計ポイント

### サーバーは単一ラベル前提

`ws-dev server <label>` は `.ws-dev/server.pid` を読み、前回の ws-dev プロセスがまだ生きていれば `SIGTERM` で停止してから新プロセスを起動する。並列ラベル起動は想定していない（ポート衝突を避けるため）。

- `.ws-dev/server.pid` — 自身の PID
- `.ws-dev/current-label` — 直近ラベル（`ws-dev logs` が省略時に参照）

### ログディレクトリ解決順

コマンドラインフラグ `--log-dir` → 環境変数 `WS_DEV_LOG_DIR` → `ws-dev.yml` の `log_dir` → デフォルト `log`。

この順序は `server` / `logs` / `mcp` すべてで一貫している。`mcp` は `cwd` 基準で解決する（cwd は各クローンリポの中で実行される想定）。

### ディレクトリ命名

`repos/<repo-name>-<label>/`。`<repo-name>` は `repo:` URL の basename（`.git` 除去込み）から推定。ユーザーが渡す `<label>` は短い名前（例: `fix-login`）で、CLI 内部でプレフィックスを付加。

### プロセスの template 展開

`processes.<name>.cmd` は Go の `text/template` で評価される：

- `{{.Label}}` — ラベル
- `{{.PortBase}}` — `--port-base` / `$WS_DEV_PORT_BASE` / デフォルト 3000
- `{{.Workspace}}` — ワークスペース絶対パス

展開後にシェル的ホワイトスペース分割（`tasks.fields`）で argv 化する。クォート対応は最小限（`"..."` / `'...'` のみ、エスケープ非対応）。

## 手動検証のしかた

```bash
# サンプルワークスペースで一通り動かす
mkdir -p /tmp/ws-play && cd /tmp/ws-play
/path/to/ws-dev init sample && cd sample
# ws-dev.yml を編集（repo を実リポに、processes を埋める）
../ws-dev clone branch-a
../ws-dev link branch-a
../ws-dev server branch-a       # 別ターミナルで
../ws-dev logs                  # 直近ラベル
../ws-dev logs branch-a web -f  # follow
../ws-dev run branch-a console  # tasks.console
```

MCP 単体テスト：
```bash
cd repos/<repo>-branch-a
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}\n{"jsonrpc":"2.0","id":2,"method":"tools/list"}\n' | ws-dev mcp
```
