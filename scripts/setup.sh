#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ ! -e .env ]; then
  umask 077
  password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  printf 'POSTGRES_PASSWORD=%s\nRESTIVRA_INSTANCE=restivra-main\nRESTIVRA_RETENTION_DAYS=30\nRESTIVRA_WEBHOOK_URL=\n' "$password" > .env
  printf 'Generated private .env\n'
fi
mkdir -p backups
printf 'Next: docker pull postgres:17-bookworm && docker compose up --build -d\n'
