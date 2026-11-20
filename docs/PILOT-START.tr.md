# Restivra: ilk geliştirici pilotu

Bu rehberi katılımcıya gönder. Pilot sürüm: **`v0.1.0-pilot.3`**.
Hedef: geliştiricinin kendi bilgisayarında bir PostgreSQL 17 yedeğini gerçekten
geri yükleyip uygulamaya özel kontrollerle doğrulaması, ardından bir hafta izlemesi.
Veritabanı dosyaları bize gönderilmez; işlem katılımcının bilgisayarında yapılır.

## Başlamadan önce

- İlk desteklenen yol: Windows, **PowerShell 7.2+ (`pwsh`)**, Git ve Docker
  Desktop'ın çalışan **Linux/amd64 containers** motoru. Windows PowerShell 5.1 uygun değil.
- Kurulumda Go, Node veya yerel PostgreSQL kurman gerekmiyor; yayımlanmış, içerik özeti sabitlenmiş imaj indirilir.
  İlk imaj indirmeleri için internet gerekiyor. Kurulum süresi bağlantıya göre değişir.
- Makinede Docker'ın kontrol veritabanı ve varsayılan 1536 MiB geri yükleme
  konteyneri için yeterli bellek; imajlar, kaynak yedek ve özel kopyası için disk olmalı.
- İlk gerçek yedek PostgreSQL **17**, `pg_dump` **17** ve **custom (`-Fc`)** formatında
  olmalı. Başlangıç için küçük, ek eklenti gerektirmeyen ve kullanmaya yetkili
  olduğun bir yedek seç. Sıkıştırılmış dosyanın küçük olması açılınca sığacağını göstermez.
- Çalışan başka Restivra kurulumunun klasörünü veya `.env` dosyasını kullanma.
  Varsayılan `7474` portu doluysa aşağıdaki port notunu izle.

## 1. Kur ve başlat

Docker Desktop'ı aç. PowerShell 7 terminalinde:

```powershell
git clone --branch v0.1.0-pilot.3 --depth 1 https://github.com/lazypanic999/restivra.git restivra-pilot
cd restivra-pilot
./scripts/install.ps1
./scripts/pilot-check.ps1
```

