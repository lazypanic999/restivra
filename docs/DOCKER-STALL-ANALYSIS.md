# Docker stall and automatic recovery — 2026-11-18

The original restore failed because Docker control operations stopped meeting
their deadlines. Restivra then treated that transient infrastructure incident as
a terminal job failure. That application recovery gap is fixed: a verified
cleanup precedes a new bounded attempt, with the old failure retained as evidence.
The exact Windows process or kernel condition that triggered the desktop stall
has **not** been established. No Windows service was disabled and no global Docker
setting was changed as a purported fix.

## Observed incident

All timestamps below are UTC, on the same host. The original job was
`236248bcce857381b606f0cdf7042843`.

| Evidence | Time / duration |
| --- | --- |
| Restore container create, VM daemon | 23:02:33.636–23:02:37.142 |
| Desktop backend `GET /time` | 23:02:44.835–23:03:09.967, **25.132 s** |
| Restore container start, VM daemon | 23:03:10.088–23:03:15.965 |
| Control-database checkpoint | About 23:03:46–23:04:12, **25.7 s** |
| Container DELETE, VM daemon | 23:03:56.510–23:04:09.122 |
| Old job terminal failure | 23:04:03, stage `start`, `cleanup_failed` |

Docker Desktop was 4.77.0.228796, Engine 29.5.3 / API 1.54, WSL 2.6.3.0,
kernel 6.6.87.2. Available Docker memory was about 15.49 GiB, with 16 CPUs.
The Desktop backend's delayed time RPC is independent of restored SQL. Together
with the delay between create and start, it places the observed problem in the
host/Desktop control path. The overlapping database checkpoint is another
observation of slow host work; it does not prove PostgreSQL caused the stall.

Windows recorded unrelated service lifecycle events near the same window.
Their timing is a correlation, not sufficient evidence to attribute causation.
The reviewed event window contained no matching host sleep or storage error.
Absence of those events does not exclude every host scheduling or I/O problem.
Raw Desktop logs and private host details are not published; the fixed operation,
UTC timing and duration are the relevant diagnostic evidence. Docker documents
the [Desktop daemon log locations](https://docs.docker.com/engine/daemon/logs/).

The previous 97.4-minute failed acceptance run remains in
[historical evidence](acceptance/pilot-20261118/summary.json). It is not relabeled
as a passing run after recovery changes.

## Corrected failure propagation

- Jobs keep one ID and immutable run settings across at most **three started
  attempts**. Retry deadlines persist in PostgreSQL: 30 seconds after attempt
  one, 60 seconds after attempt two. A process restart does not reset the budget.
- Only interrupted work, unavailable engine, database startup failure, unconfirmed
  cleanup, and engine/image/start timeouts qualify. Failed SQL checks, invalid or
  missing dumps, capacity failure and restore-stage timeouts remain final.
- Completion, attempt history and retry scheduling commit in one transaction.
  A queued retry has no current passing report. Previous reports remain separately
  downloadable; an eventual pass describes a fresh successful restore.
- A worker holds the database's advisory lock, reconciles owned containers and
  snapshots at startup, and checks the exact job container again before a retry.
  Unknown cleanup stops execution before another restore can start. Compose's
  restart policy allows another reconciliation when Docker recovers.
- Cancellation suppresses retries. Failure webhooks are queued after the final
  failed attempt, not once per temporary retry. Delivery has its own bounded
  retry policy and is distinct from verification attempts.
- Docker subprocess waits are bounded even when an inherited output pipe stays
  open. Slow/failed command timings record only a fixed operation name, elapsed
  milliseconds and safe outcome: no SQL, command arguments or raw stderr.

An engine or host that remains unavailable cannot be repaired by the application.
It can keep retrying startup reconciliation, but cannot certify cleanup or run
new work until infrastructure is accessible. A permanently unremovable resource
requires operator repair; the panel reports the worker offline.

## Fault acceptance

The dedicated `restivra-pilot-chaos` stack used its own database, volumes, port
and synthetic 135,500-row commerce dump. A test-only Unix-socket proxy, on an
unpublished isolated Docker network, delayed start/removal/list operations for
35 seconds. Production Compose has no proxy and no Docker TCP listener.

| Injected fault | Preserved first attempt | Automatic result |
| --- | --- | --- |
| Docker start/removal/list delayed 35 s, job timeout 10 s | `failed`, `cleanup_failed`; timing records show start/rm/ps deadlines | Attempt 2 passed restore, 100,000-item check and cleanup |
| Metadata database paused 25 s during active work | `interrupted`, connection/liveness loss | Worker reconciled; attempt 2 passed restore, check and cleanup |

Neither scenario submitted a new manual verification or pressed a Retry button.
Each finished with no remaining job container. A metadata export was restored
into an isolated PostgreSQL 17 database: two jobs, four attempt records, two
failed/interrupted attempts, two passing attempts and zero orphan attempts.
English and Turkish report histories and the previous failed JSON download were
checked in the browser with no console errors or warnings.

The two-hour normal-engine soak is recorded separately from these controlled
faults. Its final counts and runtime image identity belong in the accompanying
acceptance evidence; a fault-proxy success alone is not a long-duration pass.

## Reproduce in a disposable environment

Use PowerShell 7.2+ and a source copy outside the regular installation. Generate
fresh `.env` secrets with `scripts/setup.ps1`, set `RESTIVRA_PORT=19474`, and place
the synthetic commerce fixture in that copy's `backups/commerce.dump` (see
[relational acceptance](POSTGRES-ACCEPTANCE.md)). Use the exact test project name
`restivra-pilot-chaos`, never the normal `restivra` project.

1. Build `testdata/faultproxy/main.go` for Linux with `CGO_ENABLED=0`; place the
   executable outside version control. Build the Restivra application image.
2. Start the isolated Compose database/API using the copied Compose file.
   Start a container named `restivra-fault-proxy` on
   `restivra-pilot-chaos_control`, alias `fault-proxy`, with the executable mounted
   as `/fault-proxy` and the Unix Docker socket mounted at its usual path. Use
   Debian bookworm-slim, no published ports, read-only root, dropped capabilities
   and `no-new-privileges`. The proxy has Docker administrative power; keep this
   helper confined to the disposable test, then stop it.
3. In a Compose override set only the test worker's
   `DOCKER_HOST=tcp://fault-proxy:2375`. Start that worker with the override.
4. Run `scripts/unattended-chaos.ps1 -ProjectDirectory <copy>`. The script verifies
   the database's project/directory, worker override and unpublished proxy network
   before pausing anything. It writes synthetic attempt evidence under `work/`.
5. Stop the proxy and dedicated Compose project. Preserve the regular
   installation and its volumes. Remove no user volumes during a restart.

For normal-engine duration acceptance use `scripts/pilot-soak.ps1` in another
fresh `restivra-pilot-*` project. Its expected faults include worker SIGKILL,
metadata restart, an eight-second metadata pause, cancellation and intentionally
bad checks/archives. It also checks a real wall-clock schedule. Count automatic
attempts separately from final job outcomes.
