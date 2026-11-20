# Verify Restivra's own metadata backup

Create a completed PostgreSQL custom archive with the existing metadata backup
script, then check it without replacing the running installation:

```powershell
New-Item -ItemType Directory work -Force | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
./scripts/metadata-backup.ps1 -OutputPath "work/state-$stamp.dump"
./scripts/metadata-check.ps1 -Archive "work/state-$stamp.dump" -ReportPath "work/state-$stamp.json"
```

Requires PowerShell 7.2+, a **local** Docker Linux engine and a previously pulled
`postgres:17-bookworm` image. The checker can run while Restivra is running, or
with the app stopped. It reads the supplied archive, not the existing API,
database or `.env` file. `ReportPath` is optional; JSON is also printed to stdout.
Use unused output names: existing reports are never overwritten.
Relative archive/report paths follow the current PowerShell location, including
after `Set-Location`; they do not use the process's original startup directory.

The tool hashes the completed input, restores it into temporary PostgreSQL,
checks the expected metadata tables, installation identity and job/attempt relationships, and reports counts
of projects, jobs, reports, attempts and active jobs. It then checks the source
hash again and removes the temporary database. Exit 0 requires successful
restore, schema checks, cleanup and any requested report write. On a nonzero exit,
inspect the console result; do not treat an incomplete saved report as acceptance.

The temporary database uses the resolved local image identity, no network,
published port, host bind or Docker socket, a read-only root filesystem,
RAM-backed PostgreSQL data, a non-root user and explicit CPU/memory/PID limits.
The archive is streamed over `docker exec` stdin. A per-run ownership label is
checked before cleanup. **No Restivra worker is started against the restored
installation ID.** Pending jobs remain data in the temporary database.

Remote TCP/SSH Docker contexts and remote `DOCKER_HOST` values are rejected before
archive transfer. Context and host overrides must both identify local Unix
sockets or local Windows named pipes. Choose your local Linux context first.

Defaults: 256 MiB restored data, 768 MiB container memory, one CPU, 128 PIDs and a
180-second operation budget. For larger metadata, review capacity and adjust:

```powershell
./scripts/metadata-check.ps1 -Archive work/state.dump -DataMB 512 -MemoryMB 1024 -TimeoutSeconds 300
```

Memory must exceed data capacity by at least 256 MiB. The maximum input is 10 GiB.
Docker commands and readiness checks are bounded; cleanup has a separate budget.
`cleanup_failed` leaves a failed result and the exact temporary container name
for inspection/removal once Docker is responsive. It does not authorize deleting
other containers or volumes. Stderr is discarded; native stdout capture is capped.

This checks a logical metadata restore and expected tables/counts. It does not
run migrations, restart the API/worker, reproduce original owners/grants, validate
the source database archives, or recover `.env` and `private/sources.json`.
Keep those files and source backups separately. Actual application-state recovery
still follows the [operations runbook](PILOT-RUNBOOK.md#back-up-restivras-own-state).
Do not launch two workers with a cloned metadata ownership ID on the same engine.

## Results

| Code | Action |
| --- | --- |
| `archive_unavailable` / `archive_invalid` | Use a readable, completed metadata custom archive |
| `engine_not_local` / `engine_unavailable` | Select/start the local Docker Linux engine |
| `restore_image_missing` | Pull `postgres:17-bookworm` first |
| `database_start_failed` / `check_timeout` | Inspect Docker availability and capacity before retrying |
| `metadata_restore_failed` | Inspect archive integrity, PostgreSQL version and restored capacity |
| `metadata_schema_invalid` | Use Restivra's metadata backup, not an application/customer dump |
| `archive_changed` | Produce a new completed, stable metadata file |
| `report_path_unavailable` / `report_write_failed` | Choose an unused, writable report destination |
| `cleanup_failed` | Inspect the exact named temporary resource before proceeding |

The checker has real PostgreSQL acceptance cases for valid metadata, an
application archive, corrupt input, invalid headers, report non-overwrite and
remote-engine overrides. Native-process tests cover bounded output, discarded
private stderr, timeout and sanitized startup failure. CI runs these alongside
the fresh pilot setup and smoke checks.
