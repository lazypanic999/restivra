# Repeatable large-archive acceptance

PowerShell 7.2+ and a local Linux/amd64 Docker engine are required:

```powershell
./qa/large-archive/run.ps1 -ApplicationImage restivra:local
```

By default a disposable, isolated PostgreSQL source generates a synthetic archive
with 100,000 1,024-byte payload rows and 128 large text rows. The archive must
exceed 32 MiB; a small smoke dump cannot substitute for this test. To rerun an
existing reviewed **synthetic** reproduction archive, provide both `-Archive`
and its expected lowercase `-ExpectedSHA256`. Its source and copy hashes are
checked before and after acceptance; the original archive is never modified.

Every run creates a new installation, random project name, credentials and data
volumes, and pins the application by local image ID. It tests eight full restores
by default, invalid/truncated input, a false check, insufficient data capacity,
cancellation during an observed real `pg_restore` process, actual worker SIGKILL
during restore, subsequent recovery and a real wall-clock scheduled run. Successful
results must match row counts, payload widths and the source archive hash. New
reports must retain the running verifier's build identity.

Evidence remains privately under `work/large-archive-*`. Only this unique test
project's containers/network and its disposable source are removed; data volumes
remain. It never accepts an existing installation root or prunes Docker. The
1,024 MiB data / 1,536 MiB memory settings are one tested fixture, not a sizing
guarantee. Production dumps, arbitrary extensions and other PostgreSQL majors are
outside this test.
