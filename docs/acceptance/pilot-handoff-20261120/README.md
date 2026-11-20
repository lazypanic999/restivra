# Fresh developer-pilot handoff check

The new participant scripts were exercised from a fresh local Git clone of
`3fdd162`, on Windows PowerShell 7 with Docker Desktop Linux containers. An
independent Compose project, port 27474 and fresh metadata/worker volumes were
used. An image-tag override kept the existing main installation's image tag
unchanged; the application and participant script code were unchanged.

- Setup generated independent credentials; repeating it preserved the file.
- The prerequisite check required working authentication, online worker,
  API/database readiness and worker diagnostics.
- `pilot-smoke.ps1` created a real PostgreSQL 17 custom dump and ran it through
  the API/worker. The positive SQL check passed; `SELECT false` failed as intended.
  Both restores and cleanups passed. No host Go/Node invocation was needed.
- The synthetic source container was removed. Projects and sample dump remained
  for inspection, with schedules disabled and a private local evidence directory.
- A worker restart was followed by a successful prerequisite check.
- A legacy unauthenticated main installation was explicitly rejected by the
  pilot check, without changing its credentials or records.

[Setup evidence](setup.json) · [Synthetic smoke summary](smoke.json)

The final wording clarifies that the prerequisite check itself does not perform
a restore. Linux CI repeats fresh setup and the smoke script using PowerShell.
This is engineering handoff validation; an external developer's seven-day pilot
has not been completed. The primary handoff is the [Turkish participant guide](../../PILOT-START.tr.md).
