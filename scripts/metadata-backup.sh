#!/bin/sh
# Export Restivra's own project/job database. Customer archives are separate.
set -eu
cd "$(dirname "$0")/.."
if [ "$#" -ne 1 ]; then printf 'Usage: sh scripts/metadata-backup.sh /private/restivra-YYYYMMDD.dump\n' >&2; exit 2; fi
output=$1
umask 077
# Refuse to overwrite an existing path, including a symlink. Keep failed output
# under an unmistakable .partial name; never report it as a completed archive.
partial="${output}.partial"
if [ -e "$output" ] || [ -L "$output" ]; then printf 'Output already exists.\n' >&2; exit 2; fi
(set -C; docker compose exec -T db pg_dump -U restivra -d restivra -Fc > "$partial")
# Hard-link creation is exclusive, unlike a replacing rename. Both paths must
# be on the same filesystem (they share a directory).
ln "$partial" "$output"
rm -- "$partial"
printf 'Metadata archive created. Keep it private; test its restore separately.\n'
