# Pilot operations acceptance — November 24

All fixtures are synthetic. These records cover local Windows/PowerShell and
Linux/amd64 Docker; they do not establish an external customer's recovery plan.

## Functional changes exercised

- Authenticated support download excludes project names, SQL, paths and keys.
- Actual build identity appears in the interface and new restore reports. A
  retained report keeps its verifier identity independently of the presenting API.
- All projects can be searched and paged beyond the legacy 500-row cap.
- Failed refresh hides old healthy status. Clock-skewed heartbeats are rejected
  consistently by health and source-catalog endpoints.
- Failed notification delivery has its own history filter/CSV and a direct
  System check link, independently of the verification result.
- Fresh setup saves a unique project name/port. Pilot checks reject the wrong
  owned API port before authenticating and clear operator secrets in QA fixtures.
- Compatible interrupted updates recover the previous verified application
  without overwriting PostgreSQL contents. Unix private files are protected and
  only the read-only source-profile file is mounted into the worker.

## Evidence and scope

[Browser assertions](browser.json) use real Chromium, authentication and
PostgreSQL-backed UI workflows: English/Turkish, phone layout, safe support JSON,
failed refresh, delivery filtering, old-project navigation, project create/edit,
passing and deliberately failing SQL restores, JSON/CSV, printable provenance,
and archive/restore with scheduling disabled. The tested binary records source
revision `2db2a4d9c5bbacfef75177c755f3190970978755`. Later source-catalog heartbeat
consistency was separately checked in the real PostgreSQL API integration and CI.

The [original reproduction archive](large-original.json) is **59,244,344 bytes**,
SHA-256 `ccc59d31c0b1d6f03e156e9f645194848e3ab12d72d220a789e1bec7888acff2`.
It is the same archive used for the November 20 stdin/listing correction.
[Sixteen outcomes](large-original-runs.ndjson) include eight complete restores,
invalid/truncated input, a false check, insufficient data capacity, cancellation
during an observed `pg_restore`, worker SIGKILL during restore, subsequent
recovery and a real daily schedule. All retained outcomes were re-read; source
hash, fixed application identity, cleanup and absence of private job files were
checked. This is one sizing fixture, not a general database-size guarantee.

The [generated capacity fixture](large-generated.json) separately exercises the
self-contained generator and ten outcomes (two repeated complete restores plus
the remaining cases). Generation streams SQL/archive bytes with bounded native
I/O from a read-only, non-root source with tmpfs; earlier Docker-copy harness
failures before any verification are excluded from the successful run.

[Interrupted-operation drills](recovery.json) actually kill the updater during
metadata export and after a healthy candidate API. They test identity/schema/image
drift refusals, queued work, a missing worker container, preservation of unrelated
one-off commands and interrupted binary-identical adoption. Database identity,
full project/job/attempt fingerprints and private file contents are preserved.

[Legacy mount migration](legacy-private-windows.json) starts the old published
Compose topology, then updates host packaging and rolls back the application.
It verifies the narrow read-only mount, atomic profile replacement/reload and
absence of credential canaries in the API. Windows uses inherited ACLs; the
0600/0700 assertions run separately in the Linux CI migration job.
[PowerShell 7.2 private-I/O compatibility](private-io-7.2.json) is limited to that
helper and its literal-path/permission contracts.

[Published pilot.2 → pilot.3 integration](release-integration-2-to-3.json) covers
real image installation, preserved metadata/credentials/database container,
explicit rollback, queued-work refusal and actual failed-startup recovery using
the new host scripts. New runtime-identity and report-provenance paths are
exercised separately by the later release integrations below.

## Published releases

The immutable [pilot.4 manifest](pilot4-release.json) and
[anonymous pull](anonymous-pilot4.json) bind the published image to source
`61c06e3a4e79fec5236b8ccdf6a4fc4b06fd8f7e`. Its
[pilot.3 → pilot.4 integration](release-integration-3-to-4.json) additionally
creates real new-version reports and verifies their recorded provenance survives
rollback to the older binary and recovery after an actual failed startup.

The recommended participant release is **pilot.5**. Final visual review found
that pilot.4's setup/missing-profile copy still said restart, even though the
storage guide correctly required recreation after atomic profile replacement.
pilot.5 aligns those messages; pilot.4's tag/image were not changed.

[pilot.5's manifest](pilot5-release.json) records source
`78f2a782f457ed94a087f89168028d293751273e`.
[Anonymous pull and isolated version/hash checks](anonymous-pilot5.json) passed.
[A fresh public Git-tag install](fresh-tag-pilot5.json) used the tag's default
installer, then opened clean child PowerShell processes with project/port
overrides removed. The saved identity/port, authenticated checks, real
passing/failing PostgreSQL samples, repeated setup and both reports' verifier
identity passed. [pilot.3 → pilot.5 integration](release-integration-3-to-5.json)
also passed update, rollback, queued-work refusal, real startup-failure recovery
and preserved report provenance. Private settings, the original database
container and full project/job/attempt fingerprints were checked.

Exact-source [pilot.5 CI](https://github.com/lazypanic999/restivra/actions/runs/45422028750)
and [image publication](https://github.com/lazypanic999/restivra/actions/runs/45423136704)
completed successfully. A separate fixed published-image soak is recorded only
after it finishes; the development-build duration below does not qualify this
new image's elapsed runtime.

CI also runs the PostgreSQL engine/listing regressions, S3-compatible integration,
SQL-template edge cases, frontend tests/build/audit, Go vet/vulnerability checks,
fresh installation and metadata export/isolated restore checks. Successful
source checkpoints include [2db2a4d](https://github.com/lazypanic999/restivra/actions/runs/45403015772),
[private-file packaging](https://github.com/lazypanic999/restivra/actions/runs/45410729259),
and [QA isolation / legacy migration](https://github.com/lazypanic999/restivra/actions/runs/45415375966).

## Long-duration development builds

The first fixed-image [120.01-minute soak](soak-7124103-120min.json) completed
140 iterations and **204 expected outcomes**: 148 passes, 42 deliberate failures
and 14 cancellations. Two worker SIGKILLs, two metadata restarts, an eight-second
database stall and an actual scheduled run recovered. Its
[wrapper/build record](soak-7124103-wrapper.json) identifies revision `7124103`,
before verifier provenance was added; [outcomes](soak-7124103-runs.ndjson) are
retained. This earlier wrapper did not independently re-read every retained result
at the end. A later runner adds that additional audit; do not infer it here.

The second fixed development build, revision `2db2a4d`, independently completed
[120.01 minutes / 140 iterations](soak-2db2a4d-120min.json) with
[204 expected outcomes](soak-2db2a4d-runs.ndjson): 148 passes, 42 deliberate
failures and 14 cancellations. The same worker/metadata faults and actual
schedule passed. Its [stronger wrapper](soak-2db2a4d-wrapper.json) independently
re-read all 204 retained outcomes, including IDs outside the latest-100 list,
matched each available report's verifier build and restore image to the fixed
runtime, and found zero owned restore containers or private job files.
This evidence belongs to that development image, not the later published tag.

Additional long-duration and final published-image evidence is added only after
completion.
Customer/provider pilots, other PostgreSQL majors, arbitrary extensions,
macOS/ARM64, hostile dumps, billing and hosted multi-tenancy remain outside this
acceptance.
