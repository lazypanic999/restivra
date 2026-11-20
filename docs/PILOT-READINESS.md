# Pilot readiness review — 2026-11-18

Scope: a developer verifying their own trusted PostgreSQL 17 archives on their
computer, with Docker Desktop Linux containers and localhost access. PowerShell
7.2+ on Windows is the primary handoff path; native Linux is an alternative.
A real customer pilot remains
the next product milestone. Public hosted SaaS readiness is a separate project.

## November 20 ready-image packaging

**`v0.1.0-pilot.3`** packages the same qualified engine as public Linux/amd64
release images. [Ready-image acceptance](acceptance/release-images-20261120/)
records a fresh GitHub-tag install, real PostgreSQL smoke, compatible update,
explicit rollback, queued-work refusal and automatic recovery from a real
startup failure. The operator installation retained all 17 projects and 24 jobs.
The participant guides now use this packaging release. See [updates/recovery](UPDATING.md).

## November 20 resumed pilot candidate

Candidate **`v0.1.0-pilot.2`** corrects the large-archive listing issue. The worker
drains remaining Docker stdin while retaining PostgreSQL failures. The
[current acceptance](acceptance/large-archive-fix-20261120/) records repeated
full restores of the original 59 MB reproduction archive, preserved failure
outcomes, cancellation, worker recovery and an actual scheduled run. Source CI
also tests repeated large listings, corrupt input and cancellation while draining.
This engine qualification is retained by pilot.3. The previous pilot tag
remains unchanged. This supports a local developer pilot within the documented
archive/version/resource limits; external seven-day acceptance is still pending.

## Historical November 20 stopped development checkpoint

The earlier [development checkpoint](acceptance/development-checkpoint-20261120/)
records completed tests separately from planned durations. Work stopped at the
operator's request. A separate large synthetic archive exposed an intermittent
Docker-streamed archive-listing failure; at that checkpoint its candidate fix
was not merged or installed. Large-archive acceptance and the next pilot release
were held pending correction and a full restore regression. The earlier
`v0.1.0-pilot.1` tag is unchanged. Fresh Windows installation and bounded metadata
export/checking passed; these do not resolve the archive-listing issue.

## Earlier November 20 operator update

The participant handoff now includes a [pinned installation guide](PILOT-START.md),
Docker-only synthetic smoke checks, a [seven-day plan](PILOT-PLAN.tr.md) and a
[private record form](PILOT-RECORD.tr.md). A fresh authenticated Windows clone
passed setup, credential preservation and the actual positive/negative restore
checks. See [handoff evidence](acceptance/pilot-handoff-20261120/).
External participant acceptance remains pending.

The current candidate adds [retained history, CSV and archived-project recovery](HISTORY.md).
Real PostgreSQL integration tests cover tied-time pagination beyond 100 records,
literal search, date bounds, export limits and stale-attempt completion fencing.
English/Turkish browser checks cover saved filters, five history pages, CSV,
project recovery and the 390-pixel layout. See [browser evidence](acceptance/history-20261120/history-browser.json).

A cancelled Docker create can finish after its caller stops waiting. The worker
now reconciles installation-owned resources before each run and every 30 idle
seconds. A live test removed a late container and snapshot without restarting,
while preserving another installation's container. See [cleanup evidence](acceptance/history-20261120/idle-reconciliation.json).

The two-hour result below belongs to runtime `5ef8f0a`; it is historical evidence,
not a two-hour acceptance of this newer candidate. External customer acceptance
and the unresolved Windows host trigger remain open.

The newer runtime completed a **20.01-minute / 22-iteration** fixed-image
regression: all **38 expected outcomes** matched (27 passed, nine intentional
failures and two cancellations). One planned worker SIGKILL preserved its
interrupted attempt and recovered; there were no other automatic retries.
A metadata restart, eight-second pause and one real scheduled run passed.
Container/image identities stayed fixed and final inspection found no active
jobs, owned restore containers or private snapshot entries.
See [verified results](acceptance/history-20261120/soak/verified-summary.json)
and [attempt evidence](acceptance/history-20261120/soak/enriched-runs.ndjson).
An earlier short development run was interrupted by an intentional API image
replacement and is excluded from this fixed-image result.

## Previous unattended local developer pilot acceptance

Runtime `5ef8f0a` completed a fresh **120.01-minute / 143-iteration** acceptance
run on this Windows/Docker Desktop host. All **210 asserted outcomes** matched:
151 passed, 45 intentionally failed checks/archives and 14 cancellations. Two
intentional worker SIGKILLs recovered through preserved interrupted attempts and
fresh successful restores. Two metadata restarts, an eight-second metadata pause
and a real wall-clock schedule also passed. No other automatic retries or manual
retry actions occurred. API/worker container IDs and image identities stayed
unchanged throughout. Final inspection found zero active jobs, temporary restore
containers and private snapshot entries.

