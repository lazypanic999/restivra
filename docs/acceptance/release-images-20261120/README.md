# Ready-image installation and compatible update qualification

Date: 2026-11-20. Tested on Windows with PowerShell 7 and local Docker Desktop's
Linux/amd64 engine. All external-registry checks used public GHCR images.

Qualified packaging source: `42723b6c488bd2ecfae011a860ec33381c29d786`
(`v0.1.0-pilot.3`). [Exact-source CI](https://github.com/lazypanic999/restivra/actions/runs/44795420870)
passed, including npm tests/audit/build, Go vet/vulnerability/integration tests,
real SQL template checks, fresh source Compose smoke and metadata regressions.

## Public artifacts

- [pilot.2 image publication](https://github.com/lazypanic999/restivra/actions/runs/44793819989)
  and [manifest](pilot2-release.json).
- [pilot.3 image publication](https://github.com/lazypanic999/restivra/actions/runs/44795930087)
  and [manifest](pilot3-release.json).
- [Local anonymous pull proof](anonymous-pull.json), using an empty Docker
  credential directory and the explicitly selected local engine endpoint.

Both images contain application binary SHA-256
`c0f77cefba27180cfe05a3960c90065f0f08219dbd35cd0811f9e6ba083dcbee`.
The engine/UI are unchanged from the [large-archive fix qualification](../large-archive-fix-20261120/).
Image digests differ because source/version OCI labels and packaging differ.

## Real installation and update tests

[Fresh tagged-install proof](fresh-tag-install.json): cloned the published
`v0.1.0-pilot.3` tag from GitHub into a new directory, with unique Compose project,
port and newly generated credentials. Ran the default installer, authenticated
prerequisite check and Docker-only smoke. The synthetic passing check passed;
the intentional false SQL check failed while restore and cleanup passed. Repeated
setup preserved credentials. No host Go/Node or source build was used.

[Full update/rollback proof](update-rollback-integration.json) was produced by
`scripts/test-release-integration.ps1 -FromVersion v0.1.0-pilot.2 -ToVersion v0.1.0-pilot.3`.
It installed the public pilot.2 image in a separate unique project and exercised:

1. Real PostgreSQL restore with passing and intentionally failing SQL checks.
2. pilot.2 → pilot.3, explicit rollback to pilot.2, then update to pilot.3 again.
3. A synthetic deferred queue row: update refused before stopping/recreating API.
4. A Compose override making the official candidate exit with status 42 while
   the previous official image starts normally. The updater reported failure,
   restarted the previous image and passed API/worker checks. No mock Docker,
   updater bypass or database restoration was involved.

Complete project/job/attempt fingerprints, installation identity, private settings,
source profiles and the PostgreSQL container identity were compared across these
operations. They remained unchanged. Each switch used a real bounded metadata
export followed by an isolated restore/schema/relationship check. Private archives,
credentials, source profiles and filesystem paths are excluded from this record.
Only the test projects' own containers/networks were stopped; data volumes and
private evidence were retained locally.

## Existing local installation

[Local upgrade proof](local-upgrade.json) records the operator installation's
pilot.2 → pilot.3 update. It retained 17 projects and 24 jobs, all project/job/
attempt fingerprints, installation identity, PostgreSQL container, credentials
and source profiles. Its legacy authentication configuration was preserved;
fresh pilot installations above have sign-in enabled.

## Limits

This qualifies compatible application-image replacement with metadata contract 1.
It does not test database major-version upgrades, incompatible schema migrations,
ARM64/macOS, remote Docker, disk-full/power-loss recovery, hostile dumps or an
external seven-day user pilot. Interrupted operation journals require the manual
review/recovery described in [UPDATING.md](../../UPDATING.md). This short update
qualification is separate from earlier long-running acceptance evidence.
