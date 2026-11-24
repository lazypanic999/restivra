# S3-compatible backup sources

Restivra can verify an existing PostgreSQL 17 custom archive directly from an S3
bucket. The worker lists matching objects, selects the newest completed match,
downloads one bounded private snapshot and runs the same isolated restore/checks.
It does not create a bucket, upload a backup, delete an object or manage retention.
Keep your backup producer and independent recovery procedure.

## Configure a worker profile

Run the setup script once after updating. It creates `private/sources.json` if
missing, and preserves an existing file and `.env`. This directory is excluded
from Git and Docker's build context. Compose mounts only the profile file
read-only **in the worker**. Deployment records and checkpoints are excluded.
The API receives no storage credentials or storage mount.

Edit this file privately, replacing the placeholders with your operator-managed
read-only credentials. Do not put real keys in project settings or public issues.

```json
{
  "s3": [
    {
      "id": "offsite",
      "name": "Offsite backups",
      "endpoint": "",
      "region": "eu-central-1",
      "access_key_id": "REPLACE_WITH_READ_ONLY_ACCESS_KEY",
      "secret_access_key": "REPLACE_WITH_READ_ONLY_SECRET",
      "path_style": false
    }
  ]
}
```

An empty endpoint uses the AWS regional endpoint. For compatible storage, set its
HTTPS endpoint and region, and enable `path_style` if your provider requires
bucket names in the URL path. The endpoint may not contain credentials, a path,
query or fragment. An optional `session_token` supports temporary credentials,
but automatic credential renewal and ambient AWS credential chains are not
implemented. Replace expiring credentials privately and recreate the idle worker.

For storage on a trusted private network, explicit `"allow_http": true` permits
an HTTP endpoint such as `http://storage:9000`. This sends requests without TLS;
use HTTPS for public endpoints. TLS certificate validation cannot be disabled.
HTTP proxies and redirects are disabled; configure the correct endpoint/region.
Project users cannot supply endpoints or credentials through the API.

Wait until no job is active, then:

```sh
docker compose up --no-build --no-deps --pull never --force-recreate -d worker
docker compose logs --tail=30 worker
```

Recreation binds the current file even if your editor replaced its inode while
saving. A simple restart can keep the old file. See [private files](PRIVATE-FILES.md)
for permissions and compatible upgrades from earlier directory mounts.

The project form lists the profile's public name. Only its ID/name are advertised
in the heartbeat. The database stores the selection (kind/profile/bucket/pattern),
never keys, session tokens or endpoints. A missing optional profile file keeps
legacy local-only installs working. An invalid present file stops worker startup
with a sanitized error. Profile IDs must be unique, lowercase and match
`[a-z][a-z0-9_-]{0,31}`; at most 20 profiles are supported.

## Give the credential read-only access

For AWS, a starting IAM policy for one bucket/prefix is:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": ["s3:ListBucket"],
      "Resource": "arn:aws:s3:::your-backups",
      "Condition": {"StringLike": {"s3:prefix": ["daily/*", "daily/"]}}
    },
    {
      "Effect": "Allow",
      "Action": ["s3:GetObject", "s3:GetObjectVersion"],
      "Resource": "arn:aws:s3:::your-backups/daily/*"
    }
  ]
}
```

Adapt the bucket/prefix to the exact listing prefixes your project uses. The
worker does not need ListAllMyBuckets, PutObject, DeleteObject or administrator
permissions. Provider-specific KMS-encrypted objects may require a narrowly
scoped decrypt permission; encryption-key setup is outside this integration.
Credential policies remain your provider's responsibility.

## Create a project

Choose **S3-compatible storage**, the profile, a bucket and an object key pattern,
for example `daily/*.dump`. Credentials stay outside the form. Add meaningful
single-boolean SQL checks, run manually, inspect cleanup, then enable a schedule.

Patterns use case-sensitive path matching: `*` matches within one path segment,
not recursively. The fixed prefix before the first wildcard narrows server-side
listing. Among matching objects, newest Last-Modified wins; lexical key order
breaks ties. Directory marker objects are ignored. A `.partial` key does not
match `*.dump`; choose a pattern that selects only finalized backup objects.
At most 10,000 objects and 100 list pages may be examined. Narrow the fixed
prefix when that bound is exceeded; no partial-list winner is silently selected.

Version 1 of this source integration lists current object versions, rather than
searching a bucket's historical version history. When HEAD supplies a non-null
version ID, that exact version is requested. Otherwise conditional `If-Match`
binds the download to the selected ETag. Listing, HEAD and GET metadata must agree;
HTTP dates use whole-second precision, unlike some listing timestamps.
Replacement during selection/download fails explicitly. Prefer immutable backup
keys to constantly overwriting `latest.dump`.

The archive byte limit is checked before download and enforced during copying.
The same per-attempt verification deadline includes source selection/download.
One private snapshot is hashed and checked for a PostgreSQL custom header before
Docker starts. The downloaded SHA-256 is the content fingerprint: **ETag is not
assumed to be MD5**, and Last-Modified is not proof of backup creation or data
freshness. An identical archive can still contain stale data; check application
timestamps with SQL if freshness matters.

## Results, failures and privacy

Reports include profile ID, bucket/key, returned version ID, ETag and computed
SHA-256. History/CSV include source metadata; exports append six source columns
after the original fields. No provider URL, access key or raw SDK diagnostic is
retained. Bucket/key names and reports are still operational metadata; review
them before sharing. Each queued job snapshots its selection, so later project
edits cannot redirect an already queued run.

The snapshot contains the profile **ID**, bucket and pattern, not a frozen copy
of the private connection file. Profiles are resolved when the worker starts.
Use a new profile ID when changing its endpoint/account, retain the old profile
until its queued work finishes, and restart only while idle. Rotating a key or
redefining a profile requires reviewing its access separately; the source
metadata does not prove the provider account's identity.

- `source_access_denied`: repair credentials or read permissions; no auto retry.
- `source_not_found`: check bucket/key pattern and completed uploads.
- `source_changed`: use immutable finalized objects, then start a new run.
- `source_too_large`: inspect size/capacity before changing project limits.
- `source_selection_limit`: narrow the literal key prefix.
- `source_configuration`: correct endpoint/region or provider metadata support.
- `source_profile_missing`: add the profile, then recreate the idle worker using
  the command above so an atomically replaced file is mounted again.
- `source_download_failed`: incomplete or invalid archive snapshot; inspect source.
- `source_unavailable`: transient request or HTTP body failure; the existing bounded 3-attempt
  infrastructure retry policy applies. Cancelled work does not retry.

An interrupted HTTP body discards its partial private snapshot before retrying.
It is distinguished from a fully downloaded invalid archive and local snapshot
write errors, which remain final `source_download_failed` outcomes. Every retry
does a fresh selection and preserves prior attempt evidence; use immutable object
keys if a later attempt must target unchanged source content.

This is validated with the actual AWS SDK v2 against a real pinned
`rustfs/rustfs:1.0.5` S3-compatible test server and real PostgreSQL 17 restores,
including conditional-overwrite rejection and bad credentials. It does not imply
that every AWS region, R2/B2/other provider, encryption mode or private TLS setup
has had an external pilot. RustFS is test infrastructure, not a required service
or a bundled production dependency.

References: [AWS endpoint configuration](https://docs.aws.amazon.com/sdk-for-go/v2/developer-guide/configure-endpoints.html),
[conditional GetObject](https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html),
[RustFS container documentation](https://docs.rustfs.com/en/installation/container).
