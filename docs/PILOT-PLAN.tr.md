# Pilot planı: bir geliştirici, kendi bilgisayarı, yedi gün

Katılımcıya [başlangıç rehberini](PILOT-START.tr.md) ve özel dolduracağı
[kayıt formunu](PILOT-RECORD.tr.md) gönder. Başlangıç ve bitiş tarihini katılımcıyla
belirle; bu belge otomatik bir takip veya bildirim oluşturmaz.
İlk iletişim için [kısa davet mesajını](PILOT-INVITE.tr.md) kullanabilirsin.

## Pilot öncesi: sorumlu ve katılımcı birlikte

- Sürüm `v0.1.0-pilot.3`; Windows + PowerShell 7.2+ + Docker Linux/amd64 engine.
  İşletim sistemi, CPU mimarisi, Docker sürümü ve kullanılabilir kaynakları kaydet.
- Bir gerçek PostgreSQL 17 custom yedek, yetkili geliştirici, bilinen extension
  listesi ve beklenen açılmış boyut. Varsayılan sınırlara uymuyorsa başlamadan konuş.
- Mevcut yedek üreticisi, yedeğin tamamlandığını belirleme yöntemi, günlük saat
  ve bilgisayarın beklenen açık kalma süresi net olsun.
- Katılımcı mevcut yedekleme/kurtarma yöntemini korur. Müşteri dump'ları, `.env`,
  bağlantı adresleri, özel SQL ve erişim anahtarları sorumluya gönderilmez.
- Destek kanalı olarak mevcut özel iletişim yolunuzu kullanın. Herkese açık
  bug bildirimi yalnızca temizlenmiş/sentetik verilerle; açıklar [özel güvenlik yoluyla](../SECURITY.md).
  Bu pilot için bir destek SLA'sı veya garantili alarm hizmeti vaat edilmiyor.

## Günler ve gözlemler

| Zaman | Katılımcının görevi | Kaydedilecek kanıt |
| --- | --- | --- |
| Gün 0 | Rehberle kur; önce yardım almadan dene. Ön koşul kontrolü, örnek başarılı/başarısız test ve ilk gerçek manuel doğrulama | Kurulum süresi, takıldığı adım, yardım sayısı, sonuç/temizlik |
| Gün 1 | İlk gerçek planlı çalışmayı gör; zaman dilimini kontrol et | Beklenen ve gerçek kuyruk zamanı, worker/makine durumu |
| Gün 2 | Yedek üreticisinin yeni tamamlanmış dosyasını kullan; SHA-256 ve uyarıları incele | Dosyanın değişmesi bekleniyor muydu, gözlenen hash değişimi; hash tek başına veri güncelliği değildir |
| Gün 3 | İş yokken `docker compose restart worker`; Worker online ve sonraki manuel testi kontrol et | Yeniden bağlanma, yeni test sonucu, temizlik; gerçek veri üzerinde SIGKILL/arıza enjeksiyonu yapma |
| Gün 4 | Geçmişi filtrele, CSV indir, raporda başarısızlığın sebebini bul; örnek başarısız projeyi arşivle | Sonucu yardım almadan açıklayabildi mi, dil ve mobil/masaüstü kullanım notu |
| Gün 5 | Metaveriyi özel dosyaya yedekle ve izole geri yükleme kontrolünü çalıştır; varsa HTTPS alıcısına örnek başarısızlığı gönder | Export ve izole restore/şema/temizlik geçti mi; alıcı kullanılıyorsa teslim görüldü mü |
| Gün 6–7 | Günlük sonuçları gözden geçir; bitiş görüşmesi ve devam/durdurma kararı | Beklenen/oluşan test sayısı, açıklanamayan atlama, müdahale, kullanım ve ödeme isteği |

Metaveri export'u almak, geri yüklenebildiğini kanıtlamaz. Önce
[otomatik izole metaveri kontrolünü](METADATA-CHECK.md) çalıştır. Bu kontrolün
sınırları içinde geri yükleme/şema/temizlik sonucunu kaydet. Uygulamanın bütünüyle
geri açılmasını da sınamak için ayrı PostgreSQL 17
üzerinde metaveri geri yükleme provası pilot sorumlusu eşliğinde yapılır;
[işletim rehberindeki sınırlar](PILOT-RUNBOOK.md#back-up-restivras-own-state) izlenir.
Orijinal ve klon metadata ile iki worker'ı aynı Docker motorunda çalıştırmayın.
Bu prova yapılmadıysa sonuç formunda açıkça **yapılmadı** yazın.

## Kabul ölçütleri

- Geliştirici rehberden kurulumu ve ilk iki örnek sonucu tamamlayabiliyor;
  yardım ve harcanan süre gerçek değerleriyle kayıtlı.
- En az bir gerçek manuel ve bir gerçek planlı restore, uygulamaya özel SQL
  kontrolleri ve başarılı temizlikle tamamlanmış.
- Makine/Docker uygunken beklenen günlük çalışmalar açıklanabiliyor. Uykuda veya
  kapalı geçen günler ürün başarısı olarak sayılmıyor; pilot gerektiğinde uzatılıyor.
- Başarısız kontrol doğru gösteriliyor, önceki denemeler ve raporlar anlaşılabiliyor.
  Açıklanamayan başarılı sonuç veya kayıp çalışma yok.
- Başarılı biten işlerde sahipsiz restore konteyneri/özel snapshot kalmıyor.
  Bilinmeyen temizlik durumunda yeni işe devam etmek yerine inceleme yapılıyor.
- Worker yeniden başlatma sonrası işleyiş doğrulanmış; metaveri export'u alınmış,
  geri yükleme provası ve isteğe bağlı bildirim teslimi ayrı ayrı işaretlenmiş.

Yanlış başarı, veri kaybı, yetkisiz erişim veya temizlik hatasında gerçek verili
testi durdurun; mevcut kayıtları koruyup sorumluya özel kanaldan bildirin.
Uygulamanın yaptığı mantıksal restore, bağımsız uygulama kurtarma testinin yerini almaz.

## Bitiş görüşmesi: 15 dakika

1. İlk kurulumda hangi adım anlaşılmadı? Yardımsız tekrar kurabilir misin?
2. Rapor hangi somut sorunu buldu? Ne yapacağını anlayabildin mi?
3. Bunu kaldırırsan hangi işi elle yapmak zorunda kalırsın?
4. Bilgisayarda çalışması mı, ayrı bir sunucuda olması mı işine yarıyor?
5. Kullanmayı sürdürür müsün? Bunun için ödeme yapar mısın; hangi tutar ve
   hangi ek fayda karşılığında? Gerçek cevap yaz; fiyat veya satış sonucu varsayma.

Karar: **devam**, **düzeltip tekrar dene** veya **durdur**. Önce engelleyici
bulguları GitHub issue'suna temizlenmiş yeniden üretimle taşı. Geniş dağıtım,
hosted SaaS ve ödeme entegrasyonu bu pilot sonucuna göre ayrıca planlanır.
