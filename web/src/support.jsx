import React, { useEffect, useRef, useState } from "react";
import { useLanguage } from "./language.jsx";

export function SupportDiagnostics({ api }) {
  const { t, formatDate } = useLanguage();
  const [snapshot, setSnapshot] = useState(null);
  const [error, setError] = useState(false);
  const [loading, setLoading] = useState(true);
  const refresh = useRef(null);
  useEffect(() => {
    let disposed = false;
    let pending = false;
    async function load() {
      if (pending) return;
      pending = true;
      if (!disposed) setLoading(true);
      try {
        const next = await api("/support");
        if (!disposed) { setSnapshot(next); setError(false); }
      } catch {
        if (!disposed) { setSnapshot(null); setError(true); }
      } finally {
        pending = false;
        if (!disposed) setLoading(false);
      }
    }
    refresh.current = load;
    load();
    const timer = setInterval(load, 15000);
    return () => { disposed = true; clearInterval(timer); refresh.current = null; };
  }, [api]);
  const rows = snapshot ? [
    ["supportVersion", snapshot.build.version],
    ["supportDatabase", t(`supportState.${snapshot.database}`)],
    ["supportWorker", t(`supportState.${snapshot.worker}`)],
    ["supportAuthentication", t(snapshot.authentication_enabled ? "supportEnabled" : "supportDisabled")],
    ...(snapshot.counts ? [
      ["supportQueued", snapshot.counts.queued],
      ["supportDeferred", snapshot.counts.deferred],
      ["supportRunning", snapshot.counts.running],
      ["supportOverdue", snapshot.counts.overdue_projects],
    ] : []),
  ] : [];
  return <section className="support-diagnostics" aria-labelledby="support-title">
    <div className="section-heading">
      <h2 id="support-title">{t("supportTitle")}</h2>
      <button className="button" disabled={loading} onClick={() => refresh.current?.()}>{t("refresh")}</button>
    </div>
    <p>{t("supportHelp")}</p>
    <div aria-live="polite">
      {error && <p role="alert">{t("supportUnavailable")}</p>}
      {loading && !snapshot && !error && <p>{t("loading")}</p>}
      {snapshot && <>
        <dl className="support-details">
          {rows.map(([label, value]) => <div key={label}><dt>{t(label)}</dt><dd>{value}</dd></div>)}
        </dl>
        {snapshot.notices.length > 0 && <ul className="support-notices">
          {snapshot.notices.map((notice) => <li key={notice}>{t(`supportNotice.${notice}`)}</li>)}
        </ul>}
        {snapshot.counts?.failed_notifications > 0 && <p><a href="?view=history&notification=failed">{t("reviewFailedNotifications")}</a></p>}
        <small>{t("supportChecked", { time: formatDate(snapshot.generated_at, true) })}</small>
      </>}
    </div>
    <p><a className="button" href="/api/support/download" download="restivra-support.json">{t("supportDownload")}</a></p>
    <p className="help-text">{t("supportPrivacy")}</p>
  </section>;
}
