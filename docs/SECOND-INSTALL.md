# Run a second installation on the same Docker engine

Use a separate checkout and freshly generated private configuration. Do not copy
the running installation's `.env`, metadata volume or installation ID. Finish
these steps before the second installation's first `docker compose up`.

After running `./scripts/setup.ps1` in the new checkout, add unique values to its
private `.env` (choose another port/name if these are already in use):

```dotenv
COMPOSE_PROJECT_NAME=restivra-pilot-second
RESTIVRA_PORT=17474
```

Ready-image installations share immutable image content safely. No custom image
name or Compose override is needed. Keep the unique project and port above:

```powershell
./scripts/install.ps1
./scripts/pilot-check.ps1 -Base http://127.0.0.1:17474
./scripts/pilot-smoke.ps1 -Base http://127.0.0.1:17474
```

For a **source development** installation, the mutable `restivra:local` tag is
engine-wide. Create a local `compose.override.yaml` setting a unique `image:` for
both `api` and `worker` (for example `restivra:pilot-second`), then explicitly
include it after the build override:

```powershell
docker pull postgres:17-bookworm
docker compose -f compose.yaml -f compose.build.yaml -f compose.override.yaml up --build -d --wait --wait-timeout 180
```

Open `http://127.0.0.1:17474` and use the new checkout's own access token. Browser
cookies are scoped to host/path, not port: signing into another installation on
`127.0.0.1` can replace the first one's session cookie. Sign in again or use a
separate browser profile when switching. Independent script sessions do not
share this browser cookie.

Run later Compose commands from the intended checkout. Inspect `docker compose
ps` before stopping it. `docker compose down` preserves that project's volumes;
do not add `-v`. Preserve its `.env`, `private/sources.json` and source archives
separately from a [verified metadata backup](METADATA-CHECK.md).

To transfer settings, use [project export/import](PROJECT-TRANSFER.md). Starting
a second worker against a cloned metadata installation ID on the same engine
can make ownership-based cleanup unsafe; a metadata recovery is a replacement
installation procedure, not a way to make two independent installations.
