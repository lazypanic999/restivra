# Bounded PowerShell metadata export

[Five PostgreSQL cases](summary.json) passed on Windows with a separate,
network-isolated PostgreSQL 17 container. A 100,000-row fixture produced a
4,043,327-byte custom archive and restored all rows. Existing complete/partial
files were preserved. A 1 MiB limit stopped output below the bound and left only
a partial file. An actual exclusive table lock caused a two-second export budget
to fail; the subprocess invocation returned in 3.46 seconds including startup.
No failed `pg_dump` connection remained, and the test stack was removed.

[The existing installation's metadata](local-metadata-check.json) also exported
and restored with the new command: 17 projects, 24 jobs, zero active jobs and zero
orphan records. Only its logical metadata and relationships were checked; no
worker was launched against the clone. Customer archives and credentials remain
separate recovery inputs.

Native-process tests separately check exact binary bytes, private-stderr
discarding, text capture limits, child timeouts, oversized binary output and an
unwritable output stream. CI runs both native tests and real export regressions.
The Linux shell exporter has not gained the PowerShell time/size limits.
