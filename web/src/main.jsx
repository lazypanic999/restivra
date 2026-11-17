import React, { useState, useEffect, useCallback, useRef, useId } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource/public-sans/400.css";
import "@fontsource/public-sans/600.css";
import { LanguageProvider, useLanguage } from "./language.jsx";
import "./styles.css";

const active = (job) => job && ["queued", "running"].includes(job.status);
const size = (value) =>
  value >= 1073741824
    ? `${(value / 1073741824).toFixed(2)} GiB`
    : value >= 1048576
      ? `${(value / 1048576).toFixed(2)} MiB`
      : value >= 1024
        ? `${(value / 1024).toFixed(2)} KiB`
        : value
          ? `${value} B`
          : "—";

function Status({ value }) {
  const { t } = useLanguage();
  return (
    <span className={`status status-${value}`}>
      <i aria-hidden="true" />
      {t(`status.${value}`)}
    </span>
  );
}
function Button({ children, primary = false, className = "", ...props }) {
  return (
    <button
      {...props}
      className={`button ${primary ? "primary" : ""} ${className}`}
    >
      {children}
    </button>
  );
}
function Empty({ title, text, children }) {
  return (
    <div className="empty-state">
      <h3>{title}</h3>
      <p>{text}</p>
      {children}
    </div>
  );
}

