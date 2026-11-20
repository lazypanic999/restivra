# Contributing to Restivra

Restivra currently targets developers checking trusted PostgreSQL 17 custom-format
archives on their own computers. Read the [roadmap](ROADMAP.md) and
[pilot limits](docs/PILOT-READINESS.md) before proposing a larger feature.

## Issues and pull requests

English and Turkish reports are welcome. For a reproducible bug, include the
commit, operating system, Docker version, steps and expected/actual behavior.
Use synthetic data. Never upload real dumps, `.env`, access tokens, database URLs,
private SQL or unredacted logs. Report vulnerabilities privately using
[SECURITY.md](SECURITY.md).

For substantial changes, open an issue first to agree on scope. Small fixes can
go directly to a pull request. Explain the concrete behavior change and how you
verified it. Keep each pull request focused. Match the interface rules in
[DESIGN.md](DESIGN.md), preserve English as the default and update both languages.

## Local checks

Use the Go version in `go.mod`, Node 24 and Docker with Linux containers.
Run `npm ci` in `web`, then:

```sh
npm --prefix web test
npm --prefix web run build
npm --prefix web audit --audit-level=low
go vet ./...
go test ./...
docker pull postgres:17-bookworm
RESTIVRA_INTEGRATION=1 go test -p 1 ./... -count=1 -timeout=10m
```

On PowerShell, set `$env:RESTIVRA_INTEGRATION = '1'` before the last command,
then remove it with `Remove-Item Env:RESTIVRA_INTEGRATION` afterward.
Integration tests create real disposable Docker resources; run them against a
development Docker daemon. The ordinary Go suite skips that integration fixture.
Frontend changes must build before Go embeds the production assets.

CI repeats the build, audit, vulnerability scan, vet and real PostgreSQL suite.
Use meaningful regression tests for behavior changes. Describe manual browser
checks for visual changes, including English/Turkish and a narrow viewport.

Contributions are provided under the repository's [MIT license](LICENSE).
Keep existing third-party license notices intact.
