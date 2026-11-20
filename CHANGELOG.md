# Changelog

## 0.1.0-pilot.2 — local developer pilot

### Added

- Docker-only metadata backup verification with isolated PostgreSQL, schema/count
  checks, source hashing, owned cleanup and structured JSON. Local Docker-context
  guards and bounded, private native-process I/O are exercised in CI.

- Private project settings export/import with an editable review, new project
  identity, disabled imported schedules and strict version/shape/credential-field
  checks. Worker keys and run history are excluded.

- Previous-run comparison using immutable SQL definitions, with separate rule
  changes, missing execution evidence, source/limit/image changes and SHA-256
  equality. English/Turkish UI and same-project authenticated API.

- Editable SQL templates for row counts, recent application data and required
  columns, with real PostgreSQL edge-case tests and English/Turkish guidance.

- S3-compatible worker sources with private profiles, bounded conditional
  snapshots, immutable queued selections and source evidence in history/CSV.
- English/Turkish printable evidence reports with name redaction and preserved
  attempt outcomes; browser Print can save them as PDF.

- Pinned developer-pilot onboarding, Turkish seven-day plan and private record
  template; Docker-only synthetic smoke checks with no host Go/Node requirement.
- CI checks fresh Compose setup, preserved credentials on repeated setup and
  passing/intentionally failing real PostgreSQL verification.

- Search across retained run history, result/date filters, stable cursor pages
  and shareable filter URLs in English and Turkish.
- CSV export for up to 10,000 matching runs, with spreadsheet-formula protection.
- Archived-project browsing and recovery without automatically resuming schedules.
- Six transparent badger mascot poses and documented generation prompts.
- MIT license, dependency notices, contribution/security guides, issue and pull
  request templates, and weekly Dependabot checks.

### Fixed

- Large custom archives no longer fail intermittently when `pg_restore --list`
  exits before Docker finishes forwarding stdin. The listing command drains
  remaining input while preserving failures. Real Docker/PostgreSQL regressions
  cover repeated large listings, corrupt input and cancellation during draining.

- PowerShell metadata exports have time/byte limits, discard native stderr and
  retain failed output only as `.partial`. Real PostgreSQL tests cover a locked
  table, oversized export, exclusive outputs and a successful 100,000-row restore.

- Metadata export/report paths follow PowerShell's current location after `cd`,
  rather than the native process's original directory. CI reproduces the split.
- Text source and embedded assets retain LF on Windows/Linux checkouts; binary
  media is excluded from line-ending conversion.

- New existence/minimum-row templates stop after finding enough rows, verified
  by PostgreSQL query plans against 100,000 rows. Saved custom SQL is preserved.

- Interrupted S3 HTTP downloads now discard partial snapshots and use the bounded
  infrastructure retry policy. Fully downloaded corrupt archives and local
  snapshot failures remain final; cancellation still does not retry.

- Scheduled S3 jobs preserve their source selection, just like manual jobs.

- History and overview counts no longer stop at the latest 100 jobs.
- Late containers and snapshots from cancelled work are reconciled while idle
  and before the next restore; the worker preserves foreign-installation resources.
- A delayed completion from an earlier attempt cannot overwrite a newer one.

### Validation and limits

The earlier November 20 session stopped before its planned long tests ended;
its [checkpoint](docs/acceptance/development-checkpoint-20261120/) remains historical.
The subsequently corrected Docker stdin/listing failure has separate
[large-archive acceptance](docs/acceptance/large-archive-fix-20261120/), including
full API/worker restores, negative cases, cancellation and worker recovery.

Real PostgreSQL/Docker regression tests, frontend tests, browser checks and
sanitized acceptance evidence are linked in [the history guide](docs/HISTORY.md)
and [pilot readiness review](docs/PILOT-READINESS.md). The earlier two-hour run
belongs to runtime `5ef8f0a`. An external developer's week-long pilot, large dump
sizing, extension compatibility and paid-pilot validation remain open.

There is no tagged stable release yet. The supported pilot uses trusted
PostgreSQL 17 custom-format archives on the operator's own computer.
