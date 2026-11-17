# Restivra — Türkçe kullanım

Mevcut PostgreSQL 17 yedeğini geçici bir ortamda geri yükler, SQL kontrollerini çalıştırır ve sonucu raporlar. Tek ajansın kendi sunucusunda çalışması için hazırlanmış ilk MVP.

## Kurulum

Docker'ın Linux konteyner motoru açık olmalı. PowerShell 7:

```powershell
git clone https://github.com/lazypanic999/restivra.git
cd restivra
./scripts/setup.ps1
docker pull postgres:17-bookworm
docker compose up --build -d
```

Linux'ta kurulum betiği `sh scripts/setup.sh`. Panel: **[http://127.0.0.1:7474](http://127.0.0.1:7474)**.

## İlk proje

1. PostgreSQL 17 üzerinde `pg_dump` 17 ile `-Fc` formatında alınmış tamamlanmış yedeği `backups/` klasörüne koy.
2. Panelde **Yeni proje** seç. Dosya yolunu klasöre göre yaz: örneğin `musteri/*.dump`.
3. İstersen tek boolean döndüren kontrol ekle: `SELECT count(*) > 0 FROM public.customers`. Noktalı virgül kullanma.
4. **Test et** ile başlat. Raporunda geri yükleme, kontroller ve temizliği ayrı incele.
5. Günlük otomatik test için proje ayarlarında saat ve saat dilimi belirle.

Birden fazla eşleşmede değiştirilme zamanı en yeni dosya seçilir. Yedeği önce başka uzantıya yazıp tamamlandığında seçilen ada atomik taşı. Dosya değiştirilme zamanı, yedeğin alınma zamanının kesin kanıtı değildir.

Go 1.25+ kuruluysa `./scripts/seed-demo.ps1` iki hayali müşteri projesi oluşturur. **Demo** etiketli bu projelerde gerçek geri yükleme yapılır; biri geçer, diğeri bilerek yanlış kontrol nedeniyle başarısız olur. Zamanlamaları kapalıdır.

## MVP'de bulunanlar

- Genel bakış, proje ayarları, son 100 çalışma, rapor ve JSON indirme.
- Kalıcı PostgreSQL iş kuyruğu; aynı anda tek geri yükleme.
- Proje başına tek aktif iş, manuel/günlük çalışma ve iptal.
- Worker yeniden başlayınca yarım işlerin ve geçici kaynakların toparlanması.
- SHA-256, değişmeyen yedek ve eski dosya uyarıları.
- İsteğe bağlı başarısızlık webhook'u; en fazla üç teslim denemesi.
- Varsayılan 30 günlük tamamlanmış çalışma geçmişi.

## Sınırlar

Panel yerel erişim içindir; kullanıcı girişi yoktur. Uzak sunucu için `ssh -L 7474:127.0.0.1:7474 user@server` kullan. Doğrudan internete açma.

Yalnızca güvendiğin ve yetkili olduğun yedeklerle çalış. Dump çalıştırılabilir SQL içerebilir; konteyner düşmanca girdi için tam izolasyon değildir. Docker yetkisi yalnızca worker'dadır ve host üzerinde güçlü yetkidir.

Başarılı sonuç; mantıksal geri yükleme ve tanımlı sorguların geçtiğini gösterir. Tam uygulama kurtarması, orijinal roller/yetkiler, PITR ve güncellik ayrıca doğrulanmalıdır. Özel eklenti ve diğer ana sürümler bu kapsamda yoktur.

`.env` rastgele veritabanı parolası içerir; Git'e alınmaz. Yedek ve raporları da repoya ekleme. Varsayılan restore ortamı 1536 MiB bellek ve 512 MiB RAM tabanlı veri alanı kullanır; sıkıştırılmış yedek daha büyük bir veritabanına açılabilir.

## İşletim

```powershell
docker compose logs --tail=100 api worker
docker compose restart worker
docker compose down
```

`down` veriyi korur; `down -v` geçmiş dahil named volume'leri siler. `RESTIVRA_INSTANCE` sabit kalmalı; aynı Docker motorundaki kurulumlarda benzersiz olmalı.

Bildirim için `.env` içindeki `RESTIVRA_WEBHOOK_URL` değerine genel JSON kabul eden açık HTTPS adresi yazıp worker'ı yeniden oluştur. Yerel/özel ağlar ve yönlendirmeler engellenir. Bildirim hatası test sonucunu değiştirmez.

Ayrıntılı mimari, yapılandırma ve testler: [README.md](README.md).
