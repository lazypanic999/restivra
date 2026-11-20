package app

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/lazypanic999/restivra/internal/verify"
)

type evidenceRow struct{ Name, Status, Code string }
type evidenceAttempt struct {
	Number                 int
	Status, Finished, Code string
}
type evidenceDocument struct {
	Lang, Title, Project, JobID                    string
	Result, CurrentState, ErrorCode                string
	Queued, Started, Finished, Generated, Modified string
	Duration, Archive, Bytes, Hash                 string
	Image, ImageID, Source, Profile                string
	Object, Version, ETag, Restore, Cleanup        string
	Redacted                                       bool
	Attempt                                        int
	Checks                                         []evidenceRow
	Attempts                                       []evidenceAttempt
	Labels                                         map[string]string
}

var evidenceText = map[string][2]string{
	"title":           {"Restore verification evidence", "Geri yükleme doğrulama kanıtı"},
	"project":         {"Project", "Proje"},
	"run":             {"Run ID", "Çalışma kimliği"},
	"result":          {"Selected attempt result", "Seçilen denemenin sonucu"},
	"current":         {"Current run state", "Çalışmanın güncel durumu"},
	"attempt":         {"Attempt", "Deneme"},
	"queued":          {"Queued (UTC)", "Kuyruk zamanı (UTC)"},
	"started":         {"Started (UTC)", "Başlangıç (UTC)"},
	"finished":        {"Finished (UTC)", "Bitiş (UTC)"},
	"duration":        {"Attempt duration", "Deneme süresi"},
	"archive":         {"Tested archive", "Denenen arşiv"},
	"bytes":           {"Archive bytes", "Arşiv boyutu (bayt)"},
	"modified":        {"Source modified (UTC)", "Kaynak değiştirilme zamanı (UTC)"},
	"source":          {"Source", "Kaynak"},
	"profile":         {"Profile", "Profil"},
	"object":          {"Object", "Nesne"},
	"version":         {"Object version", "Nesne sürümü"},
	"image":           {"Restore image", "Geri yükleme imajı"},
	"imageID":         {"Image identity", "İmaj kimliği"},
	"checks":          {"Verification checks", "Doğrulama kontrolleri"},
	"name":            {"Check", "Kontrol"},
	"status":          {"Result", "Sonuç"},
	"code":            {"Diagnostic code", "Tanı kodu"},
	"restore":         {"Archive restore", "Arşiv geri yükleme"},
	"cleanup":         {"Temporary resource cleanup", "Geçici kaynak temizliği"},
	"history":         {"Preserved attempts", "Korunan denemeler"},
	"generated":       {"Generated (UTC)", "Oluşturulma zamanı (UTC)"},
	"print":           {"Use your browser's Print command to save this report as PDF. No external fonts or assets are required.", "Bu raporu PDF olarak kaydetmek için tarayıcının Yazdır komutunu kullan. Harici yazı tipi veya görsel gerekmez."},
	"limits":          {"This records a logical restore and selected SQL checks. It does not prove full application recovery, original roles/grants, backup freshness or point-in-time recovery. SHA-256 identifies tested bytes; modification time and ETag do not establish data recency. Keep an independent recovery procedure.", "Bu rapor mantıksal geri yüklemeyi ve seçilen SQL kontrollerini kaydeder. Tam uygulama kurtarmasını, eski rol/izinleri, yedek güncelliğini veya belirli ana dönüşü kanıtlamaz. SHA-256 denenen baytları tanımlar; değiştirilme zamanı ve ETag veri güncelliğini kanıtlamaz. Bağımsız kurtarma yöntemini koru."},
	"redacted":        {"Project, source and check names are hidden. Run IDs and content fingerprints remain; review before sharing.", "Proje, kaynak ve kontrol adları gizlendi. Çalışma kimlikleri ve içerik özetleri korunur; paylaşmadan önce incele."},
	"metadata":        {"Operational metadata. SQL text, credentials and restored rows are excluded.", "Operasyonel metaveri. SQL metni, erişim anahtarları ve geri yüklenen satırlar dahil değildir."},
	"noChecks":        {"No custom checks were configured.", "Özel kontrol yapılandırılmamış."},
	"redactedProject": {"Redacted project", "Adı gizlenmiş proje"},
	"redactedCheck":   {"Check", "Kontrol"},
	"local":           {"Local files", "Yerel dosyalar"},
	"s3":              {"S3-compatible storage", "S3 uyumlu depolama"},
	"passed":          {"Passed", "Başarılı"},
	"failed":          {"Failed", "Başarısız"},
	"interrupted":     {"Interrupted", "Yarıda kesildi"},
	"cancelled":       {"Cancelled", "İptal edildi"},
	"running":         {"Running", "Çalışıyor"},
	"queuedStatus":    {"Queued", "Kuyrukta"},
	"error":           {"Error", "Hata"},
	"not_run":         {"Not run", "Çalıştırılmadı"},
	"not_needed":      {"Not needed", "Gerekmedi"},
}

