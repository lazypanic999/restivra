# Security policy

Restivra is an early local developer tool. Security fixes target the latest
`main` revision; there is no maintained older release branch or response SLA.
Keep Docker, PostgreSQL and Restivra updated.

## Report a vulnerability privately

Use GitHub's [private vulnerability report form](https://github.com/lazypanic999/restivra/security/advisories/new).
Include the affected commit, a minimal reproduction using synthetic data,
expected impact and any relevant sanitized diagnostics. English and Turkish are
welcome. Do not open a public issue with exploit details, credentials or dumps.

## Deployment boundary

- The supported pilot binds the web UI to localhost and processes trusted,
  operator-provided PostgreSQL 17 custom-format archives.
- Temporary restores use containers, which are not a hostile-input sandbox.
  Do not offer arbitrary public dump uploads.
- The worker controls the Docker socket; access to that socket is privileged.
  The API service does not receive the socket or backup directory.
- Keep `.env`, source dumps, metadata exports, report downloads and private work
  files out of public issues and version control.
- Public exposure, untrusted tenants, arbitrary extensions and hosted SaaS are
  outside the validated pilot boundary.

See the [pilot runbook](docs/PILOT-RUNBOOK.md) for operating and recovery steps.
