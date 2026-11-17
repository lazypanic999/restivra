# Frontend build dependency migration

The previous warning came from Vite's picomatch/fdir/tinyglobby chain. [GHSA-fc5q-39xc-vjpf](https://github.com/advisories/GHSA-fc5q-39xc-vjpf) lists no patched picomatch version as of this change. Instead of suppressing the finding or forcing an older framework version, the project removes that chain.

## Current implementation

- `esbuild` 0.28.2 is a direct, exact-version development dependency, with its integrity recorded in package-lock.json.
- `web/scripts/build.mjs` bundles React, CSS and locally served fonts. HTML references hashed external assets; the CSP needs no inline script/style exception.
- Favicons and font licenses are copied from `web/public`.
- Output cleanup is restricted to the fixed checkout-local `internal/webui/dist` physical path; redirected output directories are rejected.
- Watch rebuilds publish HTML last and preserve earlier hashed assets for already-open tabs. A normal production build cleans previous generated assets and preserves `.gitkeep`.
- There is no JavaScript development HTTP server or API proxy. `npm run dev` only watches imported files. Opt-in `restivra serve --assets-dir internal/webui/dist` serves compiled assets through the same Go server and security middleware as the API.
- Disk serving uses `os.Root` to prevent symlink access outside the compiled-assets directory, rejects dotfiles and disables directory listings. Production continues to use embedded assets unless explicitly configured otherwise.

## Verification

- Full `npm audit --json`: zero known vulnerabilities in runtime **and development** dependencies.
- `npm ls vite picomatch fdir tinyglobby --all`: empty; none of the affected chain remains in the lockfile.
- Node tests cover HTML asset wiring, missing/duplicate entry markers, rejection of external/unsafe asset references, and the existing English/Turkish behavior.
- Go tests cover disk asset updates, dotfiles, directory listings and rejection of symlink escapes; the standard API request-boundary tests remain enabled.
- Production build, Docker build, browser checks and GitHub CI verify the packaged application.

CI now runs `npm audit --audit-level=low`, including development dependencies. This reports currently known advisories; it is not a guarantee against undiscovered vulnerabilities.

## Development difference

Use two terminals with `RESTIVRA_DATABASE_URL` configured:

```sh
npm --prefix web run dev
```

```sh
go run ./cmd/restivra serve --assets-dir internal/webui/dist
```

Open `http://127.0.0.1:7474` and refresh after a successful rebuild. There is no hot module replacement. Restart the watcher for HTML template or public-file changes. The default host allowlist is unchanged; port 5173 is no longer used.
