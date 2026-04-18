web: bundle exec rails s -b 0.0.0.0 -p 3015 2>&1 | tee -a log/web.log
worker: QUEUE=* bin/rails resque:work >> log/worker.log 2>&1
build: pnpm dev
