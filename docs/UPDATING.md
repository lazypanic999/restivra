# Installing and updating release images

The supported pilot path is PowerShell 7.2+, Git and a local Linux/amd64 Docker
engine. No host Go, Node or PostgreSQL is needed. Windows is tested; ARM64 and
remote Docker engines are rejected. Never download a script and pipe it to a shell.

## First installation

Start Docker Desktop, then:

```powershell
git clone --branch v0.1.0-pilot.3 --depth 1 https://github.com/lazypanic999/restivra.git restivra-pilot
cd restivra-pilot
./scripts/install.ps1
./scripts/pilot-check.ps1
./scripts/pilot-smoke.ps1
```

Open http://127.0.0.1:7474. The sign-in token is in your private `.env` file.
The installer creates random credentials only if absent, downloads the explicit
release manifest from this repository, pulls its GHCR digest, checks image labels
and the binary hash, and waits for API readiness and a new worker heartbeat.
It records the deployment privately and pins `RESTIVRA_IMAGE` in `.env`.
`docker compose up -d` subsequently starts that exact image without a build.
New setup files persist a random `COMPOSE_PROJECT_NAME` and `RESTIVRA_PORT` in
`.env`, so later commands and a new terminal select the same containers and port.
For a second installation select an unused `RESTIVRA_PORT` before installing.
You may also choose a unique `COMPOSE_PROJECT_NAME` before setup; both are saved.
Existing `.env` files are never rewritten by setup. Older installations without
these entries still require their original project/port environment settings.
Terminal environment variables override `.env`; clear stale overrides before
operating another installation. Do not change an installed project's name, which
would select different database volumes instead of moving existing data.

## Update or roll back

Read the target release notes first. Finish running tests; stop submitting work.
In the existing installation directory, with the same Compose project/port:

```powershell
./scripts/update.ps1 -Version v0.1.0-pilot.3
# To return to the retained previous deployment:
./scripts/update.ps1 -Rollback
```

Version tags are examples, not an automatic latest-version selector. Use an
explicit published release with `restivra-release.json`. The updater downloads
and verifies the target before stopping anything. Queued/running jobs, container
ownership/image drift, a remote engine or an unsupported metadata contract abort
the operation. An exclusive per-directory lock prevents concurrent installers.

During a short maintenance window the API and worker stop. Idle state is checked
again to catch work submitted during shutdown. A bounded metadata export is
restored into an isolated temporary PostgreSQL container and checked before the
application image changes. `.env`, source profiles, metadata archive and the
verification report remain under `private/checkpoints/`. Keep this directory
private and back it up securely; retained checkpoints are not automatically deleted.

Only API and worker are recreated. The existing PostgreSQL container/volume is
never upgraded, recreated or restored by this command. Compatible metadata
contract **1** is required. Application rollback starts the previous exact image;
it does **not** overwrite the database with the checkpoint or lose later writes.
Updates involving destructive migrations require a separately reviewed procedure.
The check verifies deployment prerequisites, not a new restore of every user's backup.

If startup or the worker prerequisite check fails, the updater stops the candidate
and restarts the retained previous image, checks health again and restores the
previous image selector. The command still returns an error so failures are visible.
Credentials, source archives, project definitions and histories are not replaced.
The previous image must remain available; do not prune it during a pilot.

## Existing source installation

This one-time adoption is intentionally strict:

```powershell
# Bring the packaging scripts into the existing clean source checkout first.
git fetch origin main
git merge --ff-only origin/main
./scripts/update.ps1 -Version v0.1.0-pilot.3 -Adopt
```

Adoption requires the currently running API and worker to have the **identical
binary SHA-256** as the selected release, matching the reviewed metadata contract.
It retains the exact old image under a unique local rollback tag. A different
binary is refused; do not force it by editing deployment state. Existing sign-in
configuration is preserved. New installations use authentication by default.

## Interrupted operations and recovery

This is not an unattended major-version database migration tool. A power loss,
killed PowerShell process or Docker outage can interrupt the maintenance window.
An `in_progress` record in `private/deployment-journal.json` blocks further updates
until an operator reviews it. Do not delete volumes or blindly rerun setup.

For operations written by the current scripts (`restivra.operation.v2`), use:

```powershell
./scripts/update.ps1 -Recover
./scripts/pilot-check.ps1
```

This resumes the **previous application image**, rather than retrying the failed
update. The original metadata database container must still be running and
unpaused, with the same installation identity and required tables. Recovery checks
the journal's directory, Compose project, original database container, image
digests, binary hash, metadata contract and image selectors before stopping an
application. Queued work, paused containers, unexpected images or an unrelated
one-off maintenance command block recovery. A missing API or worker container can
be recreated; a missing metadata database requires manual review.

The verified previous image can be used from the local Docker cache without a
registry download. Recovery removes only diagnostic containers labelled for the
interrupted operation. It checks API readiness and a fresh worker heartbeat,
restores the previous deployment selector/state, and marks the journal recovered.
It does **not** restore a database dump or replace credentials, source profiles,
backup archives or history. Do not edit the journal to bypass an identity check.

If the interrupted operation used older scripts without a v2 journal, or automatic
recovery refuses an inconsistent installation, use this manual review procedure:

1. Restore Docker availability. Read the private journal's `previous.image` and
   checkpoint location locally. Never paste private files into an issue.
2. Set `RESTIVRA_IMAGE` in `.env` to that previous exact image, then run
   `docker compose up --no-build --no-deps --pull never -d api worker`.
3. Check `docker compose ps`, API health and worker diagnostics, then verify your
   projects/history. PostgreSQL should still be the original container and volume.
4. Reconcile `private/deployment.json` with the recovered deployment and archive
   the interrupted journal privately before retrying. Ask for help if image/state
   identity is unclear; do not claim recovery based only on a running container.

If automatic rollback also fails, the command says so and preserves its evidence.
Manual metadata restoration is a separate, potentially destructive operation;
follow [the recovery runbook](PILOT-RUNBOOK.md) and [metadata checks](METADATA-CHECK.md).
Fresh installation failures preserve credentials and volumes; inspect those
resources before recovery rather than deleting them. Build from source using
`docker compose -f compose.yaml -f compose.build.yaml up --build -d` only in a
development installation, outside the managed release-update path.

## Release trust

The workflow requires successful CI on the tag's exact source revision. Version
tags are reused rather than overwritten; manifests bind source revision, OCI
digest, binary SHA-256, platform and metadata contract. Publishing requires an
anonymous GHCR read check. This trusts GitHub HTTPS and repository maintainers;
it is not independent cryptographic signing or a promise that a pinned image has
no vulnerabilities forever. Base-image security updates ship as new versions.
