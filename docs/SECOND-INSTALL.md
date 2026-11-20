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

Create `compose.override.yaml` in that checkout with a unique image tag for both
application services:

```yaml
services:
  api:
    image: restivra:pilot-second
  worker:
    image: restivra:pilot-second
```

The default `restivra:local` image tag is engine-wide. Separate Compose project
names isolate containers and volumes, but do not isolate image tags. A unique
tag prevents a build in one checkout from changing which application image a
later container recreation in the other checkout selects. Both services must use
the same tag. Compose automatically reads `compose.override.yaml` when no
explicit `-f` files are supplied.

```powershell
docker pull postgres:17-bookworm
docker compose up --build -d --wait --wait-timeout 180
./scripts/pilot-check.ps1 -Base http://127.0.0.1:17474
./scripts/pilot-smoke.ps1 -Base http://127.0.0.1:17474
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
