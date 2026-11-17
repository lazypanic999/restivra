export const messages = {
  protectedAccess: ["Sign-in required", "Oturum açma gerekli"],
  localAccess: ["Local access · sign-in disabled", "Yerel erişim · oturum açma kapalı"],
  "advice.resource_limit": ["The temporary database reached a storage or memory limit. Increase restore limits or test a smaller database.", "Geçici veritabanı depolama veya bellek sınırına ulaştı. Geri yükleme sınırlarını artır veya daha küçük bir veritabanı dene."],
  "advice.extension_unavailable": ["This archive needs an extension unavailable in the standard PostgreSQL image.", "Bu yedek standart PostgreSQL imajında bulunmayan bir eklenti gerektiriyor."],
  "advice.unsupported_version": ["Use PostgreSQL 17 and pg_dump 17 custom-format archives.", "PostgreSQL 17 ve pg_dump 17 ile alınmış custom-format yedek kullan."],
  "advice.archive_corrupt": ["Create a new completed archive and verify its transfer before retrying.", "Yeni ve tamamlanmış bir yedek al; yeniden denemeden önce aktarımını doğrula."],
  "advice.archive_invalid": ["Check that this is a completed pg_dump custom archive readable by PostgreSQL 17.", "Dosyanın PostgreSQL 17 tarafından okunabilen, tamamlanmış bir pg_dump custom yedeği olduğunu kontrol et."],
  "advice.backup_invalid": ["Check the backup path, file permissions, archive format and configured size limit.", "Yedek yolunu, dosya izinlerini, yedek formatını ve boyut sınırını kontrol et."],
  "advice.image_unavailable": ["Pull postgres:17-bookworm on the worker's Docker engine.", "Worker'ın Docker motorunda postgres:17-bookworm imajını indir."],
  "advice.database_start_failed": ["Check Docker availability and available host memory before retrying.", "Yeniden denemeden önce Docker erişimini ve sunucunun boş belleğini kontrol et."],
  "advice.restore_failed": ["Check archive integrity, required extensions and restored-data capacity. A compressed dump can expand beyond the temporary database limit.", "Yedek bütünlüğünü, gerekli eklentileri ve açılmış veri kapasitesini kontrol et. Sıkıştırılmış yedek geçici veritabanı sınırını aşabilir."],
  "advice.database_connection_failed": ["The restored database could not answer a connection check. Check resource limits and retry.", "Geri yüklenen veritabanı bağlantı kontrolüne cevap veremedi. Kaynak sınırlarını kontrol edip yeniden dene."],
  "advice.checks_failed": ["Inspect the individual SQL checks below.", "Aşağıdaki SQL kontrollerini tek tek incele."],
  "advice.verification_failed": ["Check the report stage and installation prerequisites before retrying.", "Yeniden denemeden önce rapor aşamasını ve kurulum gereksinimlerini kontrol et."],
  "advice.cleanup_failed": ["The worker stopped accepting jobs. Check Docker and private work-directory access, then restart the worker to reconcile resources.", "Worker yeni iş kabulünü durdurdu. Docker ve özel çalışma klasörü erişimini kontrol et; kaynakları toparlamak için worker'ı yeniden başlat."],
  "advice.run_timeout": ["Increase the run timeout if the archive needs more time to restore.", "Yedeğin geri yüklenmesi daha uzun sürüyorsa çalışma zaman sınırını artır."],
  "advice.cancelled": ["This run was cancelled; queue a new run when ready.", "Bu çalışma iptal edildi; hazır olduğunda yeni bir test başlat."],
  "advice.query_timeout": ["The SQL check exceeded its 10-second limit. Simplify the query or inspect the required index.", "SQL kontrolü 10 saniyelik sınırı aştı. Sorguyu sadeleştir veya gerekli indeksi incele."],
  "advice.read_only_violation": ["Checks must read data without writing or changing the database.", "Kontroller veritabanına yazmadan veya değiştirmeden veri okumalı."],
  "advice.object_missing": ["A referenced table, column or other database object is missing.", "Başvurulan tablo, sütun veya başka bir veritabanı nesnesi bulunamadı."],
  "advice.check_false": ["The query returned false. Compare the expected data with this archive.", "Sorgu false döndürdü. Beklenen veriyi bu yedekle karşılaştır."],
  "advice.invalid_check_result": ["Return exactly one non-null boolean from the check.", "Kontrolden tam olarak bir adet, null olmayan boolean döndür."],
  "advice.query_error": ["Check the query and object permissions. Raw SQL errors are withheld to protect restored data.", "Sorguyu ve nesne izinlerini kontrol et. Geri yüklenen veriyi korumak için ham SQL hataları gösterilmez."],
  signIn: ["Sign in", "Oturum aç"],
  signingIn: ["Signing in…", "Oturum açılıyor…"],
  signOut: ["Sign out", "Oturumu kapat"],
  signInDescription: ["Access this installation's backup verification records.", "Bu kurulumun yedek doğrulama kayıtlarına eriş."],
  accessToken: ["Access token", "Erişim anahtarı"],
  accessTokenHelp: ["Use RESTIVRA_AUTH_TOKEN from the installation's private .env file. Your browser session lasts eight hours.", "Kurulumun özel .env dosyasındaki RESTIVRA_AUTH_TOKEN değerini kullan. Tarayıcı oturumu sekiz saat sürer."],
  authInvalid: ["The access token is incorrect.", "Erişim anahtarı yanlış."],
  authRateLimit: ["Too many attempts. Wait one minute before trying again.", "Çok fazla deneme yapıldı. Yeniden denemeden önce bir dakika bekle."],
  authUnavailable: ["Could not connect. Check the service and refresh this page.", "Bağlantı kurulamadı. Servisi kontrol edip sayfayı yenile."],
  title: ["Restivra — Backup verification", "Restivra — Yedek doğrulama"],
  overview: ["Overview", "Genel bakış"],
  projects: ["Projects", "Projeler"],
  history: ["Runs", "Çalışmalar"],
  setup: ["Setup", "Kurulum"],
  language: ["Interface language", "Arayüz dili"],
  refresh: ["Refresh", "Yenile"],
  heading: ["Backup verification", "Yedek doğrulama"],
  description: [
    "Restore tests for your PostgreSQL archives.",
    "PostgreSQL yedeklerin için geri yükleme testleri.",
  ],
  projectsDescription: [
    "Backup sources, checks and daily schedules.",
    "Yedek kaynakları, kontroller ve günlük zamanlamalar.",
  ],
  historyDescription: [
    "Inspect the latest 100 verification runs.",
    "Son 100 doğrulama çalışmasını incele.",
  ],
  setupDescription: [
    "Connect a completed archive and run your first test.",
    "Tamamlanmış yedeğini tanımla ve ilk testi çalıştır.",
  ],
  newProject: ["New project", "Yeni proje"],
  workerOnline: ["Worker online", "Worker bağlı"],
  workerOffline: ["Worker offline", "Worker bağlı değil"],
  databaseOnline: ["Database connected", "Veritabanı bağlı"],
  lastUpdated: ["Updated {time}", "Güncelleme: {time}"],
  offlineNotice: [
    "Queued tests will wait until the worker reconnects.",
    "Sıradaki testler worker yeniden bağlanana kadar bekleyecek.",
  ],
  requestFailed: [
    "The request could not be completed.",
    "İstek tamamlanamadı.",
  ],
  connectionError: ["Connection error", "Bağlantı hatası"],
  loading: ["Loading verification records…", "Doğrulama kayıtları yükleniyor…"],
  summary: [
    "{projects} projects / {passed} passed / {failed} failed / {active} active",
    "{projects} proje / {passed} başarılı / {failed} başarısız / {active} aktif",
  ],
  recentWindow: [
    "Results refer to the latest {count} runs.",
    "Sonuçlar son {count} çalışmaya aittir.",
  ],
  source: ["Backup source", "Yedek kaynağı"],
  project: ["Project", "Proje"],
  latestResult: ["Latest result", "Son sonuç"],
  lastRun: ["Last run", "Son çalışma"],
  nextRun: ["Next scheduled run", "Sonraki planlı çalışma"],
  actions: ["Actions", "İşlemler"],
  notTested: ["Not tested", "Test edilmedi"],
  manualOnly: ["Manual only", "Yalnızca manuel"],
  runTest: ["Run test", "Testi çalıştır"],
  edit: ["Edit", "Düzenle"],
  queuedToast: ["Verification queued.", "Doğrulama sıraya alındı."],
  savedToast: ["Project saved.", "Proje kaydedildi."],
  archivedToast: [
    "Project archived. Its run history is retained.",
    "Proje arşivlendi. Çalışma geçmişi saklanıyor.",
  ],
  cancelledToast: [
    "Cancellation requested. Temporary resources are being removed.",
    "İptal istendi. Geçici kaynaklar kaldırılıyor.",
  ],
  noProjects: ["No backup sources yet", "Henüz yedek kaynağı yok"],
  noProjectsText: [
    "Add a project with the path to a completed PostgreSQL 17 dump.",
    "Tamamlanmış PostgreSQL 17 yedeğinin yolunu bir projeyle tanımla.",
  ],
  noMatches: [
    "No records match your filters.",
    "Filtrelerine uygun kayıt bulunamadı.",
  ],
  recentRuns: ["Recent verification runs", "Son doğrulama çalışmaları"],
  allRuns: ["View all runs", "Tüm çalışmaları aç"],
  noRuns: ["No verification runs yet", "Henüz doğrulama çalışması yok"],
  noRunsText: [
    "Run a project to record its restore and check results here.",
    "Geri yükleme ve kontrol sonuçlarını burada görmek için bir proje çalıştır.",
  ],
  searchProjects: ["Search projects", "Proje ara"],
  searchRuns: ["Search runs by project", "Çalışmalarda proje ara"],
  statusFilter: ["Filter by result", "Sonuca göre filtrele"],
  allResults: ["All results", "Tüm sonuçlar"],
  runID: ["Run ID", "Çalışma kimliği"],
  started: ["Started", "Başlangıç"],
  result: ["Result", "Sonuç"],
  duration: ["Duration", "Süre"],
  trigger: ["Trigger", "Tetikleyici"],
  inspect: ["Inspect", "İncele"],
  manual: ["Manual", "Manuel"],
  scheduled: ["Schedule", "Zamanlama"],
  seconds: ["s", "sn"],
  "status.passed": ["Passed", "Başarılı"],
  "status.failed": ["Failed", "Başarısız"],
  "status.running": ["Running", "Çalışıyor"],
  "status.queued": ["Queued", "Sırada"],
  "status.cancelled": ["Cancelled", "İptal edildi"],
  "status.interrupted": ["Interrupted", "Yarıda kaldı"],
  "status.not_run": ["Not run", "Çalıştırılmadı"],
  "status.error": ["Query error", "Sorgu hatası"],
  "status.not_needed": ["Not required", "Gerekmedi"],
  selfHosted: ["Self-hosted", "Kendi sunucunda"],
  localTime: [
    "Times use your device time zone.",
    "Saatler cihazının saat dilimine göredir.",
  ],
  close: ["Close", "Kapat"],
  dismiss: ["Dismiss message", "Bildirimi kapat"],
  cancel: ["Cancel", "Vazgeç"],
  projectSettings: ["Project settings", "Proje ayarları"],
  createProject: ["Create project", "Proje oluştur"],
  projectName: ["Project name", "Proje adı"],
  namePlaceholder: ["e.g. Northwind storefront", "Örn. Kuzey mağaza"],
  backupPattern: ["Backup file or pattern", "Yedek dosyası veya eşleşmesi"],
  patternHint: [
    "Relative to the backup root. If several files match, the newest modification time wins.",
    "Yedek köküne göre bağıl yol. Birden fazla eşleşmede en yeni değiştirilme zamanı seçilir.",
  ],
  supportedFormat: [
    "PostgreSQL 17 / pg_dump 17 / custom format (-Fc)",
    "PostgreSQL 17 / pg_dump 17 / custom format (-Fc)",
  ],
  scheduling: ["Scheduling", "Zamanlama"],
  schedulingHelp: ["Tests run while Docker and this computer are running. After downtime, one overdue test is queued; missed days do not create a backlog.", "Testler Docker ve bu bilgisayar açıkken çalışır. Kapalı kaldıktan sonra gecikmiş bir test sıraya alınır; kaçırılan günler iş yığını oluşturmaz."],
  daily: ["Run automatically every day", "Her gün otomatik çalıştır"],
  time: ["Time", "Saat"],
  timezone: ["Time zone", "Saat dilimi"],
  checks: ["Data checks", "Veri kontrolleri"],
  optional: ["Optional", "İsteğe bağlı"],
  checkHint: [
    "A single SELECT returning one non-null boolean. No semicolons or SQL comments.",
    "NULL olmayan tek bir boolean döndüren SELECT. Noktalı virgül ve SQL yorumu kullanma.",
  ],
  checkName: ["Check name", "Kontrol adı"],
  sql: ["SQL query", "SQL sorgusu"],
  addCheck: ["Add check", "Kontrol ekle"],
  removeCheck: ["Remove check {number}", "{number}. kontrolü kaldır"],
  limits: ["Resource limits", "Kaynak sınırları"],
  totalTimeout: ["Total timeout (seconds)", "Toplam süre sınırı (saniye)"],
  dataLimit: ["Restored data (MiB)", "Geri yüklenen veri (MiB)"],
  memoryLimit: ["Memory (MiB)", "Bellek (MiB)"],
  archiveLimit: ["Maximum archive (MiB)", "En büyük yedek (MiB)"],
  saveProject: ["Save project", "Projeyi kaydet"],
  saving: ["Saving…", "Kaydediliyor…"],
  archiveProject: ["Archive project", "Projeyi arşivle"],
  archiveQuestion: [
    "Archive this project? Its run history will be retained.",
    "Bu proje arşivlensin mi? Çalışma geçmişi saklanacak.",
  ],
  confirmArchive: ["Confirm archive", "Arşivlemeyi onayla"],
  report: ["Verification report", "Doğrulama raporu"],
  reportPassed: [
    "The archive restored and all configured checks passed.",
    "Yedek geri yüklendi ve tanımlı kontroller geçti.",
  ],
  reportRunning: [
    "The result updates when verification finishes.",
    "Doğrulama tamamlandığında sonuç güncellenecek.",
  ],
  reportFailed: [
    "Verification did not complete successfully.",
    "Doğrulama başarıyla tamamlanamadı.",
  ],
  stage: ["Stage / check", "Aşama / kontrol"],
  details: ["Details", "Ayrıntılar"],
  restore: ["Archive restore", "Yedeği geri yükleme"],
  cleanup: ["Temporary resource cleanup", "Geçici kaynak temizliği"],
  noChecks: [
    "No custom data checks were configured.",
    "Özel veri kontrolü tanımlanmamış.",
  ],
  archive: ["Tested archive", "Test edilen yedek"],
  file: ["File", "Dosya"],
  size: ["Size", "Boyut"],
  modified: ["File modified", "Dosya değiştirilme zamanı"],
  fingerprint: ["SHA-256 fingerprint", "SHA-256 parmak izi"],
  copyFingerprint: ["Copy fingerprint", "Parmak izini kopyala"],
  copied: ["Copied", "Kopyalandı"],
  copyFailed: [
    "Copy was blocked by the browser.",
    "Kopyalama tarayıcı tarafından engellendi.",
  ],
  scope: ["Scope and warnings ({count})", "Kapsam ve uyarılar ({count})"],
  noReport: [
    "This run ended before a restore report was produced.",
    "Bu çalışma geri yükleme raporu üretilmeden sonlandı.",
  ],
  notification: ["Notification", "Bildirim"],
  notificationSkipped: [
    "Not configured or not required",
    "Yapılandırılmadı veya gerekmedi",
  ],
  notificationPending: ["Delivery pending", "Teslim bekliyor"],
  notificationDelivered: ["Delivered", "Teslim edildi"],
  notificationFailed: ["Delivery failed", "Teslim edilemedi"],
  attempts: ["{count} attempts", "{count} deneme"],
  cancelTest: ["Cancel run", "Çalışmayı iptal et"],
  cancelling: ["Cancellation requested", "İptal istendi"],
  download: ["Download JSON", "JSON indir"],
  scopeNote: [
    "A passing result verifies a logical restore and selected checks. Application recovery, original permissions and backup freshness need separate validation.",
    "Başarılı sonuç mantıksal geri yüklemeyi ve seçili kontrolleri doğrular. Uygulama kurtarması, orijinal yetkiler ve yedek güncelliği ayrıca sınanmalıdır.",
  ],
  setupTitle1: [
    "Put a completed archive in the backup directory",
    "Tamamlanmış yedeği yedek klasörüne koy",
  ],
  setupText1: [
    "Use pg_dump 17 against PostgreSQL 17 with custom format. In a Compose installation, place the file in backups/.",
    "PostgreSQL 17 için pg_dump 17 ile custom format kullan. Compose kurulumunda dosyayı backups/ klasörüne koy.",
  ],
  setupTitle2: ["Add a project and its checks", "Proje ve kontrollerini ekle"],
  setupText2: [
    "Use a path relative to the backup root. Selects must return a single boolean; leave off the final semicolon.",
    "Yedek köküne göre bağıl yol kullan. SELECT sorgusu tek boolean döndürmeli; sonuna noktalı virgül koyma.",
  ],
  setupTitle3: [
    "Run verification and read the report",
    "Doğrulamayı çalıştır ve raporu oku",
  ],
  setupText3: [
    "Restivra restores into a temporary PostgreSQL container, runs the checks, and removes that environment. Start manually or configure a daily schedule.",
    "Restivra geçici PostgreSQL konteynerine geri yükler, kontrolleri çalıştırır ve ortamı kaldırır. Manuel başlat veya günlük saat belirle.",
  ],
  installation: ["Installation", "Kurulum"],
  shell: ["Shell", "Kabuk"],
  workerHelp: [
    "If tests remain queued, inspect the worker logs.",
    "Testler sırada kalıyorsa worker günlüklerini incele.",
  ],
  webhookHelp: [
    "Optional failure webhook: set RESTIVRA_WEBHOOK_URL in .env and recreate the worker. The receiver must accept generic JSON over public HTTPS.",
    "İsteğe bağlı başarısızlık bildirimi: .env içinde RESTIVRA_WEBHOOK_URL tanımlayıp worker’ı yeniden oluştur. Alıcı, herkese açık HTTPS üzerinden genel JSON kabul etmeli.",
  ],
  documented: ["Technical limits", "Teknik sınırlar"],
  skipContent: ["Skip to verification records", "Doğrulama kayıtlarına geç"],
};

