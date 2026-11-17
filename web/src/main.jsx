import React, { useState, useEffect, useCallback, useRef } from "react";
import { createRoot } from "react-dom/client";
import {
  Activity,
  ArrowDownToLine,
  ArrowRight,
  ArrowUpRight,
  Bell,
  Check,
  CheckCircle2,
  ChevronRight,
  Clock3,
  Database,
  FileCheck2,
  Folder,
  LayoutDashboard,
  LoaderCircle,
  MoreHorizontal,
  Play,
  Plus,
  RefreshCw,
  Search,
  Settings2,
  ShieldCheck,
  Terminal,
  Trash2,
  X,
  XCircle,
  TriangleAlert,
  CalendarClock,
  CircleHelp,
  HardDrive,
  Square,
  Copy,
} from "lucide-react";
import "./styles.css";

const labels = {
  passed: "Başarılı",
  failed: "Başarısız",
  running: "Çalışıyor",
  queued: "Sırada",
  cancelled: "İptal edildi",
  interrupted: "Yarıda kaldı",
  not_run: "Çalıştırılmadı",
  error: "Sorgu hatası",
  not_needed: "Gerekmedi",
};
const active = (j) => j && ["queued", "running"].includes(j.status);
const date = (v, full = false) =>
  v
    ? new Intl.DateTimeFormat("tr-TR", {
        day: "numeric",
        month: "short",
        ...(full ? { year: "numeric" } : {}),
        hour: "2-digit",
        minute: "2-digit",
      }).format(new Date(v))
    : "—";
const duration = (j) =>
  j?.report
    ? `${(j.report.duration_ms / 1000).toFixed(1)} sn`
    : j?.started_at && j?.finished_at
      ? `${Math.round((new Date(j.finished_at) - new Date(j.started_at)) / 1000)} sn`
      : "—";
const bytes = (v) =>
  v >= 1073741824
    ? `${(v / 1073741824).toFixed(2)} GiB`
    : v >= 1048576
      ? `${(v / 1048576).toFixed(2)} MiB`
      : v >= 1024
        ? `${(v / 1024).toFixed(2)} KiB`
        : v
          ? `${v} bayt`
          : "—";
