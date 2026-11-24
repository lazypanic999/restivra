# Hazır imajla kurulum ve güvenli güncelleme

PowerShell 7.2+, Git ve yerel Linux/amd64 Docker motoru gerekir. Pilot için Windows
denendi. Go, Node veya bilgisayarına PostgreSQL kurman gerekmiyor.

```powershell
git clone --branch v0.1.0-pilot.5 --depth 1 https://github.com/lazypanic999/restivra.git restivra-pilot
cd restivra-pilot
./scripts/install.ps1
./scripts/pilot-check.ps1
./scripts/pilot-smoke.ps1
```

Panel http://127.0.0.1:7474 adresinde. Giriş anahtarı sana özel `.env` dosyasında.
Özel port seçtiysen kurucunun yazdırdığı adresi aç. `pilot-check.ps1` ve
`pilot-smoke.ps1` kayıtlı portu kendileri okur; ayrıca `-Base` yazman gerekmez.
Ön kontrol, giriş yapmadan önce API konteynerinin gerçekten bu dizine ve
seçilen yerel porta ait olduğunu doğrular; başka kurulumun portunu reddeder.
Dosyayı paylaşma. Kurulum yayımlanmış imajı indirir, içerik özeti ve uygulama
dosyasının hash değerini doğrular. Sonraki `docker compose up -d` aynı imajı açar.

Güncelleme için çalışan işlerin bitmesini bekle ve sürüm notlarını oku:

```powershell
git fetch origin tag v0.1.0-pilot.5
git checkout --detach v0.1.0-pilot.5
./scripts/update.ps1 -Version v0.1.0-pilot.5
# Önceki sürüme dönmek gerekirse:
./scripts/update.ps1 -Rollback
```

Açık sürüm etiketi kullanılır; `latest` kendiliğinden indirilmez. Betik önce
uygulama imajını indirir; bilgisayardaki PowerShell betiklerini değiştirmez.
Bu yüzden önce incelenen hedef sürümün etiketini yukarıdaki gibi getir ve seç.
Git izlenen dosyalardaki çakışan değişiklikleri reddeder. Özel, Git tarafından
izlenmeyen ayarlar ve veri birimleri korunur; `git clean` veya zorla checkout
kullanma. Güncelleyici
imajı doğrular, bekleyen/çalışan işler varsa durur. API ve worker kısa süreliğine
kapanır; metaveri yedeği alınır ve ayrı PostgreSQL ortamında gerçekten geri
yüklenip doğrulanır. Ardından uygulama değiştirilir. Sağlık kontrolü veya yeni
worker kalp atışı başarısızsa önceki uygulama imajı yeniden açılır ve hata bildirilir.

PostgreSQL konteyneri/volümü ve veritabanının içeriği bu işlemle değiştirilmez.
Geri dönüşte eski metaveri yedeği veritabanına yazılmaz. Yalnızca aynı metaveri
sözleşmesini kullanan sürümler desteklenir; büyük veritabanı geçişleri ayrıdır.
Özel yedekler `private/checkpoints/` altında kalır ve otomatik silinmez.

Eski kaynak kurulumunda `-Adopt` yalnızca aynı uygulama dosyası hash değerine
sahip sürümle kabul edilir. Eski imaj özel bir yerel etiketle saklanır. Kimlik
doğrulama ayarın korunur. Yeni kurulumlar benzersiz `COMPOSE_PROJECT_NAME` ve
seçtiğin `RESTIVRA_PORT` değerini `.env` içine kaydeder; yeni terminalde de aynı
konteynerler ve port seçilir. İkinci kurulumda boş bir port seç. Eski `.env`
dosyaları değiştirilmez; bu iki kayıt yoksa önceki terminal ayarlarını koru.
Terminal değişkenleri `.env` değerlerini geçersiz kılar. Başka klasöre geçerken
eski değişkenleri temizle; kurulu projenin adını değiştirmek farklı veritabanı
birimlerini seçer ve mevcut veriyi taşımaz.

Elektrik kesilmesi veya Docker'ın kapanması otomatik geri dönüşü de engelleyebilir.
Yarım kalmış işlem yeni güncellemeyi bloke eder. Volüm silme; özel günlükteki önceki
imajla kurtarma ve ayrıntılı sınırlar için [işletim rehberini](UPDATING.md) izle.

Güncel betiklerle başlayan bir güncelleme PowerShell kapanması veya elektrik
kesintisi nedeniyle yarıda kalırsa Docker'ı tekrar açıp aynı kurulum dizininde
`./scripts/update.ps1 -Recover`, ardından `./scripts/pilot-check.ps1` çalıştır.
Bu komut önceki uygulama imajını geri başlatır; veritabanı yedeğini geri yüklemez.
Asıl PostgreSQL konteyneri ve verileri duruyor olmalıdır. Dizin, Compose projesi,
kurulum kimliği, imaj özeti ve ikili dosya doğrulanmadan uygulamayı durdurmaz.
Bekleyen işler, duraklatılmış konteynerler veya başka bir bakım komutu varsa
işlemi reddeder. Eksik API/worker konteynerini yeniden oluşturabilir; eksik
veritabanını bu yöntemle kurtaramaz. Önceki doğrulanmış imaj yerel Docker
önbelleğindeyse kayıt deposuna bağlanmadan kullanılabilir.

Bu yol yalnızca `restivra.operation.v2` günlüğü içindir. Eski günlükte veya kimlik
kontrolü başarısız olduğunda günlük dosyasını silip zorlamadan İngilizce rehberdeki
elle inceleme adımlarını uygula. Kimlik bilgileri, kaynak profilleri ve geçmiş
değiştirilmez; yalnızca yarıda kalan işleme ait tanılama konteynerleri temizlenir.
