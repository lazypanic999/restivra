# November 20 development checkpoint

Work stopped at the operator's request. Neither long test reached its planned
deadline. These are verified completed outcomes, **not** full-duration acceptance
or a new pilot release. The previous `v0.1.0-pilot.1` tag remains unchanged.

## Current application image

The [82.23-minute checkpoint](current-runtime/summary.json) used image
`b879f56640f0` throughout: 69 recorded outcomes (56 passed, ten intentional failures,
three cancellations) and 8,274 HTTP assertions. The
[independent re-read](current-runtime/verified-summary.json) verified all 69 persisted
jobs, including 12 with preserved retries. Final inspection found zero active
jobs, private snapshot files or owned restore containers. API/worker identities
did not change. Fault drills included truncated S3 HTTP bodies, worker SIGKILL,
storage restart and running-job cancellation. HTTP assertions covered comparison,
English/redacted Turkish evidence, scoped history, source catalog and access/origin
rejection. Small synthetic archives were used; this is not large-database acceptance.

The [earlier S3 baseline](earlier-s3-baseline/summary.json) used a different,
older image (`321bc0ec550c`). Its 194 recorded outcomes were independently
[re-read](earlier-s3-baseline/verified-summary.json), with five retried jobs and
zero active jobs/snapshots/owned restore containers. Its duration and results must
not be combined with the current image's checkpoint. Both disposable stacks were
removed without deleting their named volumes. The existing localhost installation
was retained.

## Known large-archive issue; release held

A separate 59,244,344-byte synthetic archive exposed an intermittent failure in
the Docker-streamed `pg_restore --list` stage. The same archive/hash passed a full
restore with 1,024 MiB data / 1,536 MiB memory, then failed listing on a later run.
[Original observed outcomes](large-archive/) retain that failure; it is not counted
as a successful capacity qualification. The archive failed actual restoration
with 512/128 MiB data limits; compressed file size is not restored capacity.

An isolated command-level [probe](large-archive/probe.json) reproduced 12 failures
in 30 direct listings. A candidate fixed shell command that preserves
`pg_restore`'s exit status and drains the remaining stdin produced zero failures
in 30 listings. **That candidate is not merged or installed.** It still needs a
real large-input regression, complete API/worker restore acceptance and updated
runtime evidence before release. No error is being ignored to label a run passed.

The proposed `v0.1.0-pilot.2` remains unpublished. The working patch and local
reproduction artifacts are preserved privately for the next development session.
Cloud-provider compatibility, an external developer's seven-day pilot and paid
use also remain unvalidated.
