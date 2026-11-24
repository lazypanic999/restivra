# Özel pilot kayıt formu

Bu dosyayı `work/` gibi Git'in dışında kalan özel bir konuma kopyala. Herkese açık
repoya gerçek müşteri adları, SQL, rapor, token veya dump ekleme. İş kimliği ve
hash bile operasyonel metaveridir; paylaşmadan önce değerlendir.

## Başlangıç

- Pilot kodu (kişisel bilgi içermeyen):
- Başlangıç / planlanan bitiş:
- Git etiketi / `git rev-parse HEAD`:
- Gerçekte çalışan sürüm / kaynak revizyonu (Kurulum → Sistem kontrolü; checkout etiketiyle karıştırma):
- İşletim sistemi / CPU mimarisi / PowerShell / Docker sürümü:
- Kullanılabilir Docker RAM / Docker diski / fiziksel bilgisayar diski:
- PostgreSQL / pg_dump ana sürümü ve extension gereksinimleri:
- Yedek boyutu / biliniyorsa açılmış boyut:
- Yedek üreticisi ve tamamlanma yöntemi:
- Günlük test saati / IANA saat dilimi:
- O saatlerde bilgisayarın açık kalma beklentisi:
- Kurulum süresi / ilk gerçek başarılı teste kadar süre:
- Yardım alınan adımlar ve müdahale sayısı:
- Örnek başarılı ve kasıtlı başarısız sonuçlar görüldü mü:

## Günlük kayıt

| Gün/tarih | Beklenen test zamanı | Makine + Docker uygun muydu | Gerçek sonuç/zaman | Süre / temizlik | Hash değişmesi bekleniyor muydu / gözlem | Yardım veya olay |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | | | | | | |
| 2 | | | | | | |
| 3 | | | | | | |
| 4 | | | | | | |
| 5 | | | | | | |
| 6 | | | | | | |
| 7 | | | | | | |

Başarısız sonucu `failed`, kapalı makineyi `çalışmadı: makine kapalı` gibi açık
yaz. Yedi güne karşılık otomatik yedi başarılı sonuç doldurma.

## Ayrı kontroller

- İlk gerçek manuel / planlı doğrulama:
- Worker yeniden başlatma ve sonrasındaki test:
- Geçmiş filtresi / CSV / arşiv kullanımı:
- EN / TR tercihi ve anlaşılmayan metin:
- Metaveri export'u (özel konum, tarih):
- Ayrı veritabanında metaveri geri yükleme: geçti / kaldı / yapılmadı:
- İsteğe bağlı webhook: kullanılmıyor / gerçek teslim görüldü / başarısız / denenmedi:
- Bildirim teslimi ile yedek doğrulama sonucunu ayrı okuyabildi mi:
- Destek özeti alınabildi mi / özel konumu (dosyayı otomatik paylaşma):
- Açıklanamayan atlanan test / yanlış başarı / temizlik sorunu:

## Bitiş

- Uygun makine koşullarında beklenen / oluşan planlı test:
- Başarılı / beklenen başarısız / beklenmeyen başarısız / iptal:
- Gerekli manuel müdahale ve toplam destek süresi:
- Bulunan gerçek fayda / eksik kalan temel ihtiyaç:
- Kullanmaya devam eder mi, neden:
- Ödeme isteği (katılımcının kendi sözleri ve varsa tutar):
- Karar: devam / düzeltip tekrar dene / durdur:
- Takip bulguları ve sorumlusu:

## İsteğe bağlı yeni özellik gözlemleri

- Kullanılan kaynak: yerel / S3 uyumlu (özel sağlayıcı adresini veya anahtarını yazma):
- Şablondan oluşturulan SQL kontrolünü anlayıp düzenleyebildi mi:
- Önceki sonuç karşılaştırması bir değişikliği açıklamaya yardımcı oldu mu:
- Yazdırılabilir kanıt ve isim gizleme yeterli miydi:
- Proje ayarları taşındıysa zamanlamanın kapalı başladığı görüldü mü:
- Metaveri kontrolünde restore / şema / temizlik sonucu ve hata kodu:
- Projelerde arama / eski sayfalara ve arşivlenmiş projelere erişim:
- Yeni rapordaki doğrulayıcı sürümü ile çalışan panel sürümünü ayırt edebildi mi:
- S3 profili düzenlendiyse iş yokken rehberdeki komutla worker yeniden oluşturuldu mu:

## Sorun çıktığında kısa kayıt

Önce saat, beklenen davranış, gerçek sonuç ve yapılan müdahaleyi yaz. Kurulum →
Sistem kontrolü üzerinden destek özetini indir; paylaşmadan önce sürüm ve toplu
sayıları incele. Dosya otomatik gönderilmez. Proje adları, SQL veya dump yerine
sentetik bir yeniden üretim hazırla. Veritabanı kontrol edilemiyorsa `null`
sayıları sıfır iş diye yorumlama. [Destek özetinin kapsamı](SUPPORT-SUMMARY.md).
