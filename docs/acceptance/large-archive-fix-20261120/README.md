# Large-archive correction and pilot candidate

The November 20 [stopped checkpoint](../development-checkpoint-20261120/) found
intermittent `pg_restore --list` failures with a valid 59,244,344-byte archive.
The original command failed 12 of 30 isolated listings. PostgreSQL reads only
the header and table of contents for listing; closing before Docker forwards the
remaining stdin can make Docker report an input-stream failure.

The worker now keeps that exec stream open and drains the remaining bytes. The
fixed shell program preserves PostgreSQL's failure status and rejects a failed
drain. It contains no user-provided shell text. Archive bytes still come from the
hashed private snapshot; nothing is mounted from the live source database.

## Regression coverage

`TestIntegrationLargeArchiveListing` uses actual PostgreSQL 17 and the Docker CLI
with an archive exceeding 4 MiB. Twenty repeated listings must succeed. A corrupt
8 MiB input must fail even though draining succeeds. A reader holds EOF open to
cancel specifically during stdin draining; cancellation must stop within the
command cleanup budget. The source hash must remain unchanged.

The [source CI](https://github.com/lazypanic999/restivra/actions/runs/44768089591)
passed this test alongside the existing PostgreSQL/API/S3 integration checks,
frontend tests/build/audit, Go vet/vulnerability scan, fresh Compose smoke and
PowerShell metadata export/restore regressions.

## Full API/worker acceptance

[The summary](summary.json), [individual outcomes](runs.ndjson) and runtime
identities record the original 59,244,344-byte reproduction archive, not a new
smaller replacement. Eleven outcomes passed: eight repeated full restores, a
successful retry after a worker SIGKILL, a post-fault restore and a real daily
scheduled run. Four intentional failures cover invalid archive headers,
truncated archive data, a false application check and insufficient restored-data
capacity. One running restore was cancelled. All 16 persisted outcomes were
re-read; only the deliberate worker interruption required a retry, preserving its
first interrupted attempt.

Successful runs checked 100,000 payload rows, each 1,024 bytes, plus 128 expansion
rows, each 1,120,000 bytes. They used 1,024 MiB data / 1,536 MiB memory, with the
usual 256 MiB archive bound and 180-second attempt budget. This is one synthetic
sizing example. PostgreSQL system files, indexes and WAL also need data capacity;
compressed archive size alone is not a sizing rule. Defaults were not raised.

The application image and service container IDs stayed fixed, including across
worker stop/start. Final checks found zero active jobs, private snapshot files
and owned restore containers. Development harness errors are described in the
summary: an earlier reserved-variable run is excluded; the fault observer was
corrected to include Docker's required PID column, and recorded completed
outcomes were independently re-read before and after continuation. Neither is
represented as a product failure or silently discarded archive error.

## Pilot scope

Use the [pinned participant guide](../../PILOT-START.md) or
[Turkish guide](../../PILOT-START.tr.md) on a developer's own computer with trusted
PostgreSQL 17 custom archives. Begin with the Docker-only passing/failing smoke
test before adding real data. Keep the existing backup/recovery process.

An external developer's seven-day pilot, arbitrary database sizes/extensions,
other PostgreSQL majors, macOS/ARM and provider-specific S3 configurations remain
unvalidated. The earlier interrupted long runs remain historical evidence of
different images and are not claimed as long-duration acceptance of this fix.
