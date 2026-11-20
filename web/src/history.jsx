import React, { useEffect, useState } from "react";
import { useLanguage } from "./language.jsx";
import {
  emptyHistoryFilters,
  historyFiltersFromSearch,
  historyParams,
  historySearch,
} from "./history.js";

const outcomes = [
  "passed",
  "failed",
  "interrupted",
  "running",
  "queued",
  "cancelled",
];

export function History({
  api,
  onInspect,
  onUnauthorized,
  revision,
  RunState,
  Button,
}) {
  const { t, explain, formatDate } = useLanguage();
  const [filters, setFilters] = useState(() =>
    historyFiltersFromSearch(location.search),
  );
  const [draft, setDraft] = useState(filters);
  const [cursors, setCursors] = useState([""]);
  const [result, setResult] = useState(null);
  const [error, setError] = useState("");
  const [formError, setFormError] = useState("");
  const [exportError, setExportError] = useState("");
  const [exporting, setExporting] = useState(false);
  const [refresh, setRefresh] = useState(0);
  const [busy, setBusy] = useState(false);
  const cursor = cursors[cursors.length - 1];
  let query = "",
    invalid = "";
  try {
    query = historyParams(filters).toString();
  } catch (e) {
    invalid = e.message;
  }
  const key = `${query}|${cursor}`;
  const liveRevision = cursor ? "" : revision;
  useEffect(() => {
    if (invalid) return;
    const controller = new AbortController();
    let disposed = false;
    setBusy(true);
    const params = new URLSearchParams(query);
    if (cursor) params.set("cursor", cursor);
    api(`/history?${params}`, { signal: controller.signal })
      .then((data) => {
        if (!disposed) {
          setResult({ key, data });
          setError("");
        }
      })
      .catch((e) => {
        if (!disposed) setError(e.message);
      })
      .finally(() => {
        if (!disposed) setBusy(false);
      });
    return () => {
      disposed = true;
      controller.abort();
    };
  }, [api, key, query, cursor, invalid, liveRevision, refresh]);

  const data = result?.key === key && !invalid ? result.data : null;
  const setField = (field, value) =>
    setDraft((current) => ({ ...current, [field]: value }));
  function apply(event) {
    event.preventDefault();
    try {
      historyParams(draft);
    } catch (e) {
      setFormError(e.message);
      return;
    }
    const clean = { ...draft, q: draft.q.trim() };
    setFilters(clean);
    setCursors([""]);
    setRefresh((value) => value + 1);
    setError("");
    setFormError("");
    setExportError("");
    window.history.replaceState(
      null,
      "",
      `${location.pathname}?${historySearch(clean, location.search)}${location.hash}`,
    );
  }
  function reset() {
    setFilters({ ...emptyHistoryFilters });
    setDraft({ ...emptyHistoryFilters });
    setCursors([""]);
    setError("");
    setFormError("");
    setExportError("");
    setRefresh((value) => value + 1);
    window.history.replaceState(
      null,
      "",
      `${location.pathname}?${historySearch(emptyHistoryFilters, location.search)}${location.hash}`,
    );
  }
  async function download() {
    setExporting(true);
    setExportError("");
    try {
      const response = await fetch(`/api/history/export.csv?${query}`, {
        cache: "no-store",
      });
      if (response.status === 401) onUnauthorized();
      if (!response.ok)
        throw Error(
          (await response.json()).error ||
            "The request could not be completed.",
        );
      const url = URL.createObjectURL(await response.blob());
      const link = document.createElement("a");
      link.href = url;
      link.download = "restivra-history.csv";
      document.body.appendChild(link);
      link.click();
      link.remove();
      // Allow the browser to consume the blob before revoking it.
      setTimeout(() => URL.revokeObjectURL(url), 1000);
    } catch (e) {
      setExportError(e.message);
    } finally {
      setExporting(false);
    }
  }
  return (
    <section className="history-ledger" aria-labelledby="history-heading">
      <h2 className="visually-hidden" id="history-heading">
        {t("history")}
      </h2>
      <form className="history-filters" onSubmit={apply}>
        <label className="search-field history-search">
          <span>{t("searchRuns")}</span>
          <input
            type="search"
            maxLength={120}
            value={draft.q}
            onChange={(e) => setField("q", e.target.value)}
          />
        </label>
        <label className="search-field">
          <span>{t("statusFilter")}</span>
          <select
            aria-label={t("statusFilter")}
            value={draft.status}
            onChange={(e) => setField("status", e.target.value)}
          >
            <option value="">{t("allResults")}</option>
            {outcomes.map((value) => (
              <option key={value} value={value}>
                {t(`status.${value}`)}
              </option>
            ))}
          </select>
        </label>
        <label className="search-field">
          <span>{t("historyFrom")}</span>
          <input
            type="date"
            min="1900-01-01"
            max="9998-12-31"
            value={draft.from}
            onChange={(e) => setField("from", e.target.value)}
            onInput={(e) => setField("from", e.target.value)}
          />
        </label>
        <label className="search-field">
          <span>{t("historyTo")}</span>
          <input
            type="date"
            min={draft.from || "1900-01-01"}
            max="9998-12-31"
            value={draft.to}
            onChange={(e) => setField("to", e.target.value)}
            onInput={(e) => setField("to", e.target.value)}
          />
        </label>
        <Button type="submit">{t("historyApply")}</Button>
        <button type="button" className="text-button" onClick={reset}>
          {t("historyReset")}
        </button>
      </form>
      <p className="help-text">{t("historyDateHelp")}</p>
      {filters.project_id && (
        <p className="help-text">{t("historyProjectScope")}</p>
      )}
      {(formError || error || invalid) && (
        <div className="notice error-notice" role="alert">
          {explain(t(formError || error || invalid))}{" "}
          <button onClick={() => setRefresh((value) => value + 1)}>
            {t("refresh")}
          </button>
        </div>
      )}
      <div className="history-toolbar">
        <p role="status" aria-live="polite">
          {data
            ? t("historyMatches", { count: data.summary.total })
            : t("loading")}
        </p>
        <Button
          onClick={download}
          disabled={
            !data?.summary.total ||
            data.summary.total > 10000 ||
            exporting ||
            !!invalid
          }
        >
          {t(exporting ? "historyExporting" : "historyExport")}
        </Button>
      </div>
      {data?.summary.total > 10000 && (
        <p className="help-text">{t("historyExportLimit")}</p>
      )}
      {exportError && (
        <p className="notice error-notice" role="alert">
          {explain(exportError)}
        </p>
      )}
      <div aria-busy={busy}>
        {data ? (
          <>
            <p className="history-counts">
              {t("historyOutcomes", {
                passed: data.summary.passed,
                failed: data.summary.failed + data.summary.interrupted,
                cancelled: data.summary.cancelled,
                active: data.summary.queued + data.summary.running,
              })}
            </p>
            {data.items.length ? (
              <div className="table-scroll">
                <table>
                  <thead>
                    <tr>
                      <th>{t("project")}</th>
                      <th>{t("result")}</th>
                      <th>{t("historyQueued")}</th>
                      <th className="numeric">{t("latestAttemptDuration")}</th>
                      <th>{t("trigger")}</th>
                      <th className="action-column">{t("report")}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.items.map((job) => (
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
                        <td>{formatDate(job.queued_at, true)}</td>
                        <td className="numeric">
                          {job.duration_ms == null
                            ? "—"
                            : `${(job.duration_ms / 1000).toFixed(1)} ${t("seconds")}`}
                        </td>
                        <td>
                          {t(
                            job.trigger === "schedule" ? "scheduled" : "manual",
                          )}
                        </td>
                        <td className="action-column">
                          <button
                            className="text-button"
                            onClick={() => onInspect(job)}
                          >
                            {t("inspect")}
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            ) : (
              <p className="no-results">{t("noMatches")}</p>
            )}
            <nav
              className="history-pagination"
              aria-label={t("historyPagination")}
            >
              <Button
                disabled={cursors.length === 1 || busy}
                onClick={() => setCursors((value) => value.slice(0, -1))}
              >
                {t("historyNewer")}
              </Button>
              <span>{t("historyPage", { number: cursors.length })}</span>
              <Button
                disabled={!data.next_cursor || busy}
                onClick={() =>
                  setCursors((value) => [...value, data.next_cursor])
                }
              >
                {t("historyOlder")}
              </Button>
              {cursors.length > 1 && (
                <button
                  className="text-button"
                  onClick={() => setCursors([""])}
                >
                  {t("historyLatest")}
                </button>
              )}
            </nav>
          </>
        ) : (
          !error && !invalid && <p role="status">{t("loading")}</p>
        )}
      </div>
    </section>
  );
}