function App() {
  const { language, setLanguage, t, explain, formatDate } = useLanguage();
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
    [toast, setToast] = useState(""),
    [updated, setUpdated] = useState(null);
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
    if (!response.ok)
      throw Error(data.error || "The request could not be completed.");
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
      setUpdated(new Date().toISOString());
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
    const timer = setTimeout(() => setToast(""), 5000);
    return () => clearTimeout(timer);
  }, [toast]);
  useEffect(() => {
    if (selected) {
      const current = jobs.find((job) => job.id === selected.id);
      if (current) setSelected(current);
    }
  }, [jobs]);
  const perform = async (key, action) => {
    setBusy(key);
    try {
      await action();
      await refresh();
    } catch (e) {
      setToast(e.message);
    } finally {
      setBusy("");
    }
  };
  const start = (project) =>
    perform(project.id, async () => {
      await api("/jobs", {
        method: "POST",
        body: JSON.stringify({ project_id: project.id }),
      });
      setToast({ key: "queuedToast" });
    });
  const inspect = async (job) => {
    try {
      setSelected(await api(`/jobs/${job.id}`));
    } catch (e) {
      setToast(e.message);
    }
  };
  const openProject = (project) => {
    if (!session) {
      setToast("invalid session token; refresh the page");
      return;
    }
    setForm(
      project
        ? {
            ...project,
            config: { ...project.config, checks: project.config.checks || [] },
          }
        : {
            name: "",
            backup_pattern: "",
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || "UTC",
            schedule_time: "",
            config: { ...session.default_config, checks: [] },
          },
    );
  };
  const go = (destination) => {
    setPage(destination);
    setSearch("");
    setFilter("all");
  };
  const passed = jobs.filter((job) => job.status === "passed").length,
    failed = jobs.filter((job) =>
      ["failed", "interrupted"].includes(job.status),
    ).length,
    running = jobs.filter(active).length;
  const matches = (name) =>
    name
      .toLocaleLowerCase(language)
      .includes(search.toLocaleLowerCase(language));
  const listedProjects = projects.filter((project) => matches(project.name));
  const listedJobs = jobs.filter(
    (job) =>
      matches(job.project_name) && (filter === "all" || job.status === filter),
  );
  const seconds = (job) =>
    job.report
      ? (job.report.duration_ms / 1000).toFixed(1)
      : job.started_at && job.finished_at
        ? (
            (new Date(job.finished_at) - new Date(job.started_at)) /
            1000
          ).toFixed(1)
        : null;
  const runDuration = (job) =>
    seconds(job) !== null ? `${seconds(job)} ${t("seconds")}` : "—";
  return (
    <>
      <a className="skip-link" href="#records">
        {t("skipContent")}
      </a>
      <header className="app-header">
        <div className="header-inner">
          <a
            className="wordmark"
            href="#"
            onClick={(event) => {
              event.preventDefault();
              go("overview");
            }}
          >
            Restivra
          </a>
          <nav aria-label="Restivra">
            {["overview", "projects", "history", "setup"].map((id) => (
              <button
                key={id}
                aria-current={page === id ? "page" : undefined}
                onClick={() => go(id)}
              >
                {t(id)}
              </button>
            ))}
          </nav>
          <label className="language-switch">
            <span>{t("language")}</span>
            <select
              aria-label={t("language")}
              value={language}
              onChange={(event) => setLanguage(event.target.value)}
            >
              <option value="en">English</option>
              <option value="tr">Türkçe</option>
            </select>
          </label>
        </div>
      </header>
      <main id="records" className="workspace">
        <div className="page-heading">
          <div>
            <h1>{page === "overview" ? t("heading") : t(page)}</h1>
            <p>
              {page === "overview"
                ? t("description")
                : page === "projects"
                  ? t("projectsDescription")
                  : page === "history"
                    ? t("historyDescription")
                    : t("setupDescription")}
            </p>
          </div>
          {page !== "setup" && (
            <Button primary onClick={() => openProject()} disabled={!session}>
              {t("newProject")}
            </Button>
          )}
        </div>
        <div className="system-line">
          <span
            className={`health ${health?.worker_online ? "connected" : "disconnected"}`}
          >
            <i />
            {t(health?.worker_online ? "workerOnline" : "workerOffline")}
          </span>
          {health && <span>{t("databaseOnline")}</span>}
          <span>PostgreSQL 17</span>
          <div className="update-controls">
            {updated && (
              <span>
                {t("lastUpdated", {
                  time: new Date(updated).toLocaleTimeString(
                    language === "tr" ? "tr-TR" : "en-GB",
                    { hour: "2-digit", minute: "2-digit", second: "2-digit" },
                  ),
                })}
              </span>
            )}
            <button className="text-button" onClick={refresh}>
              {t("refresh")}
            </button>
          </div>
        </div>
        {error && (
          <div className="notice error-notice" role="alert">
            <strong>{t("connectionError")}</strong> {explain(error)}{" "}
            <button onClick={refresh}>{t("refresh")}</button>
          </div>
        )}
        {!loading && health && !health.worker_online && (
          <div className="notice">{t("offlineNotice")}</div>
        )}
        {loading ? (
          <div className="loading" role="status">
            {t("loading")}
          </div>
        ) : (
          <>
            {page === "overview" && (
              <div className="summary-line">
                <p>
                  {t("summary", {
                    projects: projects.length,
                    passed,
                    failed,
                    active: running,
                  })}
                </p>
                <small>{t("recentWindow", { count: jobs.length })}</small>
              </div>
            )}
            {(page === "overview" || page === "projects") && (
              <section className="ledger" aria-labelledby="project-heading">
                <div className="section-heading">
                  <h2 id="project-heading">{t("projects")}</h2>
                  {page === "projects" && (
                    <label className="search-field">
                      <span>{t("searchProjects")}</span>
                      <input
                        type="search"
                        value={search}
                        onChange={(event) => setSearch(event.target.value)}
                      />
                    </label>
                  )}
                </div>
                {!projects.length ? (
                  <Empty title={t("noProjects")} text={t("noProjectsText")}>
                    <Button onClick={() => openProject()}>
                      {t("createProject")}
                    </Button>
                  </Empty>
                ) : (
                  <div className="table-scroll">
                    <table>
                      <thead>
                        <tr>
                          <th>{t("project")}</th>
                          <th>{t("latestResult")}</th>
                          <th>{t("lastRun")}</th>
                          <th>{t("nextRun")}</th>
                          <th className="action-column">{t("actions")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {listedProjects.map((project) => (
                          <tr key={project.id}>
                            <td>
                              <strong className="project-name">
                                {project.name}
                              </strong>
                              <code className="file-path">
                                {project.backup_pattern}
                              </code>
                            </td>
                            <td>
                              {project.last_job ? (
                                <button
                                  className="status-button"
                                  onClick={() => inspect(project.last_job)}
                                >
                                  <Status value={project.last_job.status} />
                                </button>
                              ) : (
                                <span className="secondary">
                                  {t("notTested")}
                                </span>
                              )}
                            </td>
                            <td>{formatDate(project.last_job?.queued_at)}</td>
                            <td>
                              {project.next_run_at ? (
                                <>
                                  <span>{formatDate(project.next_run_at)}</span>
                                  <small className="cell-note">
                                    {project.timezone}
                                  </small>
                                </>
                              ) : (
                                <span className="secondary">
                                  {t("manualOnly")}
                                </span>
                              )}
                            </td>
                            <td>
                              <div className="row-actions">
                                <Button
                                  disabled={
                                    active(project.last_job) ||
                                    busy === project.id
                                  }
                                  onClick={() => start(project)}
                                >
                                  {t("runTest")}
                                </Button>
                                <button
                                  className="text-button"
                                  aria-label={`${t("edit")}: ${project.name}`}
                                  onClick={() => openProject(project)}
                                >
                                  {t("edit")}
                                </button>
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                    {!listedProjects.length && (
                      <p className="no-results">{t("noMatches")}</p>
                    )}
                  </div>
                )}
              </section>
            )}
            {(page === "overview" || page === "history") && (
              <section className="ledger" aria-labelledby="run-heading">
                <div className="section-heading">
                  <h2 id="run-heading">
                    {page === "overview" ? t("recentRuns") : t("history")}
                  </h2>
                  {page === "overview" ? (
                    <button
                      className="text-button"
                      onClick={() => go("history")}
                    >
                      {t("allRuns")}
                    </button>
                  ) : (
                    <div className="table-filters">
                      <label className="search-field">
                        <span>{t("searchRuns")}</span>
                        <input
                          type="search"
                          value={search}
                          onChange={(event) => setSearch(event.target.value)}
                        />
                      </label>
                      <label className="search-field">
                        <span>{t("statusFilter")}</span>
                        <select
                          value={filter}
                          onChange={(event) => setFilter(event.target.value)}
                        >
                          <option value="all">{t("allResults")}</option>
                          {[
                            "passed",
                            "failed",
                            "running",
                            "queued",
                            "cancelled",
                            "interrupted",
                          ].map((value) => (
                            <option key={value} value={value}>
                              {t(`status.${value}`)}
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>
                  )}
                </div>
                {!jobs.length ? (
                  <Empty title={t("noRuns")} text={t("noRunsText")} />
                ) : (
                  <div className="table-scroll">
                    <table>
                      <thead>
                        <tr>
                          <th>{t("project")}</th>
                          <th>{t("result")}</th>
                          <th>{t("started")}</th>
                          <th className="numeric">{t("duration")}</th>
                          <th>{t("trigger")}</th>
                          <th className="action-column">{t("report")}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {listedJobs
                          .slice(0, page === "overview" ? 6 : 100)
                          .map((job) => (
                            <tr key={job.id}>
                              <td>
                                <strong>{job.project_name}</strong>
                                <code className="cell-note run-id">
                                  {job.id.slice(0, 8)}
                                </code>
                              </td>
                              <td>
                                <Status value={job.status} />
                              </td>
                              <td>
                                {formatDate(job.started_at || job.queued_at)}
                              </td>
                              <td className="numeric">{runDuration(job)}</td>
                              <td>
                                {t(
                                  job.trigger === "schedule"
                                    ? "scheduled"
                                    : "manual",
                                )}
                              </td>
                              <td className="action-column">
                                <button
                                  className="text-button"
                                  onClick={() => inspect(job)}
                                >
                                  {t("inspect")}
                                </button>
                              </td>
                            </tr>
                          ))}
                      </tbody>
                    </table>
                    {!listedJobs.length && (
                      <p className="no-results">{t("noMatches")}</p>
                    )}
                  </div>
                )}
              </section>
            )}
            {page === "setup" && <Setup />}
          </>
        )}
        <footer className="app-footer">
          <span>Restivra 0.1.0</span>
          <span>{t("selfHosted")}</span>
          <span>{t("localTime")}</span>
        </footer>
      </main>
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
            setToast({ key: "savedToast" });
            await refresh();
          }}
          onArchive={
            form.id
              ? async () => {
                  await api(`/projects/${form.id}`, { method: "DELETE" });
                  setForm(null);
                  setToast({ key: "archivedToast" });
                  await refresh();
                }
              : null
          }
        />
      )}
      {selected && (
        <Report
          job={selected}
          duration={runDuration(selected)}
          onClose={() => setSelected(null)}
          onCancel={() =>
            perform(selected.id, async () => {
              await api(`/jobs/${selected.id}/cancel`, {
                method: "POST",
                body: "{}",
              });
              setToast({ key: "cancelledToast" });
              setSelected(await api(`/jobs/${selected.id}`));
            })
          }
        />
      )}
      {toast && (
        <div className="toast" role="status">
          <span>
            {typeof toast === "string" ? explain(toast) : t(toast.key)}
          </span>
          <button aria-label={t("dismiss")} onClick={() => setToast("")}>
            ×
          </button>
        </div>
      )}
    </>
  );
}

function Modal({ title, description, onClose, children }) {
  const { t } = useLanguage(),
    ref = useRef(null),
    id = useId();
  useEffect(() => {
    const previous = document.activeElement;
    ref.current.showModal();
    ref.current.querySelector("input, textarea, select")?.focus();
    const overflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.body.style.overflow = overflow;
      previous?.focus?.();
    };
  }, []);
  return (
    <dialog
      ref={ref}
      className="modal"
      aria-labelledby={id}
      onCancel={(event) => {
        event.preventDefault();
        onClose();
      }}
    >
      <header className="modal-header">
        <div>
          <h2 id={id}>{title}</h2>
          {description && <p>{description}</p>}
        </div>
        <button
          className="close-button"
          aria-label={t("close")}
          onClick={onClose}
        >
          ×
        </button>
      </header>
      {children}
    </dialog>
  );
}
function ProjectForm({ project, onClose, onSave, onArchive }) {
  const { t, explain } = useLanguage();
  const [value, setValue] = useState(() => structuredClone(project)),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [archiveConfirm, setArchiveConfirm] = useState(false);
  const update = (key, next) => setValue((old) => ({ ...old, [key]: next }));
  const config = (key, next) =>
    setValue((old) => ({ ...old, config: { ...old.config, [key]: next } }));
  const updateCheck = (index, key, next) =>
    config(
      "checks",
      value.config.checks.map((check, i) =>
        i === index ? { ...check, [key]: next } : check,
      ),
    );
  const submit = async (event) => {
    event.preventDefault();
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
  const archive = async () => {
    setSaving(true);
    setError("");
    try {
      await onArchive();
    } catch (e) {
      setError(e.message);
    } finally {
      setSaving(false);
    }
  };
  return (
    <Modal
      title={t(project.id ? "projectSettings" : "createProject")}
      onClose={onClose}
    >
      <form onSubmit={submit}>
        <div className="modal-body">
          {error && (
            <div className="notice error-notice" role="alert">
              {explain(error)}
            </div>
          )}
          <label className="field">
            {t("projectName")}
            <input
              autoFocus
              required
              maxLength={120}
              placeholder={t("namePlaceholder")}
              value={value.name}
              onChange={(event) => update("name", event.target.value)}
            />
          </label>
          <label className="field">
            {t("backupPattern")}
            <input
              required
              className="code-input"
              placeholder="customer/*.dump"
              value={value.backup_pattern}
              onChange={(event) => update("backup_pattern", event.target.value)}
            />
            <small>{t("patternHint")}</small>
          </label>
          <p className="format-note">{t("supportedFormat")}</p>
          <fieldset>
            <legend>{t("scheduling")}</legend>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={!!value.schedule_time}
                onChange={(event) =>
                  update("schedule_time", event.target.checked ? "03:00" : "")
                }
              />
              {t("daily")}
            </label>
            {value.schedule_time && (
              <div className="field-grid">
                <label className="field">
                  {t("time")}
                  <input
                    type="time"
                    required
                    value={value.schedule_time}
                    onChange={(event) =>
                      update("schedule_time", event.target.value)
                    }
                  />
                </label>
                <label className="field">
                  {t("timezone")}
                  <input
                    required
                    value={value.timezone}
                    onChange={(event) => update("timezone", event.target.value)}
                  />
                </label>
              </div>
            )}
          </fieldset>
          <fieldset>
            <legend>
              {t("checks")} <small>({t("optional")})</small>
            </legend>
            <p className="help-text">{t("checkHint")}</p>
            {value.config.checks.map((check, index) => (
              <div className="check-editor" key={index}>
                <label className="field">
                  {t("checkName")}
                  <input
                    required
                    aria-label={`${t("checkName")} ${index + 1}`}
                    value={check.name}
                    onChange={(event) =>
                      updateCheck(index, "name", event.target.value)
                    }
                  />
                </label>
                <label className="field">
                  {t("sql")}
                  <textarea
                    required
                    spellCheck={false}
                    aria-label={`${t("sql")} ${index + 1}`}
                    placeholder="SELECT count(*) > 0 FROM public.customers"
                    value={check.sql}
                    onChange={(event) =>
                      updateCheck(index, "sql", event.target.value)
                    }
                  />
                </label>
                <button
                  type="button"
                  className="text-button destructive"
                  onClick={() =>
                    config(
                      "checks",
                      value.config.checks.filter((_, i) => i !== index),
                    )
                  }
                >
                  {t("removeCheck", { number: index + 1 })}
                </button>
              </div>
            ))}
            <Button
              type="button"
              disabled={value.config.checks.length >= 32}
              onClick={() =>
                config("checks", [
                  ...value.config.checks,
                  { name: "", sql: "" },
                ])
              }
            >
              {t("addCheck")}
            </Button>
          </fieldset>
          <details className="advanced">
            <summary>{t("limits")}</summary>
            <div className="field-grid">
              {[
                ["timeout_seconds", "totalTimeout", 5, 3600, 1],
                ["data_mb", "dataLimit", 128, 8192, 1],
                [
                  "memory_mb",
                  "memoryLimit",
                  value.config.data_mb + 256,
                  16384,
                  1,
                ],
                ["max_backup_bytes", "archiveLimit", 1, 10240, 1048576],
              ].map(([key, label, min, max, factor]) => (
                <label className="field" key={key}>
                  {t(label)}
                  <input
                    required
                    type="number"
                    min={min}
                    max={max}
                    step="1"
                    value={value.config[key] / factor}
                    onChange={(event) =>
                      config(key, Number(event.target.value) * factor)
                    }
                  />
                </label>
              ))}
            </div>
          </details>
          {onArchive && (
            <div className="archive-controls">
              {archiveConfirm ? (
                <>
                  <p>{t("archiveQuestion")}</p>
                  <Button
                    type="button"
                    disabled={saving}
                    className="destructive"
                    onClick={archive}
                  >
                    {t("confirmArchive")}
                  </Button>
                  <button
                    type="button"
                    className="text-button"
                    onClick={() => setArchiveConfirm(false)}
                  >
                    {t("cancel")}
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  className="text-button destructive"
                  onClick={() => setArchiveConfirm(true)}
                >
                  {t("archiveProject")}
                </button>
              )}
            </div>
          )}
        </div>
        <footer className="modal-footer">
          <Button type="button" onClick={onClose}>
            {t("cancel")}
          </Button>
          <Button primary disabled={saving}>
            {t(saving ? "saving" : "saveProject")}
          </Button>
        </footer>
      </form>
    </Modal>
  );
}
function Report({ job, duration, onClose, onCancel }) {
  const { t, explain, formatDate } = useLanguage(),
    [copied, setCopied] = useState(false),
    [copyError, setCopyError] = useState("");
  const report = job.report;
  const copy = async () => {
    try {
      await navigator.clipboard.writeText(report.backup.sha256);
      setCopied(true);
    } catch {
      setCopyError(true);
    }
  };
  const notices = report?.warnings || [];
  return (
    <Modal title={t("report")} description={job.project_name} onClose={onClose}>
      <div className="modal-body report-body">
        <div className="report-outcome">
          <Status value={job.status} />
          <code>{job.id}</code>
        </div>
        <p
          className={
            ["failed", "interrupted"].includes(job.status)
              ? "outcome-description destructive"
              : "outcome-description"
          }
        >
          {job.status === "passed"
            ? t("reportPassed")
            : active(job)
              ? t("reportRunning")
              : explain(job.error) || t("reportFailed")}
        </p>
        <dl className="report-facts">
          <div>
            <dt>{t("started")}</dt>
            <dd>{formatDate(job.started_at || job.queued_at, true)}</dd>
          </div>
          <div>
            <dt>{t("duration")}</dt>
            <dd>{duration}</dd>
          </div>
          <div>
            <dt>{t("trigger")}</dt>
            <dd>{t(job.trigger === "schedule" ? "scheduled" : "manual")}</dd>
          </div>
        </dl>
        {report ? (
          <>
            <div className="table-scroll">
              <table className="check-results">
                <thead>
                  <tr>
                    <th>{t("stage")}</th>
                    <th>{t("result")}</th>
                    <th>{t("details")}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr>
                    <td>{t("restore")}</td>
                    <td>
                      <Status value={report.restore_status} />
                    </td>
                    <td>PostgreSQL 17</td>
                  </tr>
                  {report.checks.map((check, index) => (
                    <tr key={index}>
                      <td>{check.name}</td>
                      <td>
                        <Status value={check.status} />
                      </td>
                      <td>{check.error && explain(check.error)}</td>
                    </tr>
                  ))}
                  <tr>
                    <td>{t("cleanup")}</td>
                    <td>
                      <Status value={report.cleanup_status} />
                    </td>
                    <td />
                  </tr>
                </tbody>
              </table>
            </div>
            {!report.checks.length && (
              <p className="help-text">{t("noChecks")}</p>
            )}
            <h3 className="report-section-title">{t("archive")}</h3>
            <dl className="archive-facts">
              <div>
                <dt>{t("file")}</dt>
                <dd>
                  <code>{report.backup.name}</code>
                </dd>
              </div>
              <div>
                <dt>{t("size")}</dt>
                <dd>{size(report.backup.bytes)}</dd>
              </div>
              <div>
                <dt>{t("modified")}</dt>
                <dd>{formatDate(report.backup.modified_at, true)}</dd>
              </div>
              <div>
                <dt>PostgreSQL / pg_dump</dt>
                <dd>
                  {report.backup.source_version || "—"} /{" "}
                  {report.backup.dump_version || "—"}
                </dd>
              </div>
            </dl>
            <div className="fingerprint">
              <div>
                <h4>{t("fingerprint")}</h4>
                {report.backup.sha256 && (
                  <button className="text-button" onClick={copy}>
                    {t(copied ? "copied" : "copyFingerprint")}
                  </button>
                )}
              </div>
              <code>{report.backup.sha256 || "—"}</code>
              {copyError && <p role="alert">{t("copyFailed")}</p>}
            </div>
            <details className="advanced">
              <summary>{t("scope", { count: notices.length })}</summary>
              <ul className="warning-list">
                {notices.map((notice, index) => (
                  <li key={index}>{explain(notice)}</li>
                ))}
              </ul>
            </details>
          </>
        ) : (
          !active(job) && <p className="notice">{t("noReport")}</p>
        )}
        <p className="notification-note">
          {t("notification")}:{" "}
          {t(
            {
              skipped: "notificationSkipped",
              pending: "notificationPending",
              delivered: "notificationDelivered",
              failed: "notificationFailed",
            }[job.notification_status] || "notificationSkipped",
          )}
          {job.notification_attempts > 0 &&
            ` (${t("attempts", { count: job.notification_attempts })})`}
        </p>
      </div>
      <footer className="modal-footer">
        {active(job) && (
          <Button
            className="destructive"
            disabled={job.cancel_requested}
            onClick={onCancel}
          >
            {t(job.cancel_requested ? "cancelling" : "cancelTest")}
          </Button>
        )}
        <Button onClick={onClose}>{t("close")}</Button>
        {report && (
          <a
            className="button primary"
            href={`/api/jobs/${job.id}/report`}
            download
          >
            {t("download")}
          </a>
        )}
      </footer>
    </Modal>
  );
}
function Setup() {
  const { t } = useLanguage(),
    [shell, setShell] = useState("powershell");
  const installation =
    shell === "powershell"
      ? "./scripts/setup.ps1\ndocker pull postgres:17-bookworm\ndocker compose up --build -d"
      : "sh scripts/setup.sh\ndocker pull postgres:17-bookworm\ndocker compose up --build -d";
  return (
    <div className="setup-layout">
      <section>
        <h2>{t("installation")}</h2>
        <label className="field shell-select">
          {t("shell")}
          <select
            value={shell}
            onChange={(event) => setShell(event.target.value)}
          >
            <option value="powershell">PowerShell</option>
            <option value="bash">Bash</option>
          </select>
        </label>
        <pre>
          <code>{installation}</code>
        </pre>
        {[
          [1, "pg_dump -Fc -f customer.dump your_database"],
          [2, "SELECT count(*) > 0 FROM public.customers"],
          [3, null],
        ].map(([number, code]) => (
          <section className="setup-step" key={number}>
            <h3>{t(`setupTitle${number}`)}</h3>
            <p>{t(`setupText${number}`)}</p>
            {code && (
              <pre>
                <code>{code}</code>
              </pre>
            )}
          </section>
        ))}
      </section>
      <aside className="setup-notes">
        <h2>{t("documented")}</h2>
        <p>{t("scopeNote")}</p>
        <p>{t("supportedFormat")}</p>
        <h3>{t("workerOnline")}</h3>
        <p>{t("workerHelp")}</p>
        <pre>
          <code>docker compose logs worker</code>
        </pre>
        <h3>{t("notification")}</h3>
        <p>{t("webhookHelp")}</p>
      </aside>
    </div>
  );
}

createRoot(document.getElementById("root")).render(
  <LanguageProvider>
    <App />
  </LanguageProvider>,
);
