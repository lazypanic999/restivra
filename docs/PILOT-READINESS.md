# Pilot readiness review — 2026-11-18

Scope: a developer verifying their own trusted PostgreSQL 17 archives on their
computer, with Docker Desktop Linux containers and localhost access. PowerShell
7.2+ on Windows is the primary handoff path; native Linux is an alternative.
A real customer pilot remains
the next product milestone. Public hosted SaaS readiness is a separate project.

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

## Release evidence

Long-duration isolated Compose testing and final Linux CI results are recorded
here once complete. The soak uses only synthetic commerce data, its own metadata
and volumes, and a separate loopback port; it does not alter the existing local
installation's archive/history volumes.

An early candidate's soak stopped after 14.04 minutes and 26 asserted outcomes:
an unplanned interrupted restore coincided with image extraction/compilation and
a 19.4-second metadata checkpoint (9.2 seconds syncing). That candidate had a
five-second heartbeat budget. It failed safely and recovered, but the result was
not accepted as a successful soak. Heartbeat/write changes above address short
host I/O stalls; the final soak is restarted from a fresh installation and adds
an explicit eight-second database pause during an active verification. Loss of
the advisory-lock session continues to stop the worker rather than allowing
concurrent restore ownership.

## Remaining before wider availability

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

Conclusion is intentionally scoped: completion of engineering checks supports
an assisted technical pilot; it does not establish production SLA or recovery
guarantees. See [earlier relational acceptance](POSTGRES-ACCEPTANCE.md) for the
135,500-row source-loss/restore exercise and [build security](BUILD-SECURITY.md)
for Vite removal.