export function readLanguage(storage) {
  try {
    return storage?.getItem("restivra.language") === "tr" ? "tr" : "en";
  } catch {
    return "en";
  }
}
export function text(language, key, values = {}) {
  const pair = messages[key];
  if (!pair) throw new Error(`Missing interface message: ${key}`);
  return pair[language === "tr" ? 1 : 0].replace(/\{(\w+)\}/g, (_, name) =>
    String(values[name] ?? `{${name}}`),
  );
}

const errorPairs = [
  ["Temporary resource cleanup failed. Worker stopped before accepting more jobs.", "Geçici kaynaklar temizlenemedi. Worker yeni iş kabul etmeden durdu."],
  ["worker heartbeat timed out; active restore was stopped", "worker bağlantı kontrolünün süresi doldu; aktif geri yükleme durduruldu"],
  ["Failed to fetch", "Sunucuya bağlanılamadı"],
  ["NetworkError when attempting to fetch resource.", "Sunucuya bağlanılamadı"],
  ["The request could not be completed.", "İstek tamamlanamadı."],
  ["invalid project ID", "geçersiz proje kimliği"],
  ["invalid run ID", "geçersiz çalışma kimliği"],
  ["endpoint not found", "uç nokta bulunamadı"],
  ["host is not allowed", "host izin listesinde değil"],
  ["cross-origin access denied", "farklı kaynaktan erişim reddedildi"],
  [
    "invalid session token; refresh the page",
    "oturum belirteci geçersiz; sayfayı yenileyin",
  ],
  ["application/json is required", "application/json gerekli"],
  [
    "no restore report is available for this run yet",
    "bu çalışma için henüz geri yükleme raporu yok",
  ],
  ["database unavailable", "veritabanı erişilemiyor"],
  ["exactly one JSON object is required", "tek JSON nesnesi gerekli"],
  [
    "invalid JSON or unsupported field",
    "geçersiz JSON veya desteklenmeyen alan",
  ],
  ["record not found", "kayıt bulunamadı"],
  [
    "this project already has a queued or running test",
    "bu projenin bekleyen veya çalışan bir testi var",
  ],
  ["request could not be completed", "işlem tamamlanamadı"],
  [
    "project name must be between 1 and 120 bytes",
    "proje adı 1–120 bayt olmalı",
  ],
  [
    "backup path must be relative to the backup root; example: customer/*.dump",
    "yedek yolu, yedek köküne göre bağıl bir yol olmalı; örnek: musteri/*.dump",
  ],
  ["parent directory access is not allowed", "üst klasörlere erişim yasak"],
  ["invalid backup pattern", "geçersiz dosya eşleşmesi"],
  ["invalid time zone", "geçersiz saat dilimi"],
  ["schedule must use HH:MM format", "zamanlama HH:MM biçiminde olmalı"],
  [
    "could not calculate the next run time",
    "sonraki çalışma saati hesaplanamadı",
  ],
  [
    "timeout_seconds must be between 5 and 3600",
    "timeout_seconds 5 ile 3600 arasında olmalı",
  ],
  [
    "max_backup_bytes must be between 5 and 10737418240",
    "max_backup_bytes 5 ile 10737418240 arasında olmalı",
  ],
  [
    "data_mb must be between 128 and 8192",
    "data_mb 128 ile 8192 arasında olmalı",
  ],
  [
    "memory_mb must be at least data_mb + 256, and at most 16384",
    "memory_mb en az data_mb + 256, en fazla 16384 olmalı",
  ],
  ["at most 32 checks are supported", "en fazla 32 kontrol desteklenir"],
  [
    "check names must be unique, nonempty and at most 120 bytes",
    "kontrol adları benzersiz, dolu ve en fazla 120 bayt olmalı",
  ],
  [
    "This verifies a logical restore and selected checks, not full application recovery, ownership, grants or point-in-time recovery.",
    "Bu test mantıksal geri yükleme ve seçili kontrolleri doğrular; tam uygulama kurtarması, sahiplik, yetkiler ve belirli bir zamana kurtarmayı doğrulamaz.",
  ],
  [
    "File modification time is not a verified backup creation time. Compare SHA-256 values across runs to detect unchanged backups.",
    "Dosya değiştirilme zamanı doğrulanmış yedek oluşturma zamanı değildir. Değişmeyen yedekleri görmek için çalışmaların SHA-256 değerlerini karşılaştır.",
  ],
  [
    "The tested backup is identical to the previous run; backup freshness was not established.",
    "Test edilen yedek önceki çalışmayla aynı; yedeğin güncelliği doğrulanmadı.",
  ],
  [
    "The backup file modification time is over 24 hours old; this is not a verified backup creation timestamp.",
    "Yedek dosyasının değiştirilme zamanı 24 saatten eski; bu, doğrulanmış yedek oluşturma zamanı değildir.",
  ],
  [
    "No custom checks configured; application data was not validated.",
    "Özel kontrol tanımlanmamış; uygulama verisi doğrulanmadı.",
  ],
  [
    "query failed or exceeded its time limit",
    "sorgu başarısız oldu veya süre sınırını aştı",
  ],
  [
    "query must return exactly one non-null boolean",
    "sorgu NULL olmayan tek bir boolean döndürmeli",
  ],
  [
    "one or more verification checks did not pass",
    "bir veya daha fazla doğrulama kontrolü geçmedi",
  ],
  ["run exceeded its time limit", "çalışma süre sınırını aştı"],
  ["run was cancelled", "çalışma iptal edildi"],
  [
    "archive is unreadable by PostgreSQL 17 pg_restore",
    "yedek PostgreSQL 17 pg_restore tarafından okunamıyor",
  ],
  [
    "pg_restore failed; archive may be corrupt, require unavailable extensions, or exceed resource limits",
    "pg_restore başarısız; yedek bozuk olabilir, bulunmayan eklentiler gerektirebilir veya kaynak sınırını aşabilir",
  ],
  [
    "No completed backup matched the configured path inside the backup root.",
    "Yedek kökündeki tanımlı yolla eşleşen tamamlanmış yedek bulunamadı.",
  ],
  ["Cancelled by user.", "Kullanıcı tarafından iptal edildi."],
  [
    "Worker stopped before completing this run.",
    "Worker bu çalışmayı tamamlamadan durdu.",
  ],
  [
    "Previous worker stopped before finishing. Temporary resources were reconciled.",
    "Önceki worker tamamlanmadan durdu. Geçici kaynaklar toparlandı.",
  ],
  [
    "worker heartbeat lost; active restore was stopped",
    "worker bağlantısı kayboldu; aktif geri yükleme durduruldu",
  ],
  [
    "Private work directory cleanup failed.",
    "Özel çalışma klasörü temizlenemedi.",
  ],
  [
    "Docker engine unavailable; start Docker with Linux containers",
    "Docker motoruna erişilemiyor; Linux konteyner motorunu başlat",
  ],
  [
    "only PostgreSQL 17 backups created with pg_dump 17 are supported",
    "yalnızca pg_dump 17 ile alınan PostgreSQL 17 yedekleri destekleniyor",
  ],
  [
    "could not create isolated PostgreSQL container",
    "izole PostgreSQL konteyneri oluşturulamadı",
  ],
  [
    "could not start PostgreSQL container",
    "PostgreSQL konteyneri başlatılamadı",
  ],
  ["PostgreSQL did not become ready", "PostgreSQL hazır hale gelmedi"],
  [
    "could not create restricted verification role",
    "kısıtlı doğrulama rolü oluşturulamadı",
  ],
  [
    "restored database connection check failed",
    "geri yüklenen veritabanının bağlantı kontrolü başarısız",
  ],
  ["could not open backup file", "yedek dosyası açılamadı"],
  ["backup must be a regular file", "yedek normal bir dosya olmalı"],
  ["backup exceeds max_backup_bytes", "yedek max_backup_bytes sınırını aşıyor"],
  ["could not create private snapshot", "özel yedek kopyası oluşturulamadı"],
  ["could not snapshot backup", "yedek kopyalanamadı"],
  [
    "backup grew or shrank while being copied",
    "yedek kopyalanırken boyutu değişti",
  ],
  [
    "backup changed while being copied; use an atomically completed backup",
    "yedek kopyalanırken değişti; tamamlandıktan sonra atomik olarak taşınmış yedek kullan",
  ],
  [
    "expected a pg_dump custom archive (PGDMP header)",
    "pg_dump custom yedeği bekleniyordu (PGDMP başlığı)",
  ],
  ["could not generate run ID", "çalışma kimliği üretilemedi"],
  ["could not create work directory", "çalışma klasörü oluşturulamadı"],
  ["could not rewind backup snapshot", "yedek kopyası yeniden okunamadı"],
  [
    "could not remove private backup snapshot; remove it from work directory",
    "özel yedek kopyası silinemedi; çalışma klasöründen kaldır",
  ],
];

export function translateMessage(message, language) {
  if (!message) return "";
  const pair = errorPairs.find((pair) => pair.includes(message));
  if (pair) return pair[language === "tr" ? 1 : 0];
  if (language === "tr") {
    const check = message.match(
      /^check (".*") must be a single SELECT without semicolons, comments or backslashes$/,
    );
    if (check)
      return `${check[1]} kontrolü noktalı virgül, yorum veya ters eğik çizgi içermeyen tek SELECT olmalı`;
    if (message.startsWith("PostgreSQL image missing; run: "))
      return `PostgreSQL imajı yok; çalıştır: ${message.slice("PostgreSQL image missing; run: ".length)}`;
    if (message.startsWith("container cleanup failed; use cleanup --run-id "))
      return `Konteyner temizlenemedi; kullan: cleanup --run-id ${message.split(" ").at(-1)}`;
  }
  return message;
}
