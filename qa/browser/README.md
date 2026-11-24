# Browser smoke against a synthetic Docker installation

PowerShell 7.2+ and a local Linux/amd64 Docker engine are required. First build
the application image from the source under review with a unique local tag:

```powershell
docker build --tag restivra:browser-candidate .
./qa/browser/run.ps1 -ApplicationImage restivra:browser-candidate
```

The runner creates its own directory, Compose project/volumes, credentials and
loopback port (27479 by default). It never accepts an existing installation root
or copies a user's `.env`, source profiles or database. A real PostgreSQL smoke
creates synthetic backups and passing/intentionally failing runs. It then seeds
610 active and 10 archived synthetic projects to test the old 500-row boundary.

The pinned Microsoft Playwright image and matching npm lockfile run Chromium
inside Linux Docker, sharing the **test API container's** network namespace.
Only the generated test token and output directory are mounted. The browser
holds no Docker socket. No native Windows browser, session or open user tab is
controlled. The test visits only its own loopback application.

Checks cover real sign-in, English default, Turkish switching, support-summary
download/privacy, mobile page overflow, server search, paging, opening settings
for the oldest project and retaining the Projects route after reload. Screenshots
and a structured summary are under `work/browser-*/artifacts/`. Review evidence
before publishing. A failed assertion produces `failure.png` and an error summary.
Cleanup stops/removes only this unique project's containers/networks; private
test configuration, archives and volumes are retained for review.

This is a focused workflow/visual smoke, not a full accessibility certification
or a substitute for restore-engine, updater and long-running acceptance tests.
