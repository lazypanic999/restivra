# First-user pilot runbook

Participant handoff: [English quick start](PILOT-START.md),
[full Turkish guide](PILOT-START.tr.md), [seven-day plan](PILOT-PLAN.tr.md)
and [private record template](PILOT-RECORD.tr.md). These use a pinned pilot tag
and a Docker-only smoke script; this runbook contains deeper operating details.

First pilot: a developer on their own computer, with Docker Desktop using Linux
containers and PowerShell 7.2+ on Windows. Native Linux Docker is an alternative.
Use trusted PostgreSQL 17 custom archives created by pg_dump 17, accessed over
localhost. This pilot does not provide a hosted multi-tenant
service, backup creation, offsite storage, original-role recovery or PITR.

## Install and check prerequisites

Follow the README quick start. A new setup creates independent random database
and UI credentials in `.env`, plus a unique instance name. Keep `.env` private
and backed up separately from Git. Running setup again preserves it.

Windows quick start (PowerShell 7.2+, not the older Windows PowerShell 5.1):

```powershell
git clone https://github.com/lazypanic999/restivra.git
cd restivra
./scripts/setup.ps1
docker pull postgres:17-bookworm
docker compose up --build -d
```

Start Docker Desktop first and select Linux containers. The same desktop install
has been exercised on Windows with real PostgreSQL restores. If Docker is closed,
the computer sleeps or shuts down, tests cannot run. On return, an overdue daily
schedule queues one run rather than a run for every missed day. Enable scheduling
only if this best-effort desktop behavior fits the pilot's expectations.

```sh
docker compose ps
docker compose exec worker restivra doctor --role worker
docker compose exec api restivra healthcheck
docker compose logs --tail=100 api worker
```

`doctor` checks the Linux engine, locally available restore image, backup-root
access, snapshot-directory write/delete access and metadata installation schema.
It does not perform a restore. The worker deliberately holds no UI access token;
its authentication check is skipped. `/healthz` tests API/database readiness,
without exposing project data. It does not certify worker availability: confirm
**Worker online** after signing in, or use authenticated `GET /api/health`.

The API has no Docker socket or archive mount. The worker's Docker socket grants
host-administrative power; run it on a host controlled by the operator. Keep
enough RAM for the control database and the configured restore memory limit.

## Sign in and remote access

Open `http://127.0.0.1:7474` and enter `RESTIVRA_AUTH_TOKEN` from `.env`.
The browser stores an eight-hour HttpOnly, SameSite=Strict signed session cookie;
the root token is not stored in localStorage. English is the default. The header
offers Turkish and preserves an explicit selection on that origin.

For a remote host:

```sh
ssh -L 7474:127.0.0.1:7474 user@your-server
```

Keep the Compose loopback binding. Public reverse-proxy deployment, TLS/proxy
trust configuration, separate user accounts, audit trails and tenant isolation
are outside this tested pilot profile. Treat everyone with the shared token as
an operator able to edit checks, schedule and cancel jobs.

Legacy installations are preserved: an existing `.env` without
`RESTIVRA_AUTH_TOKEN` stays unauthenticated, and the panel says so. Before sharing
access, generate a random 32-byte token privately, set `RESTIVRA_AUTH_TOKEN` in
`.env`, and run `docker compose up -d --force-recreate api`. Do not use the
placeholder from `.env.example`. Avoid changing `POSTGRES_PASSWORD` on an
initialized volume: that environment variable does not rotate PostgreSQL's
existing password.

To revoke all sessions, replace the access token with a new independent random
value and recreate the API. Sign out deletes the current browser cookie;
previously copied session cookies remain valid until expiry or root-token
rotation. Login is limited to ten attempts per actual peer IP per minute.
Reverse-proxy headers are not trusted for peer identity or secure-cookie state.

To run multiple installations on one host, use different Compose project names,
ports and backup directories. Set `RESTIVRA_PORT` for the host port; explicit
extra hostnames go in `RESTIVRA_ALLOWED_HOSTS`. Keep the generated instance name
stable. Each metadata database has its own resource-ownership ID. A copied
metadata database keeps that ID: **never run the original and its restored clone
as concurrent workers against the same Docker engine**.

## Connect the first real archive

1. Have the existing backup producer write a PostgreSQL 17 custom dump to a
   temporary extension. Atomically rename it to `customer.dump` after success.
   Put the completed archive under the configured `backups/` directory.
2. Create a project with `customer.dump` or a relative pattern such as
   `customer/*.dump`. With several matches, newest file mtime wins; filename
   order breaks ties. Symlinks escaping the backup root are rejected.
3. Add a check specific to expected application data, for example
   `SELECT count(*) > 0 FROM public.customers`. A check must return exactly one
   non-null boolean. It runs read-only with a ten-second statement timeout.
4. Run manually. Inspect restore, every SQL check, cleanup, SHA-256 and notes.
   Download the JSON report. Passing without custom checks proves less than
   passing checks designed around application invariants.
5. Make a second project/check return `SELECT false` once. Verify it fails, then
   archive that synthetic test project. If using a webhook, verify delivery with
   your actual receiver and adapter before relying on alerts.
6. Choose a time when the computer is usually running, with an explicit IANA
   time zone. Confirm a real
   scheduled run; missed days do not generate a backlog. Panel timestamps use
   the browser's time zone; project scheduling uses the configured zone.

