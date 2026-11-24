# Private installation files

PowerShell setup, atomic release-state writes and metadata exports now create Unix
files as **0600 before writing content**. New private/checkpoint directories use
**0700**. Existing configuration bytes are preserved. On Windows the helpers keep
the inherited NTFS ACL; use a protected user directory rather than a shared folder.

Newer PowerShell runtimes use [.NET's UnixCreateMode](https://learn.microsoft.com/en-us/dotnet/api/system.io.filestreamoptions.unixcreatemode).
PowerShell 7.2 / .NET 6 uses a bounded POSIX noclobber/umask fallback with arguments
passed separately. Paths containing spaces, brackets, Unicode and shell characters
are literal. Permission modes are checked before any private content is written;
filesystems that cannot enforce them fail. Existing files are never truncated by
exclusive creation, and permission changes refuse symbolic-link targets.

## Worker access and updates

Only `private/sources.json` is mounted into the worker, read-only. Deployment
records, checkpoints and copied `.env` files are no longer part of that mount.
The already Docker-administrative worker receives `DAC_READ_SEARCH` so it can
read a bind-mounted 0600 file owned by the host operator on rootful Linux.
The API and temporary PostgreSQL restore containers keep all capabilities dropped.
The worker's Docker socket remains host-administrative access; protect the host.

Managed install/update/recovery restrict existing Unix `.env` and source-profile
files to 0600 and `private/` to 0700 after the supported worker configuration is
running. This ordering keeps the old container's prerequisite check working during
an upgrade. Missing capability or linked configuration paths require review rather
than silently changing an unrelated target. Application rollback uses the current
Compose packaging and retains these permission protections.

After editing a source profile, recreate the idle worker to bind the current file:

```powershell
docker compose up --no-build --no-deps --pull never --force-recreate -d worker
./scripts/pilot-check.ps1
```

Finish active jobs first. The worker reads profiles at startup; a file bind mount
can otherwise retain an old inode after an editor atomically replaces the file.

Earlier PowerShell setup on Unix could create 0644 configuration files beneath
0755 directories. On a shared host, other local users may have been able to read
them. Restricting permissions prevents new reads; it cannot revoke secrets already
copied. Review access and rotate exposed access/storage keys if relevant. Database
password replacement requires an explicit database credential procedure; setup
does not invent a new password for an initialized PostgreSQL volume.

`test-private-io.ps1` exercises exclusive creation and literal paths; Unix runs
also check modes and refusal to follow a symbolic link. The fallback was run in
an isolated, network-disabled Microsoft PowerShell **7.2.7** container as UID 1001.
That historical runtime is a compatibility test fixture, not the application image.
