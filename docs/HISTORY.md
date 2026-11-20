# Retained history and project recovery

**Runs** searches all retained jobs, including archived projects. Search matches
the immutable project name saved with a run. Results are ordered by queue time,
with the run ID breaking ties. **Older**, **Newer** and **Return to latest** navigate
50-row pages without the former 100-record ceiling. New jobs do not shift the
cursor into duplicate older rows. Status changes and retention can change matches
between page requests; this is live history, not a frozen cross-page snapshot.

The overview counts all retained jobs and displays six lightweight recent rows.
History summaries omit SQL configuration and full reports; inspecting a run
fetches its detail. The legacy `/api/jobs` latest-100 array remains available for
existing scripts.

## Filters and CSV

Choose project-name text, result and optional dates, then **Apply filters**.
Dates use queue time in the browser's device time zone. The end date includes
the whole local day, including daylight-saving changes. Applied filters stay in
the URL and survive refresh. A project's **Run history** action scopes the view
to that project's identity; **Clear filters** includes every project again.

**Download CSV** exports all matching rows across pages, up to 10,000. Larger
selections require narrower filters and fail explicitly, without truncation.
Headers are stable English field names, strings are UTF-8 and timestamps are UTC.
The export contains outcomes, queue/start/finish times, attempt count, latest
attempt duration, archive name/size/hash, cleanup, error code and notification
state. It excludes SQL checks, raw database errors, backup rows and credentials.
Protect downloads as operational metadata.

CSV names that could be evaluated as spreadsheet formulas receive an apostrophe
prefix. Commas, double quotes and Unicode text are preserved. Report duration
is used when available; otherwise a completed attempt's start-to-finish interval
is used. An empty duration means unavailable. Webhook delivery attempts remain
separate from verification attempts.

Export count and rows use one read-only, repeatable-read database snapshot. The
bounded CSV is prepared before download headers are sent, so query failures do
not leave a partial file labeled successful.

## Archived projects

In **Projects**, choose **Archived projects** to inspect retained sources and
their last result. **Restore project** keeps the backup pattern, checks, resource
limits and history. Daily scheduling stays disabled until explicitly configured.
Restoration does not queue a verification or rewrite old outcomes. A project
with an active job cannot be archived; archiving never deletes its dump.

## API

Existing authentication, allowed-host, same-origin and CSRF protections apply.
GET responses are `no-store`.

| Endpoint | Response |
| --- | --- |
| `GET /api/history` | `{items, summary, next_cursor?}` |
| `GET /api/history/export.csv` | Complete filtered CSV, at most 10,000 rows |
| `GET /api/projects?archived=true` | Up to 500 archived projects |
| `POST /api/projects/{id}/restore` | Restored project; JSON `{}` and CSRF token required |

History parameters:

| Parameter | Meaning |
| --- | --- |
| `q` | Literal case-insensitive project-name substring, at most 480 UTF-8 bytes; database case rules apply |
| `project_id` | 32-character lowercase hexadecimal identity |
| `status` | `queued`, `running`, `passed`, `failed`, `interrupted` or `cancelled` |
| `since` | Inclusive queue-time boundary, RFC 3339 timestamp with time zone |
| `until` | Exclusive queue-time boundary, RFC 3339 timestamp with time zone |
| `limit` | Page size, 1–100; defaults to 50; does not cap exports |
| `cursor` | Previous `next_cursor` with the same filters; exports reject cursors |

Summary counts cover the full filtered selection, not just one page. Count and
page rows share a transaction snapshot. The cursor is a pagination hint, not an
access credential. Invalid filters or mismatched cursors return 400; oversized
exports return 422. A missing JSON report retains its existing 409 response.

## Worker recovery

The worker reconciles owned containers and private snapshots before executing a
job and roughly every 30 seconds while idle. This catches a Docker create that
finishes after cancellation and the first absence check. Reconciliation holds
the installation's worker lock and never overlaps its executing restore.
Unknown cleanup stops the worker. Slow/unavailable infrastructure can delay
cleanup; the interval is not a 30-second SLA. Foreign installation resources and
unrelated standalone CLI work are excluded.

Completion also checks the attempt number under the job's row lock. A delayed
result from an earlier worker cannot finish a newer active attempt or publish a
misleading success. Previous attempt evidence remains intact.

## Validation

Real PostgreSQL tests cover 207 equal timestamps, insertion between page reads,
archived sources, date boundaries, Unicode/quoted CSV, formula protection,
exports at 10,000 and rejection at 10,001, schedule-disabled project restoration
and a stale completion during a newer active attempt. Real Docker tests verify
idle cleanup of a late cancelled container/snapshot without restarting the
worker or deleting a foreign installation's container.

Browser checks: [English filters](screenshots/history-filters-en.jpg),
[Turkish history](screenshots/history-tr.jpg), [390px view](screenshots/history-mobile-tr.jpg)
and [archived sources](screenshots/archived-projects-tr.jpg). These use historical
synthetic restore results and new synthetic QA projects.
