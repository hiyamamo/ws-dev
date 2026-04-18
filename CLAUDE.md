# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## リポジトリ概要

このリポジトリは **ws-dev** ワークスペース。複数のリポジトリを並列に動かすための開発環境ラッパー。

- `repos/<number>/` に各作業ブランチのクローンを配置
- `storage/` と `.envrc` はシンボリックリンクで各クローンと共有
- `mcp/ws-dev/` は各クローンの `log/` ディレクトリを操作する MCP サーバー

## よく使うコマンド（just）

```bash
# 新しいワークスペース番号でクローン＆セットアップ
just clone <number>

# 開発サーバー起動（Rails + Resque + pnpm dev を foreman で同時起動）
just dev <number>

# Rails コンソール
just rails-console <number>

# bundle install / pnpm install
just bundle-install <number>
just pnpm-install <number>

# ログをリアルタイム追跡
just log-web <number>
just log-worker <number>
```

## アーキテクチャ

### repos/ 配下のワークスペース構成

`just clone <number>` で以下が自動構成される：

1. `<REPO_URL>` からクローン → `repos/<number>/`
2. シンボリックリンク設定：
   - `.envrc` → `../../.envrc`（direnv 設定を共有）
   - `storage/` → `../../storage/`（ActiveStorage ファイルを共有）
   - `.claude/settings.local.json` → `../../../settings.local.json`（Claude Code 設定を共有）
3. MCP サーバーをローカルスコープで登録（`claude mcp add ws-dev`）

### Procfile と foreman

`just dev <number>` は `Procfile` を `sed` で加工して `/tmp/Procfile-inv-<number>` を生成し、プロセス名に番号を付けて foreman で起動する：

- `web-<number>`: Rails サーバー（port 3015、ログは `log/web.log` に tee）
- `worker-<number>`: Resque ワーカー（ログは `log/worker.log`）
- `build-<number>`: pnpm dev（フロントエンドビルド）

### MCP サーバー（ws-dev）

`mcp/ws-dev/src/index.js` — Node.js + `@modelcontextprotocol/sdk` で実装。**起動ディレクトリ（各 `repos/<number>/`）の `log/` を操作する。**

提供ツール：

| ツール | 説明 |
|--------|------|
| `list_logs` | `log/*.log` の一覧（name, size, mtime） |
| `tail_log` | 末尾 N 行を取得 |
| `truncate_log` | ログを 0 バイトに切り詰め |
| `search_log` | regex で検索（streaming、context 行対応） |

MCP サーバーは各クローンのルートで `node ../../mcp/ws-dev/src/index.js` として起動するため、`process.cwd()` が `log/` の解決に使われる。

### mise + direnv

各クローンは `direnv exec` と `mise exec` でツールバージョンを管理している（Ruby/Node のバージョンは `repos/<number>/` 側の設定に従う）。
