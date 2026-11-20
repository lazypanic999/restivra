# Metadata checker acceptance

The PowerShell/Docker-only checker restored an actual Restivra metadata archive
into isolated PostgreSQL 17. The valid archive contained 15 projects, 21 jobs,
21 full reports and seven attempt records. Relationship checks found no orphan
jobs or attempts. No cloned worker was started and no existing database was
replaced.

[Eight-case result](summary.json) · [Structured reports](results.json)

| Input/action | Expected outcome |
| --- | --- |
| Valid metadata, paths containing spaces | Restore/schema/relationships/cleanup pass |
| Existing report destination | Rejected; existing report hash unchanged |
| Application/customer archive | Metadata schema rejected; temporary database removed |
| Corrupt custom archive | Restore fails; temporary database removed |
| Invalid header | Rejected before starting PostgreSQL |
| Remote `DOCKER_HOST` | Rejected before archive transfer |
| Remote explicit context with a local host override | Rejected before archive transfer |
| Persistently selected remote context in isolated Docker configuration | Rejected before archive transfer |

The input archive's SHA-256 remained unchanged. Temporary databases were checked
absent after cleanup. Test-created context entries were removed; the real active
Docker context was not changed. The persistent-context test used its own private
Docker configuration directory.

Native-process checks separately passed capped stdout, discarded private stderr,
a bounded timeout and sanitized executable-start failure. CI exercises the same
metadata cases after creating a fresh authenticated installation and performing
the actual positive/negative application smoke tests.

The JSON contains fingerprints/counts and synthetic resource identifiers, not
configuration SQL, access keys, source endpoints or restored rows. This evidence
covers the host-side metadata utility; the frozen service image's longer
qualification is recorded separately.
