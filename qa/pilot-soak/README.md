# Isolated pilot soak

Run with PowerShell 7.2+ and a local Linux/amd64 Docker engine:

```powershell
./qa/pilot-soak/run.ps1 -ApplicationImage restivra:local -Minutes 120 -Port 27484 -RequireReportBuild
```

The runner creates a fresh directory, random Compose project, credentials, data
volumes and synthetic commerce dump. It pins the supplied application to its local
image ID, so moving a development tag cannot change the tested binary. No host
Go, Node or PostgreSQL is required.

The existing pilot workload exercises repeated restores, false checks, missing and
corrupt archives, cancellation, worker SIGKILL, database restart/stall and a real
wall-clock schedule. Short runs cover only the scenarios reached in that time;
use the per-run evidence rather than assuming every scenario ran. A 120-minute run
is intended to reach all faults and repeated recovery.

Afterward every recorded outcome is independently read again by its ID, including
runs beyond the legacy latest-100 window. The runner checks stable application
and restore image identities, recorded verifier builds, and absence of owned
restore containers or private snapshot files. `-RequireReportBuild` requires the
new provenance field on reports; omit it when comparing an older candidate that
predates report provenance. A missing-archive failure can legitimately lack a
restore report because no attempt reached the verifier.

Cleanup removes only this newly created Compose project's containers/network and
the disposable source container. Volumes, archives and evidence remain under
`work/soak-*` for private review. It never accepts an existing installation root,
prunes Docker or changes a real user's projects. Failures remain failures even
when cleanup succeeds. This tests a local synthetic workload, not a real provider,
power loss, major database migration or customer recovery procedure.