`setup.ps1`, sana özel veritabanı parolası ve giriş anahtarını `.env` içine yazar.
Dosyayı yerel editöründe aç; `RESTIVRA_AUTH_TOKEN=` satırındaki değeri al.
`.env` dosyasını veya ekran görüntüsünü paylaşma. Panel:
**[http://127.0.0.1:7474](http://127.0.0.1:7474)**. Bu anahtarla giriş yap.
Arayüz İngilizce açılır; üstteki dil seçicisinden Türkçe seçebilirsin.

`pilot-check` başarılıysa Docker, imaj, API/veritabanı, giriş koruması ve worker
ön koşulları hazırdır. Henüz gerçek geri yükleme yapılmış değildir.

Port çakışıyorsa ilk `compose up` öncesinde özel `.env` dosyana
`RESTIVRA_PORT=17474` ekle; paneli `http://127.0.0.1:17474` ile aç ve iki pilot
komutuna da `-Base http://127.0.0.1:17474` ver. Aynı Docker üzerinde başka Restivra
da çalışıyorsa `.env` içine ayrıca benzersiz `COMPOSE_PROJECT_NAME=restivra-pilot-benim`
ekle. Ayrı klasör, yeni `.env`, ayrı port ve benzersiz uygulama imaj etiketi kullan.
Başlatmadan önce [ikinci kurulum adımlarını](SECOND-INSTALL.md) uygula; mevcut
veritabanını kopyalama.

## 2. Önce örnek veriyle dene

```powershell
./scripts/pilot-smoke.ps1
```

Bu komut gerçek PostgreSQL 17'de iki hayali müşteri oluşturur, `pg_dump -Fc` ile
yedekler ve çalışan Restivra API/worker üzerinden iki proje çalıştırır:

| Proje | Beklenen sonuç |
| --- | --- |
| `Pilot … / Passing sample` | Geri yükleme, iki müşteri kontrolü ve temizlik başarılı |
| `Pilot … / Intentional failure` | Geri yükleme ve temizlik başarılı, `SELECT false` kontrolü başarısız |

İkinci projenin kırmızı/turuncu görünmesi kasıtlıdır. İkisinin de raporunu aç;
kontrol sonucunu ve temizliği ayırt edebildiğini doğrula. JSON raporunu indir.
Komut sonunda geçici kaynak veritabanı silinir. Örnek yedek ve iki proje incelemen
için kalır; otomatik zamanlamaları kapalıdır. İnceledikten sonra projeleri
**Archive / Arşivle** ile arşivleyebilirsin. Kendi projelerine dokunulmaz.
Tekrar çalıştırmak yeni adlarla iki örnek proje daha oluşturur.

Komut hata verirse panelde oluşan Pilot projelerini incele. Hâlâ çalışan bir iş
varsa yeniden komut başlatmak yerine bitmesini bekle veya panelden iptal et.
Yerel kanıtlar `work/pilot-…/` altında; otomatik olarak hiçbir yere gönderilmez.

## 3. Kendi yedeğini doğrula

1. Mevcut, tamamlanmış PostgreSQL 17 custom yedeğini `backups/` altına kopyala:
   örneğin `backups/uygulamam/latest.dump`. Restivra kaynak veritabanına bağlanmaz
   ve yedeğini kendisi oluşturmaz. Devam eden yazımı `.partial` uzantısında tut;
   ancak başarıyla tamamlanınca aynı dosya sisteminde `.dump` adına taşı.
2. **New project / Yeni proje** ile yolu `uygulamam/latest.dump` olarak gir.
   Makinenin tam dosya yolunu yazma. Varsayılan sınırlar: 256 MiB arşiv,
   512 MiB açılmış veritabanı, 180 saniye doğrulama süresi. Sığmıyorsa destek
   notuna boyutları yaz; rastgele sınır yükseltme. Veri alanına indeksler, PostgreSQL sistem dosyaları ve WAL de dahildir; arşiv
   boyutu tek başına kapasite hesabı değildir. Kapasite rehberi
   [işletim belgesinde](PILOT-RUNBOOK.md).
3. Şemana uygun, tek bir `true/false` döndüren kontrol ekle. Örneğin tablo gerçekten
   varsa `SELECT EXISTS (SELECT 1 FROM public.orders)`. Noktalı virgül ekleme.
   Tablonun boş olması meşruysa bu kontrolü kullanma. Gerekirse birden fazla
   uygulama kuralı yaz. Sadece `SELECT true`, veri doğruluğunu sınamaz.
4. Manuel test başlat. **Passed**, kontrol sonuçları, SHA-256, süre ve
   **cleanup passed** durumunu raporda gör. Müşteri verisi veya özel SQL içeren
   raporu herkese açık issue'ya yükleme.
5. Bu başarı, eski kullanıcı/rol yetkilerini veya uygulamanın tümüyle açılacağını
   kanıtlamaz. Mevcut yedekleme ve bağımsız geri dönüş prosedürünü koru.

Özel extension gerektiren arşivler, başka PostgreSQL ana sürümleri, fiziksel
yedek, PITR ve güvenmediğin dump dosyaları bu pilotun dışında.

## 4. Günlük testi aç ve bir hafta izle

Kendi gerçek projen için makinenin genellikle açık olduğu bir saat seç ve saat
dilimini açıkça ayarla; örneğin `Europe/Istanbul`. İlk planlı çalışmanın gerçekten
oluştuğunu kontrol et. Panel saatleri cihazının, zamanlama projenin saat diliminde.

Bilgisayar uyurken/kapalıyken veya Docker çalışmıyorken test çalışmaz.
Geri dönünce gecikmiş günlük zamanlama için bir test kuyruğa alınır; kaçırılan her
güne ayrı test üretilmez. Yedi günlük takip, yedi başarılı test varsayımı değildir:
beklenen çalışma zamanlarını ve o saatlerde makinenin durumunu kaydet.

[Pilot kayıt formunu](PILOT-RECORD.tr.md) özel bir konuma kopyala. Günlük yalnızca
sonuç, süre, temizleme durumu, Docker/makine erişilebilirliği ve ihtiyaç duyulan
yardımı kaydet. Planlı testler çalışmıyorsa önce Worker online durumuna bak.
Uyarı istiyorsan kendi HTTPS alıcını ayrıca yapılandırıp teslimi test et;
pilot varsayılan olarak e-posta/Telegram bildirimi göndermez.

Gün gün görevler ve bitiş ölçütleri [pilot planında](PILOT-PLAN.tr.md).

## 5. Duraklat, geri dön, güncelle

- Ara vermek: proje zamanlamasını kapat; çalışan işi bitir veya panelden iptal et.
  Ardından `docker compose stop`. Devam etmek: `docker compose start` ve
  `./scripts/pilot-check.ps1`.
- Kaynakları kaldırıp veriyi korumak: işler bittikten sonra `docker compose down`.
  Yeniden açmak: `docker compose up -d --wait`. **`down -v` geçmişi siler; pilotta kullanma.**
- Güncelleme: pilot boyunca sürümü sabit tut. Yeni sürümde önce
  `New-Item -ItemType Directory -Path work -Force`, sonra
  `./scripts/metadata-backup.ps1 -OutputPath work/before-update.dump` ile metaveriyi
  özel konuma yedekle. `./scripts/metadata-check.ps1 -Archive work/before-update.dump` ile
  geçici, izole veritabanında geri yüklemeyi doğrula. Kaynak dump, `.env` ve
  `private/sources.json` ayrıca korunmalı. Güncelleme adımını
  pilot sorumlusuyla belirle; çalışma sırasında `git pull` yapma.
- Worker takılırsa: `docker compose logs --tail=100 worker` ve
  `docker compose exec -T worker restivra doctor --role worker` ile incele.
  Docker düzeldikten sonra worker otomatik toparlanabilir. Devam ediyorsa güvenli
  tanı bilgilerini paylaş; Docker volume'larını veya geçici kaynakları rastgele silme.

Native Linux alternatifi: aynı etiketi klonla, `sh scripts/setup.sh` çalıştır,
imajı indir ve aynı Compose komutuyla başlat. `pilot-*.ps1` komutları PowerShell
7.2+ ister; yüklü değilse [manuel ilk arşiv adımlarını](PILOT-RUNBOOK.md#connect-the-first-real-archive)
izle. Bu otomatik başlangıç paketi Windows üzerinde denenmiştir; macOS/ARM için
aynı doğrulama tamamlanmış sayılmaz.

## İlk başarıdan sonra isteğe bağlı özellikler

- [S3 uyumlu arşiv kaynakları](S3-SOURCES.md): anahtarlar worker'ın özel dosyasında kalır.
- [SQL kontrol şablonları](CHECK-TEMPLATES.md): sorguyu görüp düzenleyerek uygula.
- [Önceki çalışmayla karşılaştırma](RUN-COMPARISON.md): sorgu değişikliği ayrıca gösterilir.
- [Yazdırılabilir İngilizce/Türkçe kanıt](EVIDENCE-REPORTS.md): isimleri gizleme seçeneği vardır.
- [Proje ayarlarını taşıma](PROJECT-TRANSFER.md): içe aktarılan zamanlama kapalı başlar.
- [Restivra metaveri yedeğini doğrulama](METADATA-CHECK.md): mevcut kurulumu değiştirmeden geri yükleme testi.

İlk adım yine yukarıdaki yerel, iki satırlık örnekle doğrulamadır.

Hazır imaj güncellemesi ve önceki sürüme dönüş: [güncelleme rehberi](UPDATING.tr.md).
