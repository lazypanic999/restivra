# Gerçek PostgreSQL kabul denemesi

Deneme, Docker içinde çalışan **PostgreSQL 17.12** üzerinde, Restivra'nın çalışan API/worker servisleri kullanılarak yapıldı. Veritabanı mock değildi; veriler bu deneme için üretilmiş sentetik verilerdir. Müşteri verisi kullanılmadı.

## Kaynak ve yedek

| Nesne | Kayıt |
| --- | ---: |
| customers | 10.000 |
| products | 500 |
| orders | 25.000 |
| order_items | 100.000 |
| Toplam | 135.500 |

Şema; kimlik dizileri, üç foreign key, iki özel indeks, JSONB, numeric alanlar, bir view ve bir SQL fonksiyonu içerir. Gerçek `pg_dump -Fc` çıktısı **530.663 bayt**.

Yedeğin SHA-256 değeri: `8c2bbf550de1f5a1b4a0c6657893302c619aa2d5948b3af399960aede07ab667`.

Kaynakta hesaplanan sıralı veri özeti `fbd89235fba940ddb5b3c4f63e9bf230`; geri yüklenen 100.000 sipariş satırında aynı değer doğrulandı. Satır adetleri, foreign key'ler, indeksler, JSONB içeriği, view toplamı, fonksiyon ve identity sequence dahil **11 kontrol** geçti.

## Tek tek sonuçlar

| Senaryo | Beklenen | Gözlenen | Temizlik |
| --- | --- | --- | --- |
| İlişkisel yedeği geri yükleme | 11 kontrol başarılı | passed, restore passed | passed |
| Kaynak siparişleri boşalttıktan sonra aynı yedeği geri yükleme | 25.000 sipariş ve 100.000 satır geri gelir | 11 kontrol yeniden geçti | passed |
| Bilerek yanlış iş kuralı | Geri yükleme geçer, kontrol false olur | failed, check failed | passed |
| Bulunmayan tabloya sorgu | Sorgu hatası ayrı gösterilir | failed, check error | passed |
| Kontrolde yazma girişimi | Salt okunur işlem engeller | failed, check error | passed |
| 15 saniyelik sorgu | 10 saniyelik sınır durdurur | failed, check error | passed |
| Gerçek yedeğin yarıya kesilmiş kopyası | Geri yükleme reddedilir | failed, restore failed | passed |
| Çalışan pg_sleep sorgusunda iptal | Kullanıcı iptali işlenir | cancelled, restore passed | passed |

Kaynak `orders` ve `order_items` tabloları yalnızca bu denemeye ait izole kaynak konteynerinde boşaltıldı. Kaynak sipariş adedi **0** olarak okundu; mevcut yedek buna rağmen tüm kontrolleri geçti. Testin kaynak veritabanını okumak yerine yedeği ayrı ortamda geri yüklediği de sınandı.

Aynı yedek yeniden kullanıldığında değişmeyen SHA-256 uyarısı kaydedildi. Her senaryodan sonra ilgili restore konteynerinin kalmadığı Docker üzerinden kontrol edildi. Kaynak konteyneri de kaldırıldı. Başarısız/iptal senaryolarının projeleri arşivlendi; geçmişleri korunuyor.

## Kanıt ve tekrar çalıştırma

- [Makine tarafından üretilen özet](acceptance/summary.json)
- [Kaynak PostgreSQL ölçümleri](acceptance/source-metrics.json)
- [Şema ve veri üretimi](../testdata/commerce.sql)
- [Tekrarlanabilir PowerShell denemesi](../scripts/acceptance.ps1)

Çalışan Compose uygulamasının proje kökünde `./scripts/acceptance.ps1` çalıştırılabilir. Her deneme benzersiz adlar kullanır; zamanlama açmaz. Ayrıntılı raporlar yerelde `work/acceptance-<id>/` altında tutulur. Yedek dosyaları Git'e gönderilmez.

Bu veri seti büyük üretim veritabanlarının yük testi değildir. Gerçek müşteri eklentileri ve uzun süreli işletim ayrıca doğrulanmalıdır.
