import React, { useState, useEffect, useCallback, useRef, useId } from "react";
import { createRoot } from "react-dom/client";
import "@fontsource/public-sans/400.css";
import "@fontsource/public-sans/600.css";
import { LanguageProvider, useLanguage } from "./language.jsx";
import { History } from "./history.jsx";
import { CheckTemplates } from "./check-templates.jsx";
import { RunComparison } from "./comparison.jsx";
import { SupportDiagnostics } from "./support.jsx";
import {
  exportProject,
  importProject,
  projectTransferLimit,
} from "./project-transfer.js";
import { initialPage, emptyHistoryFilters } from "./history.js";
import "./styles.css";
import "./character.css";

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
function RunState({ job }) {
  const { t, formatDate } = useLanguage();
  return (
    <>
      <Status value={job.status} />
      {job.retry_at ? (
        <small className="attempt-info">
          {t("retryAt", { time: formatDate(job.retry_at, true) })}
        </small>
      ) : (
        job.attempt_count > 1 && (
          <small className="attempt-info">
            {t("attemptNumber", { number: job.attempt_count })}
          </small>
        )
      )}
    </>
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

function LanguageSwitch() {
  const { language, setLanguage, t } = useLanguage();
  return (
    <label className="language-switch">
      <span>{t("language")}</span>
      <select
        aria-label={t("language")}
        value={language}
        onChange={(e) => setLanguage(e.target.value)}
      >
        <option value="en">English</option>
        <option value="tr">Türkçe</option>
      </select>
    </label>
  );
}

// Refresh the per-process CSRF token before a mutation. Existing signed browser
// sessions survive API restarts, while the old process's CSRF token does not.
// Mutations are sent exactly once; ambiguous network failures are never replayed.
async function currentCSRF() {
  const response = await fetch("/api/auth", { cache: "no-store" });
  if (!response.ok) throw Error("The request could not be completed.");
  return (await response.json()).csrf_token;
}

function AuthGate() {
  const { t } = useLanguage();
  const [state, setState] = useState(null),
    [error, setError] = useState(""),
    [secret, setSecret] = useState(""),
    [busy, setBusy] = useState(false);
  const bootstrap = useCallback(async () => {
    try {
      const response = await fetch("/api/auth", { cache: "no-store" });
      if (!response.ok) throw Error();
      setState(await response.json());
      setError("");
    } catch {
      setError("authUnavailable");
    }
  }, []);
  useEffect(() => {
    bootstrap();
  }, [bootstrap]);
  const unauthorized = useCallback(() => {
    setState(null);
    bootstrap();
  }, [bootstrap]);
  async function login(event) {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      const response = await fetch("/api/auth/login", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": await currentCSRF(),
        },
        body: JSON.stringify({ token: secret }),
      });
      if (!response.ok) {
        setError(
          response.status === 429
            ? "authRateLimit"
            : response.status === 401
              ? "authInvalid"
              : "authUnavailable",
        );
        return;
      }
      setSecret("");
      await bootstrap();
    } catch {
      setError("authUnavailable");
    } finally {
      setBusy(false);
    }
  }
  async function logout() {
    try {
      const response = await fetch("/api/auth/logout", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-CSRF-Token": await currentCSRF(),
        },
        body: "{}",
      });
      if (!response.ok && response.status !== 401) throw Error();
      setState(null);
      await bootstrap();
    } catch {
      setError("authUnavailable");
    }
  }
  if (state?.authenticated)
    return (
      <>
        <App
          onUnauthorized={unauthorized}
          onLogout={state.enabled ? logout : null}
        />
        {error && (
          <div className="toast" role="alert">
            {t(error)}
          </div>
        )}
      </>
    );
  return (
    <>
      <header className="app-header">
        <div className="header-inner">
          <span className="wordmark" role="img" aria-label="Restivra">
            Rest
            <span className="wordmark-i" aria-hidden="true">
              ı
            </span>
            vra
          </span>
          <LanguageSwitch />
        </div>
      </header>
      <main className="login-workspace">
        <h1>{t("signIn")}</h1>
        <p>{t("signInDescription")}</p>
        {error && (
          <p className="notice error-notice" role="alert">
            {t(error)}
          </p>
        )}
        {state ? (
          <form onSubmit={login}>
            <label className="field">
              {t("accessToken")}
              <input
                type="password"
                name="access-token"
                autoComplete="current-password"
                required
                maxLength={256}
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                aria-describedby="token-help"
              />
            </label>
            <p className="help-text" id="token-help">
              {t("accessTokenHelp")}
            </p>
            <Button primary disabled={busy}>
              {t(busy ? "signingIn" : "signIn")}
            </Button>
          </form>
        ) : (
          <Button onClick={bootstrap}>
            {t(error ? "refresh" : "loading")}
          </Button>
        )}
      </main>
    </>
  );
}

