# Interrupted S3 download — real application regression

A private test proxy interrupted the body of a real RustFS `GetObject` response
after eight bytes, while retaining its declared 1,911-byte Content-Length. The
worker used the actual AWS SDK, persisted queue, conditional/versioned download,
private snapshot and PostgreSQL 17 restore path. No cloud account was involved.

| Phase | Final state | Attempts | Evidence |
| --- | --- | --- | --- |
| Before the fix | Failed | 1 | [`source_download_failed`](before.json) |
| After the fix | Passed | 2 | [Preserved `source_unavailable`, then successful restore/cleanup](after.json) |

Both phases selected the same version
`944ec807-4f57-44ae-a17a-8629d0ea72c8` of the synthetic object. The successful
snapshot's SHA-256 was
`ea490c5bec0d9c7d1c987bc480d7a4f35054b6c570f93bfb93e256091f0e6d26`.
Proxy counters show one dropped request in each phase; the fixed phase made a
fresh successful GET after its bounded backoff, without a manual rerun.

The implementation remembers only whether the input reader failed, never its
raw diagnostic. Context cancellation still wins and does not retry. Complete
invalid archives and private snapshot I/O failures remain final failures.
The HTTP regression test also checks removal of the partial file and the hash
of a complete fresh retry. Existing cancellation and invalid-archive tests remain.

The first drill briefly used the earlier synthetic store. The later long
qualification has its own persistent RustFS volume and version ID, so its
storage restarts cannot interfere with the earlier fixed-image S3 soak.
These tests do not establish compatibility with every cloud/provider or full
application recovery. [Source behavior and limits](../../S3-SOURCES.md).

An unrelated integration-test setup race was also isolated: a cancelled real
Docker create could reserve its name between `ps` and a synthetic late-create
injection. The late-resource assertion now uses a separate terminal fixture job,
while the real running-cancellation assertion remains. Ten consecutive real
application integration runs passed in 452.958 seconds after that change.
Three empty test containers left by the earlier failed harness runs were checked
against their exact IDs, creation times and `restivra-app-test-*` ownership before
removal. No main-installation resource was removed.
