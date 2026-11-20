import React, { useEffect, useState } from "react";
import { useLanguage } from "./language.jsx";

export function RunComparison({ jobID, api, onInspect }) {
  const { t, formatDate } = useLanguage();
  const [open, setOpen] = useState(false);
  const [result, setResult] = useState(null);
  const [failed, setFailed] = useState(false);
  const [revision, setRevision] = useState(0);
  useEffect(() => {
    if (!open) return;
    let disposed = false;
    setFailed(false);
    api(`/jobs/${jobID}/comparison`)
      .then((data) => {
        if (!disposed) setResult(data);
      })
      .catch(() => {
        if (!disposed) setFailed(true);
      });
    return () => {
      disposed = true;
    };
  }, [open, jobID, api, revision]);
  const row = (item, name) => (
    <tr key={name}>
      <th scope="row">{name}</th>
      <td>{t(`status.${item.before || "not_run"}`)}</td>
      <td>{t(`status.${item.after || "not_run"}`)}</td>
      <td className={`comparison-change comparison-${item.change}`}>
        {t(`comparison.${item.change}`)}
      </td>
    </tr>
  );
  return (
    <details
      className="run-comparison"
      onToggle={(e) => setOpen(e.currentTarget.open)}
    >
      <summary>{t("comparisonTitle")}</summary>
      <p className="help-text">{t("comparisonHelp")}</p>
      {failed ? (
        <p role="alert">
          {t("comparisonFailed")}{" "}
          <button
            type="button"
            className="text-button"
            onClick={() => setRevision((value) => value + 1)}
          >
              {t("comparisonRetry")}
          </button>
        </p>
      ) : !result ? (
        <p role="status">{t("loading")}</p>
      ) : !result.baseline ? (
        <p>{t("comparisonNoBaseline")}</p>
      ) : (
        <>
          <p className="comparison-baseline">
            {t("comparisonBaseline")}{" "}
            <button
              type="button"
              className="text-button"
              onClick={() => onInspect({ id: result.baseline.id })}
            >
              {formatDate(result.baseline.queued_at, true)} ·{" "}
              {result.baseline.id.slice(0, 8)}
            </button>
          </p>
          <p>{t(`comparisonContent.${result.content}`)}</p>
          {(result.source_changed ||
            result.limits_changed ||
            result.image_changed) && (
            <ul className="comparison-context">
              {result.source_changed && <li>{t("comparisonSourceChanged")}</li>}
              {result.limits_changed && <li>{t("comparisonLimitsChanged")}</li>}
              {result.image_changed && <li>{t("comparisonImageChanged")}</li>}
            </ul>
          )}
          <div className="table-scroll">
            <table className="comparison-results">
              <thead>
                <tr>
                  <th>{t("stage")}</th>
                  <th>{t("comparisonBefore")}</th>
                  <th>{t("comparisonAfter")}</th>
                  <th>{t("comparisonChange")}</th>
                </tr>
              </thead>
              <tbody>
                {row(result.restore, t("restore"))}
                {result.checks.map((item) => row(item, item.name))}
                {row(result.cleanup, t("cleanup"))}
              </tbody>
            </table>
          </div>
        </>
      )}
    </details>
  );
}
