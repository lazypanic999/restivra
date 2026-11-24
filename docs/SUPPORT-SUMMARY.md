# System checks and support summaries

Open **Setup → System check** after signing in. It shows the actual application
build, metadata connectivity, this installation's worker heartbeat, queued work,
deferred retries, running jobs and schedules overdue by more than a minute.
The view refreshes every 15 seconds while open. It does not enqueue, cancel,
restore, migrate or delete anything. A failed refresh clears the old status
instead of leaving an old green result on screen.

**Download support summary** saves `restivra-support.json` locally. Nothing is
uploaded automatically. Review it before sharing in a support conversation.
It contains only build identity, platform/toolchain, authentication enabled/disabled,
metadata PostgreSQL major version, heartbeat age, aggregate counts and fixed
diagnostic codes. It excludes project names/IDs, SQL checks, archive paths,
source profiles, storage keys, hostnames, logs, connection strings and credentials.
Counts can still reveal the size/activity of an installation; review accordingly.

When metadata is unavailable, the download remains useful: database status is
`unavailable`, worker status is `unknown` and counts are `null`. Unknown counts
must not be interpreted as zero. A heartbeat from another instance does not make
this worker online; timestamps substantially in the future are reported as
`clock_skew`. A short overdue interval can be ordinary queue delay rather than
data loss. Computers that are asleep cannot run daily tests.

The authenticated endpoints are `GET /api/support` and
`GET /api/support/download`. Both require the existing operator session and
same-origin/host protection. Legacy installations with authentication disabled
remain loopback-only as described in the [runbook](PILOT-RUNBOOK.md).
Database work is a bounded, read-only transaction. Database error messages and
private settings are never copied into the summary.

`restivra version` prints the same public build identity as JSON. Published image
builds inject their version and source revision; local builds say `development`.
The tool cannot infer a published release from a mutable local Docker tag.
The support check verifies prerequisites and workload state, not whether every
customer backup restores successfully. Use the pilot smoke and meaningful SQL
checks for actual restore evidence.

## Türkçe

Oturum açtıktan sonra **Kurulum → Sistem kontrolü** bölümünü aç. Çalışan sürümü,
veritabanı/worker durumunu, bekleyen işleri, yeniden denemeleri ve geciken
zamanlamaları görebilirsin. **Destek özetini indir** yalnızca bilgisayarına JSON
dosyası indirir; otomatik gönderim yapılmaz.

Dosyada sürüm, sağlık durumu ve toplu sayılar vardır. Proje adları/kimlikleri,
SQL, yedek yolları, depolama ayarları, günlükler ve erişim anahtarları bulunmaz.
Paylaşmadan önce incele. Veritabanı kontrol edilemiyorsa sayılar `null` olur;
bu, sıfır iş olduğu anlamına gelmez. Bu tanı ekranı gerçek geri yükleme testinin
yerini almaz.
