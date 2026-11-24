# Publishing a ready-image pilot

Releases contain a public Linux/amd64 image and `restivra-release.json` asset.
Installation/update scripts trust this repository's HTTPS release assets and
maintainer-declared metadata compatibility. Never replace a published tag or
manifest to repair a release; publish a new version.

1. Review the source and compatibility. `metadata_contract: 1` permits application
   rollback against the same live PostgreSQL data. Any incompatible migration or
   write-format change must block this update path and receive a separately
   reviewed migration/recovery procedure. Do not label an incompatible image as
   contract 1 merely because its tests pass.
2. Update version examples/defaults, change notes and pilot instructions for the
   new tag. Run the relevant tests and push. Require successful `CI` on the exact
   source commit that will be tagged.
3. Create an explicit prerelease with that commit and reviewed notes, using
   `gh release create <version> --target <commit> --prerelease --notes-file <file>`.
4. Dispatch `Publish release image` with that version:

   ```powershell
   gh workflow run publish-image.yml -f version=v0.1.0-pilot.4
   ```

   The workflow checks the exact tag's CI, builds once with OCI source/revision/
   version/contract labels, reuses existing version images, verifies binary SHA-256
   and requires anonymous registry access before attaching the digest manifest.
   It uses GitHub Actions' scoped `GITHUB_TOKEN`; no personal registry password is
   required. For a newly created GHCR package, ensure Public visibility and rerun
   if the anonymous check fails. Do not publish a private-image pilot handoff.
5. On the supported Windows/Docker environment, run the real release regression:

   ```powershell
   ./scripts/test-release-integration.ps1 -FromVersion v0.1.0-pilot.2 -ToVersion v0.1.0-pilot.4
   ```

   It creates a unique installation with synthetic data. It exercises a real
   restore and intentional SQL failure, update, explicit rollback, queued-work
   refusal and a real candidate startup failure followed by automatic recovery.
   It compares complete project/job/attempt fingerprints, private settings and
   the unchanged PostgreSQL container. It retains private checkpoints/volumes
   under `work/` and stops only its own project. Inspect its `summary.json`.
6. Independently clone the published tag and follow the participant's exact
   default install/check/smoke commands with a unique Compose project/port.
   Verify anonymous image pull with an empty Docker credential configuration.
   Sanitize public evidence; never attach `.env`, private profiles or metadata.
7. Attach the qualified synthetic summary and ready-image evidence to the
   release, replace provisional release-note claims with actual results and link
   the install/update guides. Keep the existing pilot version fixed until its
   owner deliberately schedules a compatible update.

Publication and tests do not establish ARM64/macOS support, cross-major PostgreSQL
upgrades, recovery after every host failure, or an external seven-day user pilot.
Source development builds use `compose.build.yaml` and are outside the managed
release-image path unless binary-identical adoption is explicitly validated.