See the [verified summary](acceptance/unattended-20261118/soak/verified-summary.json),
[attempt-enriched outcomes](acceptance/unattended-20261118/soak/enriched-runs.ndjson),
[runtime identity](acceptance/unattended-20261118/soak/runtime.json) and
[resource samples](acceptance/unattended-20261118/soak/resource-samples.ndjson).
Shorter candidates stopped for a test-harness image-recreation issue and a
cancelled-retry metadata correction are excluded from this two-hour result.

A separate release-candidate fault drill delayed Docker start/removal/list by
35 seconds, paused metadata for 25 seconds during a restore, and cancelled a
pending retry. Recovery required no manual retry; cancellation stayed final
beyond its original deadline. The later metadata export/restore preserved ten
synthetic jobs and eighteen attempts with no orphan records.
[Fault evidence](acceptance/unattended-20261118/release-chaos/summary.json) and
[stall analysis](DOCKER-STALL-ANALYSIS.md) distinguish the observed host delay,
the corrected application recovery gap and the still-unproven Windows trigger.

This supports an **unattended local developer pilot while the computer and Docker
are running**, using trusted PostgreSQL 17 archives within tested sizing. It is
not a seven-day external-user acceptance or a guarantee that Docker/Windows
cannot stall. The exact host trigger remains unresolved; Restivra now handles
the observed transient failure mode autonomously and preserves attempt evidence.

The existing localhost installation was exported before upgrade. All fourteen
original job statuses/reports retained the same aggregate fingerprint after
migration. A new real two-row restore passed with one preserved attempt and clean
resources; its synthetic project was archived. The local API/database are healthy
and the worker is online. Test stacks were removed without deleting user volumes.
See [local upgrade proof](acceptance/unattended-20261118/local-upgrade.json) and
[final cleanup](acceptance/unattended-20261118/final-cleanup.json).

## Fixed in this review

- Database-scoped installation identity replaces shared instance names as
  container ownership evidence. Legacy recovery uses exact running-job IDs.
  A real Docker regression test preserves a foreign installation's container.
- Cleanup failures persist a failed report and stop the worker before it can
  claim another job. A fault-injection test verifies stop, queued-job retention
  and successful recovery on restart.
- Fresh setup generates independent database/access secrets and a unique name;
  exclusive creation avoids replacing an existing `.env`.
- Access-token login protects API records and downloads. Eight-hour signed
  HttpOnly/SameSite cookies, CSRF/origin checks, bounded peer rate limiting,
  expiry, tamper rejection, token rotation and logout have automated coverage.
  English/Turkish login, wrong-token rejection and logout were checked in-browser.
- API readiness has a bounded database timeout and Compose healthcheck. A
  `doctor` command checks worker prerequisites without restoring customer data.
- Compose waits for PostgreSQL's final PID 1 process, not initdb's temporary
  server. A separate empty-volume install completed without API/worker startup
  errors after this startup-gate correction.
- Reports carry stable safe error codes and English/Turkish next actions.
  PostgreSQL stderr is classified in bounded memory, never persisted verbatim.
- The compiler and Docker/CI build moved to patched Go 1.27.4 after a reachable
  HTTP standard-library advisory was detected. The patched scan is clean;
  `govulncheck` now runs in CI alongside the npm audit.
- Heartbeat writes are a single transaction, with a 15-second budget and a
  30-second UI liveness window. Metadata requests have bounded contexts.
  Normal shutdown cancellation is distinguished from lost database connectivity.
- Project/latest-job listing uses one indexed database statement instead of an
  additional query for every project.
- Real capacity exhaustion is tested with about 137 MiB of uncompressed text
  restored into a 128 MiB RAM-backed database from a roughly 282 KB dump. Failure
  must be reported and cleaned, including when the server loses its connection
  before a specific diagnostic reaches pg_restore.
- Windows and POSIX metadata export tools refuse overwrites. Both generated
  archives were restored in isolated PostgreSQL databases: 8 project rows,
  14 job rows, 12 reports and no orphan jobs in the initial drill.
- [Pilot runbook](PILOT-RUNBOOK.md) covers setup, first checks, sizing, access,
  metadata backup, recovery and week-long user acceptance.

## Earlier acceptance attempts — historical evidence

The checks use synthetic commerce data, isolated metadata/volumes and a separate
loopback port. Existing local archive/history volumes are preserved. Linux CI
passed for the initial hardening and startup-gate commits. Windows integration,
Linux race checks, npm audit and source/packaged-binary Go vulnerability scans
also passed. The recovery-phase results below preserve the original test output
and distinguish product failures from a test-harness false negative.

