# Printable verification evidence

Open a completed run's report and choose **Print / PDF report**. A separate,
authenticated HTML document opens in the selected English/Turkish language.
Use the browser's **Print → Save as PDF** command, or save the standalone HTML.
This is browser-assisted PDF output, not a server-side PDF generator.

The report is self-contained: no external fonts, scripts, images or analytics.
Print styling uses A4 margins, repeated table headers and rows that avoid page
breaks. All dates are explicitly UTC, so another reader does not silently see
times in a different browser time zone.

Included evidence:

- Immutable run/project identity, selected attempt outcome and current run state.
- Attempt start/finish, duration and retained previous attempt summaries.
- Archive restore, individual check names/results/codes and resource cleanup.
- Archive size, modification time, downloaded SHA-256 and restore-image identity.
- For S3 sources: profile ID, bucket/key, returned object version and ETag.
- Explicit limits: a logical restore is not full application recovery, original
  role/grant recovery, point-in-time recovery or proof of data recency.

SQL text, restored rows, access tokens, storage credentials and raw provider
diagnostics are excluded. Project names, check names and object keys may still be
sensitive metadata. Before sharing, use **Hide project, archive and check names**.
That option also hides storage profile, bucket/key, ETag and object version.
Run IDs, timing, byte sizes and SHA-256 remain; redaction is not anonymization.

Previous attempts with a stored report have their own Print/PDF link. An old
failed attempt stays visibly failed even if the run later succeeds. The current
run state is shown separately. Interrupted attempts without a full report remain
in the attempt summary; no report is fabricated for them.

## API

`GET /api/jobs/{id}/evidence.html?lang=en&redact=0`

- `lang`: `en` (default) or `tr`.
- `redact`: `0` (default) or `1`.
- `attempt`: optional preserved attempt number, 1–3.

The endpoint uses the same sign-in, allowed-host and same-origin checks as JSON
reports. Responses are `no-store`. Invalid options return 400; a missing full
report returns 409. User names are escaped with Go's HTML templates, never
rendered as HTML. A separate Content Security Policy disables all scripts and
external resources; the fixed inline stylesheet is the only inline asset.

Downloading or printing a report does not queue a verification, alter history or
change notification state. Keep saved files private until reviewed for sharing.