function Mark() {
  return (
    <svg viewBox="0 0 40 40" aria-hidden="true">
      <rect width="40" height="40" rx="12" fill="currentColor" />
      <path
        d="M12 27V13h8a5 5 0 0 1 0 10h-3m6-1 6 5"
        fill="none"
        stroke="white"
        strokeWidth="3.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
function Badge({ status }) {
  const Icon = ["passed"].includes(status)
    ? CheckCircle2
    : active({ status })
      ? status === "running"
        ? LoaderCircle
        : Clock3
      : ["failed", "error"].includes(status)
        ? XCircle
        : TriangleAlert;
  return (
    <span className={`badge ${status}`}>
      <Icon size={13} className={status === "running" ? "spin" : ""} />
      {labels[status] || status}
    </span>
  );
}
function Empty({ icon: Icon = Folder, title, text, children }) {
  return (
    <div className="empty">
      <div className="empty-icon">
        <Icon size={27} />
      </div>
      <h3>{title}</h3>
      <p>{text}</p>
      {children}
    </div>
  );
}
function App() {
  const [page, setPage] = useState("overview"),
    [projects, setProjects] = useState([]),
    [jobs, setJobs] = useState([]),
    [health, setHealth] = useState(null),
    [session, setSession] = useState(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [form, setForm] = useState(null),
    [selected, setSelected] = useState(null),
    [search, setSearch] = useState(""),
    [filter, setFilter] = useState("all"),
    [busy, setBusy] = useState(""),
    [toast, setToast] = useState("");
  const token = useRef("");
  const api = useCallback(async (path, options = {}) => {
    const response = await fetch(`/api${path}`, {
      ...options,
      headers: {
        ...(options.method
          ? {
              "Content-Type": "application/json",
              "X-CSRF-Token": token.current,
            }
          : {}),
        ...options.headers,
      },
    });
    const data = await response.json();
    if (!response.ok) throw new Error(data.error || "İşlem tamamlanamadı");
    return data;
  }, []);
  const refresh = useCallback(async () => {
    try {
      const [p, j, h] = await Promise.all([
        api("/projects"),
        api("/jobs"),
        api("/health"),
      ]);
      setProjects(p);
      setJobs(j);
      setHealth(h);
      setError("");
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }, [api]);
  useEffect(() => {
    api("/session")
      .then((s) => {
        token.current = s.csrf_token;
        setSession(s);
        return refresh();
      })
      .catch((e) => {
        setError(e.message);
        setLoading(false);
      });
    const timer = setInterval(refresh, 4000);
    return () => clearInterval(timer);
  }, [api, refresh]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 4500);
    return () => clearTimeout(timer);
  }, [toast]);
  const perform = async (key, fn) => {
    setBusy(key);
    try {
      await fn();
      await refresh();
    } catch (e) {
      setToast(e.message);
    } finally {
      setBusy("");
    }
  };
  const start = (p) =>
    perform(p.id, async () => {
      await api("/jobs", {
        method: "POST",
        body: JSON.stringify({ project_id: p.id }),
      });
      setToast("Test sıraya alındı. Worker hazır olduğunda başlayacak.");
    });
  const openReport = async (j) => {
    try {
      setSelected(await api(`/jobs/${j.id}`));
    } catch (e) {
      setToast(e.message);
    }
  };
  useEffect(() => {
    if (selected) {
      const updated = jobs.find((j) => j.id === selected.id);
      if (updated) setSelected(updated);
    }
  }, [jobs]);
  const showForm = (p) => {
    if (!session) {
      setToast("Bağlantı kurulamadı; sayfayı yenileyin.");
      return;
    }
    setForm(
      p
        ? { ...p, config: { ...p.config, checks: p.config.checks || [] } }
        : {
            name: "",
            backup_pattern: "",
            timezone: "Europe/Istanbul",
            schedule_time: "",
            config: { ...session.default_config, checks: [] },
          },
    );
  };
  const total = jobs.length,
    passed = jobs.filter((j) => j.status === "passed").length,
    failed = jobs.filter((j) =>
      ["failed", "interrupted"].includes(j.status),
    ).length,
    running = jobs.filter(active).length;
  const visibleProjects = projects.filter((p) =>
    p.name.toLocaleLowerCase("tr").includes(search.toLocaleLowerCase("tr")),
  );
  const visibleJobs = jobs.filter(
    (j) =>
      (filter === "all" || j.status === filter) &&
      j.project_name
        .toLocaleLowerCase("tr")
        .includes(search.toLocaleLowerCase("tr")),
  );
  const title = {
    overview: "Genel bakış",
    projects: "Projeler",
    history: "Test geçmişi",
    guide: "Kurulum rehberi",
  }[page];
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a
          className="brand"
          href="#"
          onClick={(e) => {
            e.preventDefault();
            setPage("overview");
          }}
        >
          <Mark />
          <span>
            restivra<span className="brand-dot">.</span>
          </span>
        </a>
        <div className="workspace">
          <span className="workspace-avatar">R</span>
          <div>
            <strong>Çalışma alanım</strong>
            <small>Kendi sunucunda · PostgreSQL 17</small>
          </div>
          <ShieldCheck size={16} />
        </div>
        <div className="nav-label">ÇALIŞMA ALANI</div>
        <nav>
          {[
            ["overview", LayoutDashboard, "Genel bakış"],
            ["projects", Database, "Projeler"],
            ["history", Activity, "Test geçmişi"],
          ].map(([id, Icon, label]) => (
            <button
              key={id}
              className={page === id ? "nav-item selected" : "nav-item"}
              onClick={() => {
                setPage(id);
                setSearch("");
              }}
            >
              <Icon size={19} />
              {label}
              {id === "projects" && <span>{projects.length}</span>}
              {id === "history" && running > 0 && <i />}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="assurance">
            <ShieldCheck size={22} />
            <strong>Verin senin alanında.</strong>
            <p>Geri yükleme ortamları her testten sonra temizlenir.</p>
            <button onClick={() => setPage("guide")}>
              Nasıl çalışır? <ArrowUpRight size={14} />
            </button>
          </div>
          <button
            className={`nav-item ${page === "guide" ? "selected" : ""}`}
            onClick={() => setPage("guide")}
          >
            <CircleHelp size={18} />
            Kurulum rehberi
          </button>
          <div className="version">
            <span className="tiny-dot" />
            Restivra <span>v0.1.0</span>
          </div>
        </div>
      </aside>
      <div className="main-shell">
        <header className="topbar">
          <div className="breadcrumb">
            Çalışma alanı <ChevronRight size={14} />
            <strong>{title}</strong>
          </div>
          <div className="top-actions">
            <span
              className={`worker-indicator ${health?.worker_online ? "online" : ""}`}
            >
              <i />
              {health?.worker_online ? "Worker bağlı" : "Worker bekleniyor"}
            </span>
            <button
              className="icon-btn"
              title="Verileri yenile"
              aria-label="Verileri yenile"
              onClick={refresh}
            >
              <RefreshCw size={17} />
            </button>
            <span className="profile">R</span>
          </div>
        </header>
        <main>
          <div className="page-heading">
            <div>
              <div className="eyebrow">GERİ YÜKLEME MERKEZİ</div>
              <h1>{title}</h1>
              <p>
                {page === "overview"
                  ? "Yedeklerinin geri yüklenebildiğini düzenli olarak doğrula."
                  : page === "projects"
                    ? "Müşterilerinin yedeklerini tek bir yerden yönet."
                    : page === "history"
                      ? "Her çalışmanın sonucunu ve doğrulama raporunu incele."
                      : "İlk geri yükleme testine birkaç adımda hazırlan."}
              </p>
            </div>
            {page !== "guide" && (
              <button className="button primary" onClick={() => showForm()}>
                <Plus size={17} />
                Yeni proje
              </button>
            )}
          </div>
          {error && (
            <div className="notice danger">
              <XCircle size={18} />
              <span>Bağlantı hatası: {error}</span>
              <button onClick={refresh}>Tekrar dene</button>
            </div>
          )}
          {!loading && health && !health.worker_online && (
            <div className="notice">
              <TriangleAlert size={18} />
              <span>
                Worker henüz bağlı değil. Sıraya alınan testler worker
                başladığında çalışır.
              </span>
              <button onClick={() => setPage("guide")}>Kurulumu incele</button>
            </div>
          )}
          {loading ? (
            <div className="loading">
              <LoaderCircle className="spin" />
              Çalışma alanı yükleniyor…
            </div>
          ) : (
            <>
              {page === "overview" && (
                <>
                  <section className="summary-grid">
                    {[
                      [
                        Database,
                        "Aktif proje",
                        projects.length,
                        "Tanımlı yedek kaynağı",
                        "purple",
                      ],
                      [
                        ShieldCheck,
                        "Başarılı test",
                        passed,
                        total
                          ? `Son ${total} çalışma içinde`
                          : "Henüz çalışma yok",
                        "green",
                      ],
                      [
                        TriangleAlert,
                        "Dikkat gerekiyor",
                        failed,
                        "Başarısız veya yarıda kalan",
                        "orange",
                      ],
                      [
                        Activity,
                        "Aktif çalışma",
                        running,
                        "Çalışan ve sıradaki testler",
                        "blue",
                      ],
                    ].map(([Icon, label, value, hint, color]) => (
                      <div className="stat-card" key={label}>
                        <div className="stat-label">
                          {label}
                          <span className={`stat-icon ${color}`}>
                            <Icon size={18} />
                          </span>
                        </div>
                        <strong>{value.toString().padStart(2, "0")}</strong>
                        <small>{hint}</small>
                      </div>
                    ))}
                  </section>
                  <section className="overview-split">
                    <div className="confidence-card">
                      <div>
                        <span className="section-kicker">
                          HER TESTTE SOMUT BİR SONUÇ
                        </span>
                        <h2>
                          Geri dönüşe
                          <br />
                          hazır ol.
                        </h2>
                        <p>
                          Yedeği aç. Veriyi kontrol et.
                          <br />
                          Sonucu raporla.
                        </p>
                        <button
                          className="button"
                          onClick={() => setPage("history")}
                        >
                          Test sonuçlarına git <ArrowRight size={16} />
                        </button>
                      </div>
                      <div className="restore-art" aria-hidden="true">
                        <div className="orbit one" />
                        <div className="orbit two" />
                        <div className="art-database">
                          <Database size={58} strokeWidth={1.2} />
                          <span>
                            <Check size={19} />
                          </span>
                        </div>
                        <div className="art-tag">
                          <span />
                          RESTORE · VERIFY
                        </div>
                      </div>
                    </div>
                    <div className="card workflow-card">
                      <div className="card-title">
                        <h2>Bir testin yolculuğu</h2>
                        <span className="muted">4 adım</span>
                      </div>
                      {[
                        [
                          Folder,
                          "Yedeği seç",
                          "Klasördeki en yeni eşleşen dosya",
                        ],
                        [
                          Database,
                          "Geri yükle",
                          "Dış ağa kapalı geçici PostgreSQL",
                        ],
                        [
                          FileCheck2,
                          "Kontrolleri çalıştır",
                          "Salt okunur SQL doğrulamaları",
                        ],
                        [
                          ShieldCheck,
                          "Raporla ve temizle",
                          "Sonuç kaydı, geçici ortam temizliği",
                        ],
                      ].map(([Icon, label, text], i) => (
                        <div className="workflow-step" key={label}>
                          <span>
                            <Icon size={17} />
                          </span>
                          <div>
                            <strong>{label}</strong>
                            <small>{text}</small>
                          </div>
                          <b>0{i + 1}</b>
                        </div>
                      ))}
                    </div>
                  </section>
                </>
              )}
              {(page === "overview" || page === "projects") && (
                <section className="card projects-card">
                  <div className="card-title">
                    <div>
                      <h2>
                        Projeler{" "}
                        <span className="count">{projects.length}</span>
                      </h2>
                      {page === "overview" && (
                        <p>Son test sonuçları ve yaklaşan çalışmalar</p>
                      )}
                    </div>
                    {page === "overview" ? (
                      <button
                        className="text-button"
                        onClick={() => setPage("projects")}
                      >
                        Tüm projeler <ArrowUpRight size={15} />
                      </button>
                    ) : (
                      <div className="search">
                        <Search size={16} />
                        <input
                          aria-label="Proje ara"
                          placeholder="Proje ara…"
                          value={search}
                          onChange={(e) => setSearch(e.target.value)}
                        />
                      </div>
                    )}
                  </div>
                  {projects.length === 0 ? (
                    <Empty
                      title="İlk projen için her şey hazır"
                      text="Yedek dosyanın yolunu tanımla ve ilk geri yükleme testini başlat."
                    >
                      <button
                        className="button primary"
                        onClick={() => showForm()}
                      >
                        <Plus size={16} />
                        İlk projeyi oluştur
                      </button>
                      <button
                        className="text-button"
                        onClick={() => setPage("guide")}
                      >
                        Önce kurulum rehberine bak <ArrowRight size={14} />
                      </button>
                    </Empty>
                  ) : (
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>PROJE / YEDEK KAYNAĞI</th>
                            <th>SON TEST</th>
                            <th>SONUÇ</th>
                            <th>SONRAKİ ÇALIŞMA</th>
                            <th />
                          </tr>
                        </thead>
                        <tbody>
                          {visibleProjects
                            .slice(0, page === "overview" ? 5 : 500)
                            .map((p) => (
                              <tr key={p.id}>
                                <td>
                                  <div className="project-name">
                                    <span className="project-avatar">
                                      <Database size={18} />
                                    </span>
                                    <div>
                                      <strong>{p.name}</strong>
                                      <code>{p.backup_pattern}</code>
                                    </div>
                                  </div>
                                </td>
                                <td>{date(p.last_job?.queued_at)}</td>
                                <td>
                                  {p.last_job ? (
                                    <button
                                      className="badge-button"
                                      onClick={() => openReport(p.last_job)}
                                    >
                                      <Badge status={p.last_job.status} />
                                    </button>
                                  ) : (
                                    <span className="muted">
                                      Henüz test yok
                                    </span>
                                  )}
                                </td>
                                <td>
                                  <div>{date(p.next_run_at)}</div>
                                  {p.schedule_time && (
                                    <small className="cell-sub">
                                      {p.timezone}
                                    </small>
                                  )}
                                </td>
                                <td>
                                  <div className="row-actions">
                                    <button
                                      className="button small"
                                      disabled={
                                        active(p.last_job) || busy === p.id
                                      }
                                      onClick={() => start(p)}
                                    >
                                      {busy === p.id ? (
                                        <LoaderCircle
                                          className="spin"
                                          size={14}
                                        />
                                      ) : (
                                        <Play size={13} />
                                      )}
                                      Test et
                                    </button>
                                    <button
                                      className="icon-btn"
                                      title={`${p.name} ayarları`}
                                      aria-label={`${p.name} ayarları`}
                                      onClick={() => showForm(p)}
                                    >
                                      <Settings2 size={16} />
                                    </button>
                                  </div>
                                </td>
                              </tr>
                            ))}
                        </tbody>
                      </table>
                      {!visibleProjects.length && (
                        <div className="no-results">
                          Eşleşen proje bulunamadı.
                        </div>
                      )}
                    </div>
                  )}
                </section>
              )}
              {(page === "overview" || page === "history") && (
                <section className="card history-card">
                  <div className="card-title">
                    <div>
                      <h2>
                        {page === "overview" ? "Son çalışmalar" : "Çalışmalar"}{" "}
                        <span className="count">{jobs.length}</span>
                      </h2>
                      <p>
                        En son 100 çalışma · tüm saatler cihazının saat
                        diliminde
                      </p>
                    </div>
                    {page === "overview" ? (
                      <button
                        className="text-button"
                        onClick={() => setPage("history")}
                      >
                        Geçmişi aç <ArrowUpRight size={15} />
                      </button>
                    ) : (
                      <div className="search">
                        <Search size={16} />
                        <input
                          aria-label="Çalışmalarda proje ara"
                          placeholder="Proje ara…"
                          value={search}
                          onChange={(e) => setSearch(e.target.value)}
                        />
                      </div>
                    )}
                  </div>
                  {page === "history" && (
                    <div className="filters">
                      {[
                        ["all", "Tümü"],
                        ["passed", "Başarılı"],
                        ["failed", "Başarısız"],
                        ["running", "Çalışıyor"],
                        ["queued", "Sırada"],
                        ["cancelled", "İptal"],
                        ["interrupted", "Yarıda kalan"],
                      ].map(([id, label]) => (
                        <button
                          key={id}
                          className={filter === id ? "active" : ""}
                          onClick={() => setFilter(id)}
                        >
                          {label}
                        </button>
                      ))}
                    </div>
                  )}
                  {!jobs.length ? (
                    <Empty
                      icon={Activity}
                      title="Henüz bir test çalışmadı"
                      text="İlk testin ardından geri yükleme sonuçları burada görünecek."
                    />
                  ) : (
                    <div className="table-scroll">
                      <table>
                        <thead>
                          <tr>
                            <th>PROJE</th>
                            <th>BAŞLANGIÇ</th>
                            <th>SONUÇ</th>
                            <th>SÜRE</th>
                            <th>TETİKLEYİCİ</th>
                            <th />
                          </tr>
                        </thead>
                        <tbody>
                          {visibleJobs
                            .slice(0, page === "overview" ? 5 : 100)
                            .map((j) => (
                              <tr key={j.id}>
                                <td>
                                  <strong>{j.project_name}</strong>
                                  <small className="cell-sub mono">
                                    {j.id.slice(0, 8)}
                                  </small>
                                </td>
                                <td>{date(j.started_at || j.queued_at)}</td>
                                <td>
                                  <Badge status={j.status} />
                                </td>
                                <td className="mono">{duration(j)}</td>
                                <td>
                                  <span className="trigger">
                                    {j.trigger === "schedule" ? (
                                      <CalendarClock size={13} />
                                    ) : (
                                      <Play size={12} />
                                    )}{" "}
                                    {j.trigger === "schedule"
                                      ? "Zamanlama"
                                      : "Manuel"}
                                  </span>
                                </td>
                                <td>
                                  <button
                                    className="text-button"
                                    onClick={() => openReport(j)}
                                  >
                                    İncele <ChevronRight size={14} />
                                  </button>
                                </td>
                              </tr>
                            ))}
                        </tbody>
                      </table>
                      {!visibleJobs.length && (
                        <div className="no-results">
                          Bu filtreye uygun çalışma bulunamadı.
                        </div>
                      )}
                    </div>
                  )}
                </section>
              )}
              {page === "guide" && <Guide />}
            </>
          )}
          <footer>
            <span>
              <Mark />
              restivra <i /> Geri yükleme doğrulaması
            </span>
            <span>PostgreSQL 17 · Kendi sunucunda</span>
          </footer>
        </main>
      </div>
      {form && (
        <ProjectForm
          project={form}
          onClose={() => setForm(null)}
          onSave={async (value) => {
            const body = {
              name: value.name,
              backup_pattern: value.backup_pattern,
              config: value.config,
              schedule_time: value.schedule_time,
              timezone: value.timezone,
            };
            await api(form.id ? `/projects/${form.id}` : "/projects", {
              method: form.id ? "PUT" : "POST",
              body: JSON.stringify(body),
            });
            setForm(null);
            setToast("Proje kaydedildi.");
            await refresh();
          }}
          onArchive={
            form.id
              ? () =>
                  perform(form.id, async () => {
                    await api(`/projects/${form.id}`, { method: "DELETE" });
                    setForm(null);
                    setToast("Proje arşivlendi; test geçmişi saklanıyor.");
                  })
              : null
          }
        />
      )}
      {selected && (
        <Report
          job={selected}
          onClose={() => setSelected(null)}
          onCancel={() =>
            perform(selected.id, async () => {
              await api(`/jobs/${selected.id}/cancel`, {
                method: "POST",
                body: "{}",
              });
              setToast("İptal istendi; geçici kaynaklar temizleniyor.");
              setSelected(await api(`/jobs/${selected.id}`));
            })
          }
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <Bell size={17} />
          {toast}
          <button aria-label="Bildirimi kapat" onClick={() => setToast("")}>
            <X size={15} />
          </button>
        </div>
      )}
    </div>
  );
}

function ProjectForm({ project, onClose, onSave, onArchive }) {
  const [value, setValue] = useState(() => structuredClone(project)),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [archiveConfirm, setArchiveConfirm] = useState(false);
  const update = (key, v) => setValue((old) => ({ ...old, [key]: v }));
  const config = (key, v) =>
    setValue((old) => ({ ...old, config: { ...old.config, [key]: v } }));
  const check = (i, key, v) =>
    config(
      "checks",
      value.config.checks.map((c, index) =>
        index === i ? { ...c, [key]: v } : c,
      ),
    );
  const submit = async (e) => {
    e.preventDefault();
    setError("");
    setSaving(true);
    try {
      await onSave(value);
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  };
  return (
    <Modal
      title={project.id ? "Proje ayarları" : "Yeni proje oluştur"}
      subtitle="Yedeğin hazırsa, ilk geri yükleme testi de hazır."
      onClose={onClose}
      wide
    >
      <form onSubmit={submit}>
        <div className="form-body">
          {error && <div className="notice danger">{error}</div>}
          <div className="form-section-label">
            <Database size={16} />
            PROJE BİLGİLERİ
          </div>
          <label>
            Proje adı
            <input
              autoFocus
              required
              maxLength={120}
              placeholder="Örn. Atlas E-ticaret"
              value={value.name}
              onChange={(e) => update("name", e.target.value)}
            />
          </label>
          <label>
            Yedek dosyası veya eşleşmesi
            <input
              required
              placeholder="atlas/*.dump"
              value={value.backup_pattern}
              onChange={(e) => update("backup_pattern", e.target.value)}
            />
            <small>
              Yedek köküne göre bağıl yol. Birden fazla eşleşmede değiştirilme
              zamanı en yeni olan dosya seçilir.
            </small>
          </label>
          <div className="inline-note">
            <Folder size={16} />
            <span>
              PostgreSQL 17 · pg_dump 17 · Custom format (<code>-Fc</code>)
            </span>
          </div>
          <div className="form-section-label">
            <CalendarClock size={16} />
            ZAMANLAMA
          </div>
          <label className="toggle-row">
            <input
              type="checkbox"
              checked={!!value.schedule_time}
              onChange={(e) =>
                update("schedule_time", e.target.checked ? "03:00" : "")
              }
            />
            <span>Her gün otomatik test et</span>
          </label>
          {value.schedule_time && (
            <div className="form-grid">
              <label>
                Çalışma saati
                <input
                  type="time"
                  required
                  value={value.schedule_time}
                  onChange={(e) => update("schedule_time", e.target.value)}
                />
              </label>
              <label>
                Saat dilimi
                <input
                  required
                  placeholder="Europe/Istanbul"
                  value={value.timezone}
                  onChange={(e) => update("timezone", e.target.value)}
                />
              </label>
            </div>
          )}
          <div className="form-section-label">
            <FileCheck2 size={16} />
            VERİ KONTROLLERİ <span>İsteğe bağlı</span>
          </div>
          <p className="field-help">
            Tek bir boolean döndüren SELECT sorguları ekle. Noktalı virgül ve
            SQL yorumu kullanma.
          </p>
          {value.config.checks.map((c, i) => (
            <div className="check-editor" key={i}>
              <div>
                <input
                  required
                  aria-label={`Kontrol ${i + 1} adı`}
                  placeholder="Kontrol adı"
                  value={c.name}
                  onChange={(e) => check(i, "name", e.target.value)}
                />
                <button
                  type="button"
                  className="icon-btn"
                  aria-label={`Kontrol ${i + 1} sil`}
                  onClick={() =>
                    config(
                      "checks",
                      value.config.checks.filter((_, n) => n !== i),
                    )
                  }
                >
                  <Trash2 size={15} />
                </button>
              </div>
              <textarea
                required
                aria-label={`Kontrol ${i + 1} SQL`}
                spellCheck={false}
                placeholder="SELECT count(*) > 0 FROM public.customers"
                value={c.sql}
                onChange={(e) => check(i, "sql", e.target.value)}
              />
            </div>
          ))}
          <button
            type="button"
            className="button small"
            disabled={value.config.checks.length >= 32}
            onClick={() =>
              config("checks", [...value.config.checks, { name: "", sql: "" }])
            }
          >
            <Plus size={14} />
            Kontrol ekle
          </button>
          <details className="advanced">
            <summary>Kaynak sınırları</summary>
            <div className="form-grid">
              <label>
                Toplam süre (saniye)
                <input
                  type="number"
                  min="5"
                  max="3600"
                  required
                  value={value.config.timeout_seconds}
                  onChange={(e) =>
                    config("timeout_seconds", Number(e.target.value))
                  }
                />
              </label>
              <label>
                Veri alanı (MiB)
                <input
                  type="number"
                  min="128"
                  max="8192"
                  required
                  value={value.config.data_mb}
                  onChange={(e) => config("data_mb", Number(e.target.value))}
                />
              </label>
              <label>
                Bellek sınırı (MiB)
                <input
                  type="number"
                  min={value.config.data_mb + 256}
                  max="16384"
                  required
                  value={value.config.memory_mb}
                  onChange={(e) => config("memory_mb", Number(e.target.value))}
                />
              </label>
              <label>
                En büyük yedek (MiB)
                <input
                  type="number"
                  min="1"
                  max="10240"
                  required
                  value={value.config.max_backup_bytes / 1048576}
                  onChange={(e) =>
                    config("max_backup_bytes", Number(e.target.value) * 1048576)
                  }
                />
              </label>
            </div>
          </details>
          {onArchive && (
            <div className="archive-zone">
              {archiveConfirm ? (
                <>
                  <span>Proje arşivlensin mi? Geçmişi saklanacak.</span>
                  <button
                    type="button"
                    className="text-button danger-text"
                    onClick={onArchive}
                  >
                    Evet, arşivle
                  </button>
                  <button
                    type="button"
                    className="text-button"
                    onClick={() => setArchiveConfirm(false)}
                  >
                    Vazgeç
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className="text-button danger-text"
                  onClick={() => setArchiveConfirm(true)}
                >
                  <Trash2 size={14} />
                  Projeyi arşivle
                </button>
              )}
            </div>
          )}
        </div>
        <div className="modal-footer">
          <button type="button" className="button" onClick={onClose}>
            Vazgeç
          </button>
          <button disabled={saving} className="button primary">
            {saving ? (
              <LoaderCircle className="spin" size={16} />
            ) : (
              <Check size={16} />
            )}
            Projeyi kaydet
          </button>
        </div>
      </form>
    </Modal>
  );
}
function Modal({ title, subtitle, onClose, children, wide = false }) {
  const ref = useRef(null);
  useEffect(() => {
    const previous = document.activeElement;
    ref.current.showModal();
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = "";
      previous?.focus?.();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className={`modal ${wide ? "wide" : ""}`}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
      onClick={(e) => {
        if (e.target === ref.current) {
          const rect = ref.current.getBoundingClientRect();
          if (
            e.clientX < rect.left ||
            e.clientX > rect.right ||
            e.clientY < rect.top ||
            e.clientY > rect.bottom
          )
            onClose();
        }
      }}
    >
      <div className="modal-header">
        <div>
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        <button
          className="icon-btn"
          aria-label="Pencereyi kapat"
          onClick={onClose}
        >
          <X size={20} />
        </button>
      </div>
      {children}
    </dialog>
  );
}
function Report({ job, onClose, onCancel }) {
  const r = job.report;
  const notices = r?.warnings || [];
  return (
    <Modal
      title="Test raporu"
      subtitle={`${job.project_name} · ${job.id.slice(0, 8)}`}
      onClose={onClose}
      wide
    >
      <div className="form-body report-body">
        <div className={`report-status ${job.status}`}>
          <div>
            {job.status === "passed" ? (
              <ShieldCheck size={28} />
            ) : active(job) ? (
              <LoaderCircle size={28} className="spin" />
            ) : (
              <TriangleAlert size={28} />
            )}
            <div>
              <h3>{labels[job.status]}</h3>
              <p>
                {job.status === "passed"
                  ? "Geri yükleme ve tanımlı kontroller tamamlandı."
                  : active(job)
                    ? "İş tamamlandığında sonuçlar otomatik olarak güncellenecek."
                    : job.error || "Bu çalışma tamamlanamadı."}
              </p>
            </div>
          </div>
          <Badge status={job.status} />
        </div>
        <div className="report-meta">
          <div>
            <small>BAŞLANGIÇ</small>
            <strong>{date(job.started_at || job.queued_at, true)}</strong>
          </div>
          <div>
            <small>SÜRE</small>
            <strong>{duration(job)}</strong>
          </div>
          <div>
            <small>TETİKLEYİCİ</small>
            <strong>
              {job.trigger === "schedule" ? "Günlük zamanlama" : "Manuel"}
            </strong>
          </div>
        </div>
        {r ? (
          <>
            <div className="form-section-label">
              <FileCheck2 size={16} />
              DOĞRULAMA ADIMLARI
            </div>
            <div className="result-row">
              <div>
                <Database size={17} />
                <strong>Geri yükleme</strong>
              </div>
              <Badge status={r.restore_status} />
            </div>
            {r.checks.map((c, i) => (
              <div className="result-row" key={i}>
                <div>
                  <FileCheck2 size={17} />
                  <span>
                    <strong>{c.name}</strong>
                    {c.error && <small>{c.error}</small>}
                  </span>
                </div>
                <Badge status={c.status} />
              </div>
            ))}
            {!r.checks.length && (
              <p className="field-help">
                Özel SQL kontrolü tanımlanmamış. Uygulama verisi doğrulanmadı.
              </p>
            )}
            <div className="result-row">
              <div>
                <Trash2 size={17} />
                <strong>Geçici ortam temizliği</strong>
              </div>
              <Badge status={r.cleanup_status} />
            </div>
            <div className="form-section-label">
              <HardDrive size={16} />
              TEST EDİLEN YEDEK
            </div>
            <div className="backup-detail">
              <div>
                <span>Dosya</span>
                <strong>{r.backup.name}</strong>
              </div>
              <div>
                <span>Boyut</span>
                <strong>{bytes(r.backup.bytes)}</strong>
              </div>
              <div>
                <span>Dosya değiştirilme zamanı</span>
                <strong>{date(r.backup.modified_at, true)}</strong>
              </div>
              <div>
                <span>PostgreSQL / pg_dump</span>
                <strong>
                  {r.backup.source_version || "—"} /{" "}
                  {r.backup.dump_version || "—"}
                </strong>
              </div>
              <div className="hash">
                <span>SHA-256</span>
                <code>{r.backup.sha256 || "Hesaplanamadı"}</code>
              </div>
            </div>
            <details className="advanced">
              <summary>Kapsam ve uyarılar ({notices.length})</summary>
              <p className="field-help">
                Bu test, mantıksal geri yüklemeyi ve seçili sorguları doğrular.
                Tam uygulama kurtarmasını, orijinal yetkileri veya yedeğin
                güncelliğini garanti etmez.
              </p>
              {notices.map((n, i) => (
                <div className="report-warning" key={i}>
                  <TriangleAlert size={14} />
                  <span>{n}</span>
                </div>
              ))}
            </details>
          </>
        ) : (
          !active(job) && (
            <div className="notice">
              Bu çalışma geri yükleme raporu üretmeden sonlandı. Hata özeti
              yukarıda.
            </div>
          )
        )}
        <div className="notification-line">
          <Bell size={14} />
          Bildirim:{" "}
          {
            {
              skipped: "Yapılandırılmadı / gerekli değil",
              pending: "Teslim bekliyor",
              delivered: "Teslim edildi",
              failed: "Teslim edilemedi",
            }[job.notification_status]
          }
          {job.notification_attempts > 0 &&
            ` · ${job.notification_attempts} deneme`}
        </div>
      </div>
      <div className="modal-footer">
        {active(job) && (
          <button className="button danger-text" onClick={onCancel}>
            <Square size={13} />
            Testi iptal et
          </button>
        )}
        <button className="button" onClick={onClose}>
          Kapat
        </button>
        {r && (
          <a
            className="button primary"
            href={`/api/jobs/${job.id}/report`}
            download
          >
            <ArrowDownToLine size={16} />
            JSON indir
          </a>
        )}
      </div>
    </Modal>
  );
}
function Guide() {
  return (
    <div className="guide-grid">
      <section className="card guide-main">
        <div className="card-title">
          <h2>İlk testine başla</h2>
          <span className="count">4 adım</span>
        </div>
        {[
          [
            "Yedek dosyanı klasöre koy",
            "PostgreSQL 17 üzerinde pg_dump 17 ile alınmış, tamamlanmış bir custom dump dosyası kullan. Compose kurulumunda dosyaları projenin backups/ klasörüne yerleştir.",
            "pg_dump -Fc -f customer.dump your_database",
          ],
          [
            "Bir proje oluştur",
            "Projene müşteri adını ver ve yedek klasörüne göre bağıl dosya yolunu gir. *.dump eşleşmesinde dosya değiştirilme zamanı en yeni olan seçilir.",
            "customer/*.dump",
          ],
          [
            "Veri kontrollerini tanımla",
            "Uygulaman için anlamlı bir kontrol ekle. Sorgu tek bir true / false değeri döndürmeli. Kontroller isteğe bağlıdır.",
            "SELECT count(*) > 0 FROM public.customers",
          ],
          [
            "Test et ve raporu incele",
            "Manuel test başlat veya günlük saat belirle. Worker geri yüklemeyi yapar, kontrolleri çalıştırır ve geçici ortamı kaldırır.",
            null,
          ],
        ].map(([title, text, code], i) => (
          <div className="guide-step" key={title}>
            <span>0{i + 1}</span>
            <div>
              <h3>{title}</h3>
              <p>{text}</p>
              {code && (
                <pre>
                  <code>{code}</code>
                </pre>
              )}
            </div>
          </div>
        ))}
      </section>
      <div>
        <section className="card guide-note">
          <ShieldCheck size={25} />
          <h3>Testin kapsamı</h3>
          <p>
            Başarılı sonuç, yedeğin geri yüklendiği ve tanımlanan kontrollerin
            geçtiği anlamına gelir.
          </p>
          <p>
            Uygulamanın tamamının kurtarılabildiği, orijinal yetkilerin
            korunduğu veya yedeğin güncel olduğu ayrıca doğrulanmalıdır.
          </p>
        </section>
        <section className="card guide-note">
          <Terminal size={25} />
          <h3>Worker bağlantısı</h3>
          <p>
            Docker Compose kurulumunda worker otomatik başlar. Sorun varsa
            sunucuda servis günlüklerini incele.
          </p>
          <pre>
            <code>docker compose logs worker</code>
          </pre>
          <p>
            Başarısız işler için HTTPS webhook adresini sunucudaki{" "}
            <code>.env</code> dosyasında <code>RESTIVRA_WEBHOOK_URL</code> ile
            tanımlayabilirsin.
          </p>
        </section>
      </div>
    </div>
  );
}
createRoot(document.getElementById("root")).render(<App />);