An early candidate's soak stopped after 14.04 minutes and 26 asserted outcomes:
an unplanned interrupted restore coincided with image extraction/compilation and
a 19.4-second metadata checkpoint (9.2 seconds syncing). That candidate had a
five-second heartbeat budget. It failed safely and recovered, but the result was
not accepted as a successful soak. Heartbeat/write changes above address short
host I/O stalls; the final soak is restarted from a fresh installation and adds
an explicit eight-second database pause during an active verification. Loss of
the advisory-lock session continues to stop the worker rather than allowing
concurrent restore ownership.

The longer candidate then ran for **97.4 minutes**, asserting **169 expected
outcomes** (120 passed, 36 intentionally failed, 11 cancelled, two intentionally
interrupted). Its next routine restore unexpectedly failed during database
startup: the run lasted 90 seconds, cleanup could not be confirmed within its
budget, and the worker stopped before taking another job. Startup reconciliation
subsequently recovered; no restore container remained when inspected. This is a
**failed long-duration acceptance run**, not a clean two-hour soak.

A 25.7-second control-database checkpoint overlapped the incident. The exact
cause of the host/Docker delay is unresolved; available evidence does not prove
a sleep event or a database-restore defect. The incident report and resource
samples are preserved with the acceptance evidence.

Evidence: [long-run summary](acceptance/pilot-20261118/summary.json),
[unexpected failure](acceptance/pilot-20261118/unexpected-failure.json),
[individual asserted runs](acceptance/pilot-20261118/runs.ndjson),
[sampled resources](acceptance/pilot-20261118/resource-samples.ndjson),
[readiness during a stall](acceptance/pilot-20261118/readiness-stall.json), and
[135,500-row relational acceptance](acceptance/pilot-20261118/relational-acceptance.json).

The 97 controller-resource observations sampled API usage at 8.28–10.76 MiB,
worker usage at 3.77–15.95 MiB, and metadata PostgreSQL usage at 29.84–38.16 MiB.
These exclude the temporary restore database and do not establish the cause of
the Docker delay or prove absence of memory leaks.

Inspection identified a separate cleanup bug: removal and absence confirmation
shared the same context, so a timed-out removal left no time to confirm whether
Docker had actually removed the container. Commit `633d55e` gives confirmation
its own bounded context. A regression test simulates a removal deadline both
with the container absent and still present. Unknown cleanup remains a stop
condition. This corrects confirmation behavior; it does not establish the cause
of the earlier Docker stall. A 25-minute recovery-phase run follows this fix.

That phase completed **25.01 minutes / 30 iterations**, with **48 runtime
assertions** at their expected outcomes (34 passed, nine intentionally failed,
three cancelled, two intentionally interrupted). Its final scheduler assertion
initially failed because PowerShell passed Invoke-RestMethod's JSON array to
Where-Object as a single object. This was reproduced: direct filtering returned
zero, while explicitly assigning/enumerating the same response returned one.
The API and PostgreSQL independently confirmed the scheduled job ran exactly
once, passed and cleaned up. The harness is corrected; the raw failed summary
is retained alongside the [verified 49-assertion summary](acceptance/pilot-20261118/recovery/verified-summary.json).
This shorter successful recovery phase is not a clean two-hour acceptance run.

The updated local installation passed another real restore/data-check/cleanup
smoke test. Final isolated-pilot inspection found zero active jobs, restore
containers or private snapshot entries. Test containers were stopped; existing
local data volumes were preserved. See [final cleanup evidence](acceptance/pilot-20261118/final-cleanup.json).
Linux CI passed runtime commit `633d55e`, including integration tests and the
vulnerability scan; the packaged Linux binary was independently scanned too.

## Remaining before wider availability

- Continue host-level diagnosis if the Desktop delay recurs; its exact Windows
  trigger is not established. The bounded recovery and two-hour acceptance above
  address the observed application failure propagation.
- Have an external operator complete a seven-day pilot using their real backup
  producer, actual database size and extensions, host and alert receiver.
- Verify the actual receiver's webhook payload/adapter and delivery. Local tests
  cover SSRF restrictions and durable retries, not a third-party provider.
- Validate willingness to pay and the service/support promise with the pilot.
- Public SaaS requires individual identities, tenant isolation, appropriate
  execution/storage design, encrypted remote transport, operational monitoring,
  recovery policy and independent security review. The shared-token local
  product does not supply those capabilities.
- Larger archives, PostgreSQL versions other than 17, extra extensions, PITR and
  application-level recovery remain outside the supported tested scope.

Readiness: offer an **unattended local developer pilot within the scope above**.
Complete the external seven-day pilot before broad availability. See
[earlier relational acceptance](POSTGRES-ACCEPTANCE.md) for the
135,500-row source-loss/restore exercise and [build security](BUILD-SECURITY.md)
for Vite removal.
