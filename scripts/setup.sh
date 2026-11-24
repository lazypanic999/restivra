#!/bin/sh
set -eu
cd "$(dirname "$0")/.."
if [ ! -e .env ]; then
  umask 077
  password=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  instance=$(od -An -N6 -tx1 /dev/urandom | tr -d ' \n')
  compose_project=${COMPOSE_PROJECT_NAME:-restivra-$instance}
  port=${RESTIVRA_PORT:-7474}
  case "$compose_project" in ''|[!a-z0-9]*|*[!a-z0-9_-]*) printf 'Invalid COMPOSE_PROJECT_NAME.\n' >&2; exit 1;; esac
  [ "${#compose_project}" -le 63 ] || { printf 'COMPOSE_PROJECT_NAME is too long.\n' >&2; exit 1; }
  case "$port" in ''|*[!0-9]*) printf 'Invalid RESTIVRA_PORT.\n' >&2; exit 1;; esac
  [ "${#port}" -le 5 ] && [ "$port" -ge 1024 ] && [ "$port" -le 65535 ] || { printf 'RESTIVRA_PORT must be 1024-65535.\n' >&2; exit 1; }
  auth_token=$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')
  (set -C; printf 'POSTGRES_PASSWORD=%s\nRESTIVRA_AUTH_TOKEN=%s\nRESTIVRA_INSTANCE=restivra-%s\nCOMPOSE_PROJECT_NAME=%s\nRESTIVRA_PORT=%s\nRESTIVRA_RETENTION_DAYS=30\nRESTIVRA_WEBHOOK_URL=\n' "$password" "$auth_token" "$instance" "$compose_project" "$port" > .env)
  printf 'Generated private .env\n'
  printf 'Sign in with RESTIVRA_AUTH_TOKEN from your private .env file.\n'
fi
mkdir -p backups
umask 077
mkdir -p private
if [ ! -e private/sources.json ]; then
  (set -C; printf '{"s3":[]}\n' > private/sources.json)
fi
printf 'Next: docker pull postgres:17-bookworm && docker compose -f compose.yaml -f compose.build.yaml up --build -d\n'
