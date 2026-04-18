# Rails example (legacy)

Previous Procfile + justfile combination that ws-dev replaces.
Kept here for reference when writing a `ws-dev.yml` for a Rails project.

## Equivalent `ws-dev.yml`

```yaml
repo: <REPO_URL>
log_dir: log
exec_wrapper: ["direnv", "exec", ".", "mise", "exec", "--"]

processes:
  web:
    cmd: "bundle exec rails s -b 0.0.0.0 -p {{.PortBase}}"
  worker:
    cmd: "bin/rails resque:work"
    env:
      QUEUE: "*"
  build:
    cmd: "pnpm dev"

tasks:
  console: "bundle exec rails console"
  rails: "bundle exec rails"
  bundle-install: "bundle install"
  pnpm-install: "pnpm install"

links:
  - .envrc
  - .claude/settings.local.json
  - storage
```