Default limits: 256 MiB archive, 512 MiB RAM-backed restored data, 1536 MiB
container memory, a 180-second verification budget plus bounded cleanup, one CPU,
128 PIDs. Each automatic attempt has its own verification budget; backoff and
reconciliation add elapsed time. The database volume counts
toward container memory. A small compressed dump can expand far beyond 512 MiB.
Raise `data_mb` and `memory_mb` together, with at least 256 MiB headroom, after
checking host capacity. One private archive snapshot also needs local disk.

Only the standard PostgreSQL image is supported. An archive requiring additional
extensions can fail. A pass does not establish backup recency, grants, ownership,
roles, RLS access, full application startup or point-in-time recovery. Old mtime
and unchanged SHA-256 notes describe observations; inspect the backup producer
separately. Never feed an untrusted archive to the restore administrator.

## Operate and recover

```sh
docker compose logs --tail=100 api worker
docker compose restart worker
```

An abrupt worker exit preserves an `interrupted` attempt after reconciliation.
The job can then retry automatically; only a fresh successful restore can pass.
Startup removes containers carrying this database's installation
label and exact legacy running-job containers, then known private snapshots.
Another installation sharing the same human-readable instance name is excluded.
An unremovable resource stops startup or the worker before another job can run.

Transient infrastructure failures get at most three attempts with persisted
30/60-second retry delays. The report shows previous attempts and their JSON
downloads. A queued retry has no current result; a recovered pass does not erase
the first failure. Cancellation suppresses further retries. The third failure is
final, so repair its cause before starting a new manual verification.

For `cleanup_failed`, the worker stops and Compose restarts it for owned-resource
reconciliation. When Docker becomes responsive again and cleanup succeeds, an
eligible queued retry runs automatically. If cleanup remains unknown, the worker
stays unavailable and starts no restores. Repair Docker/work-directory access;
inspect logs and confirm worker health. A manual restart is a recovery tool when
infrastructure is repaired, not a required step after every brief outage.

A missing archive fails before restore and can have no full report.
For `restore_failed`, inspect archive compatibility, extensions and restored-data
capacity. PostgreSQL may close its connection without reporting the original
disk/memory cause, so that code intentionally remains broad. Raw row-bearing
PostgreSQL errors are never retained.

Bad archives, failed SQL checks, capacity failures and restore-stage timeouts are
not retried. Failure notifications wait for the final verification outcome;
their delivery attempts are a separate counter. Safe slow/failed Docker timings
in JSON reports help distinguish engine/start/cleanup delays without retaining
SQL or command arguments. See [stall analysis](DOCKER-STALL-ANALYSIS.md).

`docker compose down` preserves named volumes. `down -v` deletes metadata and
history; do not use it to restart a real installation. Completed jobs expire
after `RESTIVRA_RETENTION_DAYS` (30 by default); pending notifications are kept.
The overview lists six recent jobs and counts all retained outcomes. Runs searches
all retained jobs, with 50-row pages and filtered CSV exports up to 10,000 rows.
Projects lists up to 500 entries per active/archived view. Restoring an archived
source preserves its settings/history and disables daily scheduling. See
[history and recovery](HISTORY.md).

## Back up Restivra's own state

These scripts export the **project/job metadata database**, not customer dumps
or `.env`. Use private encrypted storage and a separate off-host copy according
to your own retention policy. Parent directories must already exist.

```sh
sh scripts/metadata-backup.sh /private/restivra-20261118.dump
```

PowerShell 7.2+:

```powershell
./scripts/metadata-backup.ps1 -OutputPath work/restivra-20261118.dump
```

Existing output is never replaced. A failed export remains `.partial` and is not
a completed archive. Export can run while the application operates; PostgreSQL
provides a consistent logical snapshot. Protect the archive: it contains project
names, SQL checks and reports. Snapshots are deleted, not securely erased.

For a restore drill, use a **new isolated PostgreSQL 17 database** and
`pg_restore --no-owner --no-privileges --exit-on-error`. Check project/job/report
counts, relationships and the `installation` row. Do not start a worker from a
clone while the original runs. The engineering acceptance drill restored both
Windows- and Linux-generated metadata archives into separate databases and
confirmed intact relationships. Exports also include `job_attempts`; check that
attempt histories survive and have no orphan job references.

For actual host replacement: stop the original API/worker; preserve its volumes
and files; provision a fresh isolated installation; stop the new API/worker;
restore metadata into its empty application database; restore private `.env` and
customer archives separately; start the replacement; confirm installation
identity, reconciliation, health and a manual restore. Keep the original stopped
until deciding whether to roll back. Do not rehearse destructive restore commands
against the only copy of your metadata.

## One-week pilot acceptance

Keep the existing backup process and independently tested recovery procedure.
Record seven days of scheduled outcomes, changed/unchanged fingerprints, runtime,
cleanup, host memory and notification delivery. Test one worker restart and a
metadata restore drill. Ask the operator to complete installation and first
verification from this guide without developer intervention. A successful local
soak is evidence for starting this pilot, not a substitute for it. The current
review includes a passing two-hour fixed-image desktop run and separate automatic
fault recovery. Begin each new installation with manual checks and confirm an
actual schedule on that operator's host before leaving it unattended. The exact
Windows trigger behind the earlier stall remains unproven. See
[readiness results](PILOT-READINESS.md).

Webhook delivery can be repeated if delivery succeeds but the metadata update is interrupted. Receivers should deduplicate by job_id. A successful verification with no configured webhook does not imply alert delivery was tested.
