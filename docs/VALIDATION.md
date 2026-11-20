# MVP validation — 2026-11-17

Latest assessment: [2026-11-18 pilot readiness review](PILOT-READINESS.md).
The final fixed-image desktop run passed 120.01 minutes and all 210 expected
outcomes, with two planned worker recoveries and no other automatic retries.
Separate live Docker/metadata stalls recovered automatically, and a delayed retry
remained cancelled. The earlier failed 97.4-minute run remains historical evidence;
the exact Windows stall trigger is still unproven.

Additional validation: [PostgreSQL 17.12 relational acceptance run](POSTGRES-ACCEPTANCE.md) with 135,500 synthetic rows, source-data loss and eight expected outcomes; [English/Turkish interface and design review](DESIGN-REVIEW.md).

## November 20 regression coverage

- Actual PostgreSQL/Docker suite passes with retained-history pagination, CSV,
  archive restoration and stale-attempt completion rejection.
- Nine frontend tests pass, including device-local dates across daylight-saving
  changes and shareable filter URLs. Production build and vet pass.
- English/Turkish browser checks verified more than 100 real retained records,
  full and scoped CSV downloads, archive recovery and a 390-pixel viewport.
- Live idle reconciliation cleaned a late cancelled container and snapshot in
  28.17 seconds without a worker restart; foreign resources remained intact.
- A fixed-image 20.01-minute regression passed all 38 expected outcomes, including
  a planned worker kill, metadata restart/pause and actual scheduled restore.
  There were no other automatic retries or final resource leftovers.
- Linux race checks and the packaged Linux binary vulnerability scan pass.
- New behavior and limits: [history guide](HISTORY.md). Sanitized evidence:
  [November 20 acceptance](acceptance/history-20261120/).

## Completed locally

- Frontend production build with direct esbuild; the full npm dependency audit is clean, including development tools. Vite/picomatch removal and the development-asset boundary are documented in [BUILD-SECURITY.md](BUILD-SECURITY.md).
- `go vet ./...` passed.
- `RESTIVRA_INTEGRATION=1 go test -p 1 ./... -count=1 -timeout=10m` passed against real Docker/PostgreSQL 17 containers.
- Multi-stage Docker build and Compose startup completed; API, application database and worker are running.
- API localhost binding and worker heartbeat verified.
- Browser checks: create a project, add a SQL check, queue a run, observe a passing result, open its report and download its JSON.
- Narrow-screen layout checked at a 390-pixel viewport; wide data tables scroll within their cards.
- No browser console errors during the final check.
- Synthetic demo contains a genuine passing restore/check and a genuine intentionally failing business check. It is not customer data.

## Automated behavior covered

- Restore success, corrupt archive, false/NULL/error checks, blocked write, query and total-run timeouts.
- Snapshot fingerprints, resource-isolation flags, cleanup after failure/cancellation and refusal to clean unowned containers.
- Path traversal, contained backup access, supported query grammar and unsafe webhook destinations.
- Browser host/origin/fetch-metadata/CSRF request boundaries.
- Time-zone-aware next-run calculation, including a nonexistent daylight-saving time.
- PostgreSQL schema initialization, persistent queue, duplicate prevention and queued/running cancellation.
- Single-worker exclusion, real abandoned-container cleanup, abandoned-snapshot cleanup and interrupted-run recovery.
- Unchanged-backup warning, report download, single scheduled enqueue and advancing next_run_at.
- Persisted webhook retry exhaustion without changing verification status. No real external webhook was contacted.

## Limits

Validation uses synthetic, small PostgreSQL dumps. A real customer pilot, one week of unattended operation, custom-extension compatibility and large-database resource sizing have not been completed. Screenshots show only synthetic Demo projects. External webhook delivery requires the operator's own receiver and has not been tested against a real third-party endpoint.

GitHub CI independently runs the production frontend build, vet, integration suite and Go build on Linux after each push.