function App({ onUnauthorized, onLogout }) {
  const { language, t, explain, formatDate } = useLanguage();
  const [page, setPage] = useState(() => initialPage(location.search)),
    [projects, setProjects] = useState([]),
    [projectTotal, setProjectTotal] = useState(0),
    [directoryResult, setDirectoryResult] = useState(null),
    [projectCursors, setProjectCursors] = useState([""]),
    [showArchived, setShowArchived] = useState(false),
    [directoryLoading, setDirectoryLoading] = useState(false),
    [directoryError, setDirectoryError] = useState(""),
    [jobs, setJobs] = useState([]),
    [runSummary, setRunSummary] = useState(null),
    [health, setHealth] = useState(null),
    [sourceProfiles, setSourceProfiles] = useState([]),
    [session, setSession] = useState(null),
    [error, setError] = useState(""),
    [loading, setLoading] = useState(true),
    [form, setForm] = useState(null),
    [selected, setSelected] = useState(null),
    [search, setSearch] = useState(""),
    [busy, setBusy] = useState(""),
    [toast, setToast] = useState(""),
    [updated, setUpdated] = useState(null);
  const token = useRef("");
  const importInput = useRef(null);
  const projectCursor = projectCursors[projectCursors.length - 1];
  const directoryParams = new URLSearchParams({ archived: String(showArchived), q: search.trim(), limit: "50" });
  if (projectCursor) directoryParams.set("cursor", projectCursor);
  const directoryQuery = directoryParams.toString();
  const directoryData = directoryResult?.key === directoryQuery ? directoryResult.data : null;
  const api = useCallback(
    async (path, options = {}) => {
      if (options.method) token.current = await currentCSRF();
      let response;
      try {
        response = await fetch(`/api${path}`, {
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
      } catch (cause) {
        if (cause.name === "AbortError") throw cause;
        throw Error("API connection unavailable; refresh and try again");
      }
      if (response.status === 401) onUnauthorized();
      let data;
      try { data = await response.json(); }
      catch { throw Error("The request could not be completed."); }
      if (!response.ok)
        throw Error(data.error || "The request could not be completed.");
      return data;
    },
    [onUnauthorized],
  );
  const refresh = useCallback(async () => {
    try {
      const [p, history, h, sources] = await Promise.all([
        api("/projects/page?limit=6"),
        api("/history?limit=6"),
        api("/health"),
        api("/sources"),
      ]);
      setProjects(p.items);
      setProjectTotal(p.active_total);
      setJobs(history.items);
      setRunSummary(history.summary);
      setHealth(h);
      setSourceProfiles(sources.profiles);
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
    if (page !== "projects") return;
    let disposed = false;
    const controller = new AbortController();
    setDirectoryLoading(true);
    setDirectoryError("");
    api(`/projects/page?${directoryQuery}`, { signal: controller.signal })
      .then((data) => {
        if (!disposed) {
          setDirectoryResult({ key: directoryQuery, data });
          setDirectoryError("");
        }
      })
      .catch((e) => {
        if (!disposed) setDirectoryError(e.message);
      })
      .finally(() => {
        if (!disposed) setDirectoryLoading(false);
      });
    return () => {
      disposed = true;
      controller.abort();
    };
  }, [api, page, directoryQuery, updated]);
  useEffect(() => {
    if (!toast) return;
    const timer = setTimeout(() => setToast(""), 5000);
    return () => clearTimeout(timer);
  }, [toast]);
  useEffect(() => {
    if (!selected?.id) return;
    let disposed = false;
    api(`/jobs/${selected.id}`)
      .then((job) => {
        if (!disposed) setSelected(job);
      })
      .catch(() => {});
    return () => {
      disposed = true;
    };
  }, [jobs, selected?.id, api]);
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
  const go = (destination, projectID = "") => {
    setPage(destination);
    setSearch("");
    setProjectCursors([""]);
    const query = new URLSearchParams(location.search);
    if (destination === "history") {
      query.set("view", "history");
      if (projectID) {
        for (const key of Object.keys(emptyHistoryFilters)) query.delete(key);
        query.set("project_id", projectID);
      }
    } else {
      if (destination === "projects" || destination === "setup") query.set("view", destination);
      else query.delete("view");
      for (const key of Object.keys(emptyHistoryFilters)) query.delete(key);
    }
    window.history.replaceState(
      null,
      "",
      `${location.pathname}${query.size ? `?${query}` : ""}${location.hash}`,
    );
  };
  const readProjectFile = async (event) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    try {
      if (file.size > projectTransferLimit) throw Error("transferInvalid");
      setForm({ ...importProject(await file.text()), imported: true });
    } catch {
      setToast({ key: "transferInvalid" });
    }
  };
  const passed = runSummary?.passed || 0,
    failed = (runSummary?.failed || 0) + (runSummary?.interrupted || 0),
    running = (runSummary?.queued || 0) + (runSummary?.running || 0);
  const visibleProjects =
    page === "projects" ? (directoryData?.items || []) : projects;
  const listedProjects = visibleProjects;
  const seconds = (job) =>
    job.duration_ms != null
      ? (job.duration_ms / 1000).toFixed(1)
      : job.report
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
            aria-label="Restivra"
            href="#"
            onClick={(event) => {
              event.preventDefault();
              go("overview");
            }}
          >
            Rest
            <span className="wordmark-i" aria-hidden="true">
              ı
            </span>
            vra
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
          <LanguageSwitch />
          {onLogout && (
            <button className="text-button" onClick={onLogout}>
              {t("signOut")}
            </button>
          )}
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
            <div className="page-actions">
              {page === "projects" && (
                <>
                  <input
                    ref={importInput}
                    type="file"
                    hidden
                    accept="application/json,.json"
                    onChange={readProjectFile}
                    aria-label={t("importProject")}
                  />
                  <Button
                    onClick={() => importInput.current.click()}
                    disabled={!session}
                  >
                    {t("importProject")}
                  </Button>
                </>
              )}
              <Button primary onClick={() => openProject()} disabled={!session}>
                {t("newProject")}
              </Button>
            </div>
          )}
        </div>
        <div className="system-line">
          <span
            className={`health ${!error && health?.worker_online ? "connected" : "disconnected"}`}
          >
            <i />
            {t(error || !health ? "systemUnknown" : health.worker_online ? "workerOnline" : "workerOffline")}
          </span>
          {health && !error && <span>{t("databaseOnline")}</span>}
          {health && !error && (
            <span>
              {t(
                health.authentication_enabled
                  ? "protectedAccess"
                  : "localAccess",
              )}
            </span>
          )}
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
                    projects: projectTotal,
                    passed,
                    failed,
                    cancelled: runSummary?.cancelled || 0,
                    active: running,
                  })}
                </p>
                <small>
                  {t("retainedWindow", { count: runSummary?.total || 0 })}
                </small>
              </div>
            )}
            {(page === "overview" || page === "projects") && (
              <section className="ledger" aria-labelledby="project-heading">
                <div className="section-heading">
                  <h2 id="project-heading">{t("projects")}</h2>
                  {page === "projects" && (
                    <div className="table-filters">
                      <label className="search-field">
                        <span>{t("searchProjects")}</span>
                        <input
                          type="search"
                          value={search}
                          maxLength={120}
                          onChange={(event) => { setSearch(event.target.value); setProjectCursors([""]); }}
                        />
                      </label>
                      <label className="search-field">
                        <span>{t("projectView")}</span>
                        <select
                          aria-label={t("projectView")}
                          value={showArchived ? "archived" : "active"}
                          onChange={(event) => {
                            setDirectoryLoading(
                              event.target.value === "archived",
                            );
                            setDirectoryError("");
                            setShowArchived(event.target.value === "archived");
                            setSearch("");
                            setProjectCursors([""]);
                          }}
                        >
                          <option value="active">{t("activeProjects")}</option>
                          <option value="archived">
                            {t("archivedProjects")}
                          </option>
                        </select>
                      </label>
                    </div>
                  )}
                </div>
                {page === "projects" && directoryError && (
                  <div className="notice error-notice" role="alert">
                    {explain(directoryError)}{" "}
                    <button onClick={refresh}>{t("refresh")}</button>
                  </div>
                )}
                {page === "projects" &&
                  showArchived &&
                  visibleProjects.length > 0 && (
                    <p className="help-text">{t("archivedProjectsHelp")}</p>
                  )}
                {page === "projects" && directoryError ? null : page === "projects" &&
                  !directoryData ? (
                  <p role="status">{t("loading")}</p>
                ) : page === "projects" && !visibleProjects.length && (search.trim() || projectCursors.length > 1) ? (
                  <p className="no-results">{t("noMatches")}</p>
                ) : page === "projects" &&
                  showArchived &&
                  !visibleProjects.length ? (
                  <Empty
                    title={t("noArchivedProjects")}
                    text={t("archivedProjectsHelp")}
                  />
                ) : !visibleProjects.length ? (
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
                                {project.source?.kind === "s3"
                                  ? `s3://${project.source.bucket}/${project.backup_pattern}`
                                  : project.backup_pattern}
                              </code>
                            </td>
                            <td>
                              {project.last_job ? (
                                <button
                                  className="status-button"
                                  onClick={() => inspect(project.last_job)}
                                >
                                  <RunState job={project.last_job} />
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
                                {project.archived ? (
                                  <Button
                                    disabled={busy === project.id}
                                    onClick={() =>
                                      perform(project.id, async () => {
                                        await api(
                                          `/projects/${project.id}/restore`,
                                          { method: "POST", body: "{}" },
                                        );
                                        setShowArchived(false);
                                        setProjectCursors([""]);
                                        setSearch("");
                                        setToast({ key: "restoredToast" });
                                      })
                                    }
                                  >
                                    {t("restoreProject")}
                                  </Button>
                                ) : (
                                  <>
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
                                  </>
                                )}
                                <button
                                  className="text-button"
                                  onClick={() => go("history", project.id)}
                                >
                                  {t("projectHistory")}
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
                {page === "projects" && directoryData && <nav className="history-pagination" aria-label={t("projectPagination")}>
                  <Button disabled={projectCursors.length === 1 || directoryLoading} onClick={() => setProjectCursors((value) => value.slice(0, -1))}>{t("historyNewer")}</Button>
                  <span>{t("projectPageCount", { number: projectCursors.length, count: directoryData.total })}</span>
                  <Button disabled={!directoryData.next_cursor || directoryLoading} onClick={() => setProjectCursors((value) => [...value, directoryData.next_cursor])}>{t("historyOlder")}</Button>
                  {projectCursors.length > 1 && <button className="text-button" onClick={() => setProjectCursors([""])}>{t("historyLatest")}</button>}
                </nav>}
                {page === "overview" && projectTotal > projects.length && <p className="help-text">{t("overviewProjectWindow", { count: projects.length, total: projectTotal })} <button className="text-button" onClick={() => go("projects")}>{t("allProjects")}</button></p>}
              </section>
            )}
            {page === "history" && (
              <History
                api={api}
                onInspect={inspect}
                onUnauthorized={onUnauthorized}
                revision={updated}
                RunState={RunState}
                Button={Button}
              />
            )}
            {page === "overview" && (
              <section className="ledger" aria-labelledby="run-heading">
                <div className="section-heading">
                  <h2 id="run-heading">{t("recentRuns")}</h2>
                  <button className="text-button" onClick={() => go("history")}>
                    {t("allRuns")}
                  </button>
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
                        {jobs.slice(0, 6).map((job) => (
                          <tr key={job.id}>
                            <td>
                              <strong>{job.project_name}</strong>
                              <code className="cell-note run-id">
                                {job.id.slice(0, 8)}
                              </code>
                            </td>
                            <td>
                              <RunState job={job} />
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
                  </div>
                )}
              </section>
            )}
            {page === "setup" && <Setup sourceProfiles={sourceProfiles} api={api} />}
          </>
        )}
        <footer className="app-footer">
          <span>Restivra {session?.version || "—"}</span>
          <span>{t("selfHosted")}</span>
          <span>{t("localTime")}</span>
        </footer>
      </main>
      {form && (
        <ProjectForm
          project={form}
          sourceProfiles={sourceProfiles}
          onClose={() => setForm(null)}
          onSave={async (value) => {
            const body = {
              name: value.name,
              backup_pattern: value.backup_pattern,
              source: value.source || { kind: "local" },
              config: value.config,
              schedule_time: value.schedule_time,
              timezone: value.timezone,
            };
            await api(form.id ? `/projects/${form.id}` : "/projects", {
              method: form.id ? "PUT" : "POST",
              body: JSON.stringify(body),
            });
            if (!form.id) {
              setProjectCursors([""]);
              setSearch("");
              setShowArchived(false);
            }
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
          key={selected.id}
          job={selected}
          api={api}
          onInspect={inspect}
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
function ProjectForm({ project, sourceProfiles, onClose, onSave, onArchive }) {
  const { t, explain } = useLanguage();
  const [value, setValue] = useState(() => structuredClone(project)),
    [saving, setSaving] = useState(false),
    [error, setError] = useState(""),
    [archiveConfirm, setArchiveConfirm] = useState(false);
  const update = (key, next) => setValue((old) => ({ ...old, [key]: next }));
  const downloadSettings = () => {
    try {
      const url = URL.createObjectURL(
        new Blob([exportProject(project)], { type: "application/json" }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = "restivra-project.json";
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch {
      setError(t("transferInvalid"));
    }
  };
  const source = value.source || { kind: "local" };
  const setSource = (key, next) => update("source", { ...source, [key]: next });
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
          {project.imported && (
            <p className="notice">{t("importProjectReview")}</p>
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
            {t("backupSource")}
            <select
              aria-label={t("backupSource")}
              value={source.kind || "local"}
              onChange={(event) =>
                update(
                  "source",
                  event.target.value === "s3"
                    ? { kind: "s3", profile: "", bucket: "" }
                    : { kind: "local" },
                )
              }
            >
              <option value="local">{t("sourceLocal")}</option>
              <option value="s3">{t("sourceS3")}</option>
            </select>
          </label>
          {source.kind === "s3" && (
            <>
              <div className="field-grid">
                <label className="field">
                  {t("sourceProfile")}
                  <select
                    required
                    aria-label={t("sourceProfile")}
                    value={source.profile || ""}
                    onChange={(event) =>
                      setSource("profile", event.target.value)
                    }
                  >
                    <option value="">{t("chooseSourceProfile")}</option>
                    {source.profile &&
                      !sourceProfiles.some(
                        (profile) => profile.id === source.profile,
                      ) && (
                        <option value={source.profile}>
                          {source.profile} · {t("sourceUnavailable")}
                        </option>
                      )}
                    {sourceProfiles.map((profile) => (
                      <option key={profile.id} value={profile.id}>
                        {profile.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="field">
                  {t("sourceBucket")}
                  <input
                    required
                    maxLength={63}
                    value={source.bucket || ""}
                    placeholder="application-backups"
                    onChange={(event) =>
                      setSource("bucket", event.target.value)
                    }
                  />
                </label>
              </div>
              <p className="help-text">
                {t(
                  sourceProfiles.length
                    ? "sourceSecretHint"
                    : "sourceNoProfiles",
                )}
              </p>
            </>
          )}
          <label className="field">
            {t(source.kind === "s3" ? "sourceObjectPattern" : "backupPattern")}
            <input
              required
              className="code-input"
              placeholder="customer/*.dump"
              value={value.backup_pattern}
              onChange={(event) => update("backup_pattern", event.target.value)}
            />
            <small>
              {t(source.kind === "s3" ? "sourcePatternHint" : "patternHint")}
            </small>
          </label>
          <p className="format-note">{t("supportedFormat")}</p>
          <fieldset>
            <legend>{t("scheduling")}</legend>
            <p className="help-text">{t("schedulingHelp")}</p>
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
            <CheckTemplates
              checks={value.config.checks}
              onInsert={(check) =>
                config("checks", [...value.config.checks, check])
              }
            />
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
          {project.id && (
            <div className="project-transfer">
              <button
                type="button"
                className="text-button"
                disabled={saving}
                onClick={downloadSettings}
              >
                {t("exportProject")}
              </button>
              <p className="help-text">{t("exportProjectHelp")}</p>
            </div>
          )}
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
function Report({ job, duration, onClose, onCancel, api, onInspect }) {
  const { t, explain, formatDate, language } = useLanguage(),
    [copied, setCopied] = useState(false),
    [copyError, setCopyError] = useState(""),
    [redactEvidence, setRedactEvidence] = useState(false);
  const evidenceLink = (attempt) =>
    `/api/jobs/${job.id}/evidence.html?${new URLSearchParams({ lang: language, redact: redactEvidence ? "1" : "0", ...(attempt ? { attempt: String(attempt) } : {}) })}`;
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
          <RunState job={job} />
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
        {report?.error_code && (
          <p className="notice">
            {t(`advice.${report.error_code}`)} <code>{report.error_code}</code>
          </p>
        )}
        {job.retry_at && (
          <p className="notice">
            {t("retryExplanation", { time: formatDate(job.retry_at, true) })}
          </p>
        )}
        {job.attempts?.length > 0 &&
          (job.attempt_count > 1 || job.retry_at) && (
            <section aria-label={t("attemptHistory")}>
              <h3>{t("attemptHistory")}</h3>
              <div className="table-scroll">
                <table className="attempt-history">
                  <thead>
                    <tr>
                      <th>{t("attempt")}</th>
                      <th>{t("result")}</th>
                      <th>{t("details")}</th>
                      <th>{t("report")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {job.attempts.map((a) => (
                      <tr key={a.number}>
                        <td>{a.number}</td>
                        <td>
                          <Status value={a.status} />
                        </td>
                        <td>
                          {explain(a.error) || formatDate(a.finished_at, true)}
                        </td>
                        <td>
                          {a.report && (
                            <a
                              href={`/api/jobs/${job.id}/report?attempt=${a.number}`}
                              download
                            >
                              JSON
                            </a>
                          )}
                          {a.report && (
                            <>
                              {" "}
                              ·{" "}
                              <a
                                href={evidenceLink(a.number)}
                                target="_blank"
                                rel="noreferrer"
                              >
                                {t("printReport")}
                              </a>
                            </>
                          )}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>
          )}
        <dl className="report-facts">
          <div>
            <dt>{t("started")}</dt>
            <dd>{formatDate(job.started_at || job.queued_at, true)}</dd>
          </div>
          <div>
            <dt>
              {t(job.attempt_count > 1 ? "latestAttemptDuration" : "duration")}
            </dt>
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
                      <td>
                        {check.error_code
                          ? t(`advice.${check.error_code}`)
                          : check.error && explain(check.error)}
                      </td>
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
            {!active(job) && (
              <RunComparison jobID={job.id} api={api} onInspect={onInspect} />
            )}
            <h3 className="report-section-title">{t("archive")}</h3>
            <dl className="archive-facts">
              <div>
                <dt>{t("backupSource")}</dt>
                <dd>
                  {report.backup.source?.kind === "s3"
                    ? t("sourceS3")
                    : t("sourceLocal")}
                </dd>
              </div>
              {report.backup.source?.kind === "s3" && (
                <>
                  <div>
                    <dt>{t("sourceProfile")}</dt>
                    <dd>
                      <code>{report.backup.source.profile}</code>
                    </dd>
                  </div>
                  {report.backup.source.key && (
                    <div>
                      <dt>{t("sourceObject")}</dt>
                      <dd>
                        <code>
                          s3://{report.backup.source.bucket}/
                          {report.backup.source.key}
                        </code>
                      </dd>
                    </div>
                  )}
                  {report.backup.source.version_id && (
                    <div>
                      <dt>{t("sourceVersion")}</dt>
                      <dd>
                        <code>{report.backup.source.version_id}</code>
                      </dd>
                    </div>
                  )}
                  <div>
                    <dt>ETag</dt>
                    <dd>
                      <code>{report.backup.source.etag}</code>
                    </dd>
                  </div>
                </>
              )}
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
            <details className="advanced" open={notices.length > 2}>
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
      {report && (
        <div className="evidence-options">
          <label className="checkbox-label">
            <input
              type="checkbox"
              checked={redactEvidence}
              onChange={(event) => setRedactEvidence(event.target.checked)}
            />
            {t("redactEvidence")}
          </label>
          <p className="help-text">{t("evidenceHint")}</p>
        </div>
      )}
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
            className="button"
            href={evidenceLink()}
            target="_blank"
            rel="noreferrer"
          >
            {t("printReport")}
          </a>
        )}
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
function Setup({ sourceProfiles, api }) {
  const { t } = useLanguage(),
    [shell, setShell] = useState("powershell");
  const installation =
    shell === "powershell"
      ? "./scripts/install.ps1\n./scripts/pilot-check.ps1"
      : "sh scripts/setup.sh\ndocker pull postgres:17-bookworm\ndocker compose -f compose.yaml -f compose.build.yaml up --build -d";
  return (
    <div className="setup-layout">
      <section>
        <SupportDiagnostics api={api} />
        <h2>{t("installation")}</h2>
        <p>{t(shell === "powershell" ? "readyImageInstall" : "sourceBuildInstall")}</p>
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
        <section className="setup-step">
          <h3>{t("sourceSetupTitle")}</h3>
          <p>{t("sourceSetupHelp")}</p>
          <p>{t("sourceSetupRestart")}</p>
          {sourceProfiles.length > 0 && (
            <ul>
              {sourceProfiles.map((profile) => (
                <li key={profile.id}>
                  {profile.name} <code>{profile.id}</code>
                </li>
              ))}
            </ul>
          )}
          <a
            href="https://github.com/lazypanic999/restivra/blob/main/docs/S3-SOURCES.md"
            target="_blank"
            rel="noreferrer"
          >
            {t("sourceSetupGuide")}
          </a>
        </section>
      </section>
      <aside className="setup-notes">
        <h2>{t("documented")}</h2>
        <p>{t("scopeNote")}</p>
        <p>{t("supportedFormat")}</p>
        <h3>{t("workerDiagnostics")}</h3>
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
    <AuthGate />
  </LanguageProvider>,
);
