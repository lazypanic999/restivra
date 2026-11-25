import React, { useEffect, useState } from "react";
import { useLanguage } from "./language.jsx";
import { TREND_WINDOW, summarize, sparkPoints, mapLimited } from "./trends.js";

const W = 132, H = 30;

function Spark({ values, label }) {
  if (values.length < 2) return <span className="secondary">—</span>;
  const pts = sparkPoints(values, W, H, 3);
  const [lx, ly] = pts[pts.length - 1];
  return (
    <svg className="spark" width={W} height={H} viewBox={`0 0 ${W} ${H}`} role="img" aria-label={label}>
      <polyline points={pts.map((p) => p.join(",")).join(" ")} />
      <circle cx={lx} cy={ly} r="2.6" />
    </svg>
  );
}

function RunStrip({ runs, onInspect, t, formatDate }) {
  if (!runs.length) return <span className="secondary">{t("trendNoRuns")}</span>;
  return (
    <div className="run-strip">
      {runs.map((run) => (
        <button
          key={run.id}
          className={`tick tick-${run.status}`}
          title={`${t(`status.${run.status}`)} · ${formatDate(run.queued_at)}`}
          aria-label={`${t(`status.${run.status}`)}, ${formatDate(run.queued_at)}`}
          onClick={() => onInspect(run)}
        />
      ))}
    </div>
  );
}

export const TREND_PROJECTS = 30;

export function Trends({ api, onInspect, size, Empty }) {
  const { t, explain, formatDate, language } = useLanguage();
  const [rows, setRows] = useState(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const [total, setTotal] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    let disposed = false;
    setError("");
    api(`/projects/page?limit=${TREND_PROJECTS}`, { signal: controller.signal })
      .then((page) => {
        if (!disposed) setTotal(page.active_total || 0);
        return mapLimited(page.items || [], 4, async (project) => {
          const query = new URLSearchParams({ project_id: project.id, limit: String(TREND_WINDOW) });
          const data = await api(`/history?${query}`, { signal: controller.signal });
          return { project, summary: summarize(data.items || []) };
        });
      })
      .then((result) => !disposed && setRows(result))
      .catch((e) => !disposed && e.name !== "AbortError" && setError(e.message));
    return () => {
      disposed = true;
      controller.abort();
    };
    // Trends load on entry and on demand, not on every dashboard poll.
  }, [api, reload]);

  const number = (n) => n.toLocaleString(language === "tr" ? "tr-TR" : "en-GB");
  const signalText = (s) => t(`trendSignal.${s.kind}`, { ...s, times: s.times && number(s.times) });
  const flagged = rows ? rows.filter((r) => r.summary.signals.some((s) => s.severity >= 2)).length : 0;

  if (rows && !rows.length) return <Empty title={t("noProjects")} text={t("noProjectsText")} />;
  return (
    <section className="ledger trends" aria-labelledby="trend-heading">
      <div className="section-heading">
        <h2 id="trend-heading">{t("trendHeading", { count: TREND_WINDOW })}</h2>
        <button className="text-button" onClick={() => setReload((n) => n + 1)}>{t("trendReload")}</button>
      </div>
      {error && <p className="notice error-notice" role="alert">{explain(error)}</p>}
      {!rows && !error && <div className="loading" role="status">{t("loading")}</div>}
      {rows && (
        <>
          <p className="trend-summary">
            {flagged ? t("trendFlagged", { count: flagged, total: rows.length }) : t("trendQuiet", { total: rows.length })}
          </p>
          <div className="table-scroll">
            <table>
              <thead>
                <tr>
                  <th>{t("project")}</th>
                  <th>{t("trendRuns")}</th>
                  <th className="numeric">{t("trendPassRate")}</th>
                  <th>{t("trendArchive")}</th>
                  <th>{t("trendRestoreTime")}</th>
                  <th>{t("trendSignals")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(({ project, summary: s }) => (
                  <tr key={project.id} className={s.signals.some((x) => x.severity >= 3) ? "trend-alert" : undefined}>
                    <td><strong>{project.name}</strong><code className="file-path">{project.source?.kind === "s3" ? `s3://${project.source.bucket}/${project.backup_pattern}` : project.backup_pattern}</code></td>
                    <td><RunStrip runs={s.runs} onInspect={onInspect} t={t} formatDate={formatDate} /></td>
                    <td className="numeric">{s.passRate === null ? "—" : `${Math.round(s.passRate * 100)}%`}</td>
                    <td>
                      <div className="trend-cell">
                        <Spark values={s.sizes} label={t("trendArchive")} />
                        <span>{s.latestBytes ? size(s.latestBytes) : "—"}</span>
                      </div>
                    </td>
                    <td>
                      <div className="trend-cell">
                        <Spark values={s.durations} label={t("trendRestoreTime")} />
                        <span>{s.medianDurationMS ? t("trendMedian", { seconds: number(Math.round(s.medianDurationMS / 100) / 10) }) : "—"}</span>
                      </div>
                    </td>
                    <td className="trend-signals">
                      {s.signals.length ? (
                        <ul>{s.signals.map((x) => <li key={x.kind} className={`sev-${x.severity}`}>{signalText(x)}</li>)}</ul>
                      ) : (
                        <span className="secondary">{s.runs.length >= 4 ? t("trendNothing") : t("trendTooFew")}</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          {total > rows.length && <p className="help-text">{t("trendWindow", { count: rows.length, total })}</p>}
          <p className="help-text">{t("trendHelp")}</p>
        </>
      )}
    </section>
  );
}
