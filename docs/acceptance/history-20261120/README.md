# November 20 retained-history and recovery acceptance

This is synthetic local-developer validation, not an external customer pilot.
The application source tested here was committed as `d36ebdd`; the image was
built from that application's working tree before the commit. Runtime identities
are recorded in [soak/runtime.json](soak/runtime.json).

- [Browser checks](history-browser.json): all retained records accessible,
  no duplicate pages, English/Turkish, local dates, CSV and 390px layout.
  Counts were captured before the longer regression added more synthetic jobs.
- [Archived project recovery](project-restore.json): checks and historical result
  preserved, daily schedule disabled, actual restore passing.
- [Idle cleanup](idle-reconciliation.json): late cancelled container/snapshot
  removed in 28.17 seconds without a restart; foreign installation preserved.
- [Fixed-image regression](soak/verified-summary.json): 20.01 minutes,
  22 iterations and 38 matched outcomes. Nine failures and two cancellations were
  intentional. One worker kill recovered with its interrupted attempt retained;
  no other automatic retries occurred. Includes a metadata restart, eight-second
  metadata pause and real wall-clock dispatch.
- [Local upgrade](local-upgrade.json): private metadata export restored into a
  separate PostgreSQL instance before upgrade, original statuses/reports
  preserved, and one actual synthetic restore passing on the upgraded main app.

The shorter first development run was interrupted by intentionally replacing
the API image; it is excluded. The historical two-hour acceptance of runtime
`5ef8f0a` is separate in [the pilot review](../../PILOT-READINESS.md).
The exact Windows/Desktop stall trigger and external week-long pilot remain open.

The full real PostgreSQL/Docker suite, nine frontend tests, production build,
vet, Linux race checks, npm audit and source/binary Go vulnerability scans passed.
CI repeats the repository checks on Linux. Screenshots use only synthetic data
and are linked from [the history guide](../../HISTORY.md).

The upgraded [installed history screen](../../screenshots/history-installed-en.jpg)
shows all 19 main-installation records and an online worker. The extra record is
the post-upgrade synthetic smoke test. [Final cleanup](final-cleanup.json) removed
the isolated test stack without deleting volumes. One container left by an
earlier intentionally failing integration fixture was identified by its exact
test ownership labels and removed separately; it was not part of the passing soak.
