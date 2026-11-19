# Restivra roadmap

## Implemented MVP

- [x] PostgreSQL 17 custom-dump restore engine and CLI.
- [x] Isolated containers, limits, fingerprinting, boolean checks and JSON reports.
- [x] Persistent PostgreSQL projects/jobs, single worker and heartbeats.
- [x] Duplicate prevention, cancellation and restart resource reconciliation.
- [x] React dashboard, project settings, history and report downloads.
- [x] Daily time-zone-aware schedules and unchanged/old-file warnings.
- [x] Failure webhook with persisted delivery retries.
- [x] Docker Compose; Docker socket only in worker.
- [x] Unit/security tests, real PostgreSQL integration tests and CI.
- [x] Single-operator access-token sessions, worker diagnostics and API readiness.
- [x] Database-scoped resource ownership, fail-stop cleanup and metadata exports.
- [x] Durable bounded infrastructure retries, preserved attempt history and retry cancellation.
- [x] Fixed-image two-hour desktop acceptance and real Docker/metadata fault recovery.
- [x] Dark black-and-crimson interface layer, new logo, README screenshots and walkthrough video.

## Pilot acceptance — requires real users

- [ ] Continue host-level investigation if the Desktop stall recurs; its exact trigger is still unproven. [Recovery acceptance](docs/PILOT-READINESS.md) passed.
- [ ] One developer, their own computer, two authorized projects and one week of scheduled tests.
- [ ] Validate extensions, dump versions and database sizing.
- [ ] Observe installation, diagnosis and maintenance needs.
- [ ] Offer a paid pilot and validate willingness to pay.

## Later

S3, PDF, extra PostgreSQL versions/custom extension images, pagination, public remote deployment, roles and billing. Hosted multi-tenancy and hostile-dump execution require a separate security design. Other engines and PITR require separate restore strategies.
