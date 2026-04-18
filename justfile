dir number:
  @echo repos/{{number}}

dev number:
  direnv exec repos/{{number}} mise exec --cd repos/{{number}} -- bash -c 'mkdir -p log && sed "s/^web:/web-{{number}}:/; s/^worker:/worker-{{number}}:/; s/^build:/build-{{number}}:/" ../../Procfile > /tmp/Procfile-inv-{{number}} && foreman start --root . --procfile /tmp/Procfile-inv-{{number}}'

log-web number:
  less +F repos/{{number}}/log/web.log

log-worker number:
  less +F repos/{{number}}/log/worker.log

bundle-install number:
  direnv exec repos/{{number}} mise exec --cd repos/{{number}} -- bundle install

pnpm-install number:
  direnv exec repos/{{number}} mise exec --cd repos/{{number}} -- pnpm install

rails-console number:
  direnv exec repos/{{number}} mise exec --cd repos/{{number}} -- bundle exec rails console

rails number *args:
  direnv exec repos/{{number}} mise exec --cd repos/{{number}} -- bundle exec rails {{args}}

clone number:
  git clone <REPO_URL> repos/{{number}}
  just setup-links {{number}}

setup-links number:
  ln -sf ../../.envrc repos/{{number}}/.envrc
  rm -rf repos/{{number}}/storage
  ln -s ../../storage repos/{{number}}/storage
  git -C repos/{{number}} update-index --assume-unchanged storage/.keep
  grep -qxF 'storage' repos/{{number}}/.git/info/exclude || echo 'storage' >> repos/{{number}}/.git/info/exclude
  mkdir -p repos/{{number}}/.claude
  ln -sf ../../../settings.local.json repos/{{number}}/.claude/settings.local.json
  cd repos/{{number}} && claude mcp add ws-dev --scope local -- "$(mise which node)" "$PWD/../../mcp/ws-dev/src/index.js"
