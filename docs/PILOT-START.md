# Developer pilot handoff

Pinned candidate: **`v0.1.0-pilot.5`**. The primary tested path is Windows,
PowerShell 7.2+ (`pwsh`), Git and Docker Desktop with Linux/amd64 containers.
Go, Node and a host PostgreSQL installation are not required: the installer pulls a published, digest-pinned image.

[Full Turkish participant guide](PILOT-START.tr.md) ·
[Seven-day operator plan](PILOT-PLAN.tr.md) · [Private record template](PILOT-RECORD.tr.md)

```powershell
git clone --branch v0.1.0-pilot.5 --depth 1 https://github.com/lazypanic999/restivra.git restivra-pilot
cd restivra-pilot
./scripts/install.ps1
./scripts/pilot-check.ps1
./scripts/pilot-smoke.ps1
```

Start Docker first. Open `http://127.0.0.1:7474` and sign in using the value of
`RESTIVRA_AUTH_TOKEN` from the private `.env` file. Never share that file.
English is the default; Turkish is available in the header.

The smoke script creates a real PostgreSQL 17 archive containing two synthetic
customers and verifies it through the API/worker. One project must pass; the
other intentionally fails `SELECT false`, while restore and cleanup succeed.
Schedules stay disabled. Inspect both reports, then archive the sample projects.
The source container is removed; sample dumps and private `work/pilot-…` evidence
remain locally. Re-running creates new samples, without overwriting existing ones.
If it fails, inspect/wait for or cancel its jobs before trying again.

Then use your own completed, trusted PostgreSQL 17 custom-format (`pg_dump 17 -Fc`)
archive under `backups/`. Create a project using its relative path, add meaningful
single-boolean SQL checks without semicolons, pass a manual run and confirm an
actual daily scheduled run. Preserve your existing backup/recovery process.

Default limits: 256 MiB archive, 512 MiB RAM-backed restored data, 1536 MiB restore
container memory and 180-second verification budget. Allow additional memory
for metadata PostgreSQL and disk for images plus a private archive copy.
The data limit also includes PostgreSQL system files, indexes and WAL. Compressed
archive size is not a reliable capacity estimate. A synthetic 59 MB archive
required a larger data limit in acceptance; this is an example, not a sizing rule.
Extensions, large databases and different major versions need prior compatibility
review. This pilot is for trusted local inputs, not public uploads.

If port 7474 is occupied, set `$env:RESTIVRA_PORT='17474'` before the first install.
Setup saves that port and a unique Compose project name in the new `.env`.
The installer prints the actual URL; pilot scripts read the saved port. For an
already generated `.env`, edit its `RESTIVRA_PORT` before retrying installation.
For a second installation on the same engine use a separate
checkout, port, freshly generated `.env` and a pinned release image; follow
[the second-installation steps](SECOND-INSTALL.md) before starting it. Never duplicate
metadata ownership.

Keep this version fixed for the seven-day pilot. Record planned versus actual
runs, host availability, duration, cleanup, interventions and usefulness. Sleeping
computers cannot run tests; on return only one overdue daily run is queued.
No email/Telegram alert is configured by default. Test a real optional HTTPS
receiver separately before relying on notifications.

Stop with `docker compose stop`; resume with `docker compose start`, then run the
prerequisite check. `docker compose down` preserves volumes; **do not use `down -v`**.
Back up metadata, `.env`, `private/sources.json` and source archives separately before
any update. [Verify a metadata backup](METADATA-CHECK.md) in an isolated temporary
database before relying on it.
Failures need sanitized diagnostics, never customer dumps or secrets.

Linux/amd64 with PowerShell 7.2+ can use the same installer. Source builds use
`sh scripts/setup.sh` and `docker compose -f compose.yaml -f compose.build.yaml up --build -d`. The pilot
scripts need PowerShell 7.2+; without it, follow the manual archive procedure in
the [operations runbook](PILOT-RUNBOOK.md). macOS/ARM acceptance is not established.

The [readiness review](PILOT-READINESS.md) contains engineering evidence.
Seven-day external-user acceptance and willingness to pay remain unvalidated until
an actual participant completes them.

## Optional features after the first successful restore

- [Read-only S3-compatible archives](S3-SOURCES.md), configured privately in the worker.
- [Editable SQL check templates](CHECK-TEMPLATES.md) for application-specific rules.
- [Previous-run comparison](RUN-COMPARISON.md) with changed definitions called out.
- [Printable English/Turkish evidence](EVIDENCE-REPORTS.md), including optional name redaction.
- [Project settings export/import](PROJECT-TRANSFER.md); imported schedules start disabled.

These are optional; the local two-row smoke check above remains the starting path.

For a planned release update and rollback, follow [the update guide](UPDATING.md).
