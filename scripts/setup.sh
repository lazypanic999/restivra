#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ ! -e .env ]; then
  umask 077
  password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  instance=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
  auth_token=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  (set -C; printf 'POSTGRES_PASSWORD=%s\nRESTIVRA_AUTH_TOKEN=%s\nRESTIVRA_INSTANCE=restivra-%s\nRESTIVRA_RETENTION_DAYS=30\nRESTIVRA_WEBHOOK_URL=\n' "$password" "$auth_token" "$instance" > .env)
  printf 'Generated private .env\n'
  printf 'Sign in with RESTIVRA_AUTH_TOKEN from your private .env file.\n'
fi
mkdir -p backups
umask 077
mkdir -p private
if [ ! -e private/sources.json ]; then
  (set -C; printf '{"s3":[]}\n' > private/sources.json)
fi
printf 'Next: docker pull postgres:17-bookworm && docker compose up --build -d\n'