func makeEvidence(j Job, report *verify.Report, lang string, redact bool) evidenceDocument {
	index := 0
	if lang == "tr" {
		index = 1
	}
	labels := map[string]string{}
	for key, pair := range evidenceText {
		labels[key] = pair[index]
	}
	status := func(value string) string {
		if value == "queued" {
			return labels["queuedStatus"]
		}
		if text, ok := labels[value]; ok {
			return text
		}
		return value
	}
	at := func(value *time.Time) string {
		if value == nil || value.IsZero() {
			return "—"
		}
		return value.UTC().Format("2006-01-02 15:04:05Z")
	}
	d := evidenceDocument{Lang: lang, Title: labels["title"], Labels: labels, Project: j.ProjectName, JobID: j.ID, Result: status(report.Status), CurrentState: status(j.Status), Queued: at(&j.QueuedAt), Started: at(&report.StartedAt), Finished: at(&report.FinishedAt), Generated: at(timePointer(time.Now())), Duration: fmt.Sprintf("%.3f s", float64(report.DurationMS)/1000), Archive: report.Backup.Name, Bytes: strconv.FormatInt(report.Backup.Bytes, 10), Hash: report.Backup.SHA256, Image: report.Image, ImageID: report.ImageID, Restore: status(report.RestoreStatus), Cleanup: status(report.CleanupStatus), ErrorCode: report.ErrorCode, Redacted: redact, Attempt: report.Attempt, Source: labels["local"]}
	if d.Attempt == 0 {
		d.Attempt = j.AttemptCount
	}
	d.Modified = at(&report.Backup.ModifiedAt)
	if report.Backup.Source != nil && report.Backup.Source.Kind == "s3" {
		s := report.Backup.Source
		d.Source = labels["s3"]
		d.Profile = s.Profile
		d.Object = "s3://" + s.Bucket + "/" + s.Key
		d.Version = s.VersionID
		d.ETag = s.ETag
	}
	for i, check := range report.Checks {
		name := check.Name
		if redact {
			name = fmt.Sprintf("%s %d", labels["redactedCheck"], i+1)
		}
		d.Checks = append(d.Checks, evidenceRow{name, status(check.Status), check.ErrorCode})
	}
	for _, attempt := range j.Attempts {
		code := ""
		if attempt.Report != nil {
			code = attempt.Report.ErrorCode
		}
		d.Attempts = append(d.Attempts, evidenceAttempt{attempt.Number, status(attempt.Status), at(&attempt.FinishedAt), code})
	}
	if redact {
		d.Project = labels["redactedProject"]
		d.Archive = "—"
		d.Profile = ""
		d.Object = ""
		d.Version = ""
		d.ETag = ""
	}
	return d
}
func timePointer(at time.Time) *time.Time { return &at }

func (a *API) evidence(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !ValidID(id) {
		jsonError(w, 400, "invalid run ID")
		return
	}
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}
	if lang != "en" && lang != "tr" {
		jsonError(w, 400, "unsupported report language")
		return
	}
	redaction := r.URL.Query().Get("redact")
	if redaction != "" && redaction != "0" && redaction != "1" {
		jsonError(w, 400, "invalid report redaction option")
		return
	}
	j, err := a.Store.Job(r.Context(), id)
	if err != nil {
		apiError(w, err)
		return
	}
	report := j.Report
	suffix := ""
	if value := r.URL.Query().Get("attempt"); value != "" {
		number, e := strconv.Atoi(value)
		if e != nil || number < 1 || number > MaxAttempts {
			jsonError(w, 400, "invalid attempt number")
			return
		}
		report = nil
		for _, attempt := range j.Attempts {
			if attempt.Number == number {
				if attempt.Report != nil {
					copy := *attempt.Report
					copy.Attempt = number
					report = &copy
				}
				break
			}
		}
		suffix = "-attempt-" + strconv.Itoa(number)
	}
	if report == nil {
		jsonError(w, 409, "bu çalışma için henüz geri yükleme raporu yok")
		return
	}
	var body bytes.Buffer
	if err = evidenceTemplate.Execute(&body, makeEvidence(j, report, lang, redaction == "1")); err != nil {
		jsonError(w, 500, "report export failed")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="restivra-`+id+suffix+`.html"`)
	// The standalone report has no scripts, links, remote images or fonts. Inline
	// style is a fixed template; every user-controlled field is auto-escaped.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; script-src 'none'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	w.Header().Set("Content-Length", strconv.Itoa(body.Len()))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body.Bytes())
}

//go:embed templates/evidence.html
var evidenceHTML string

var evidenceTemplate = template.Must(template.New("evidence").Parse(evidenceHTML))
