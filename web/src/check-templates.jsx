import React, { useState } from "react";
import { useLanguage } from "./language.jsx";
import { checkTemplate, availableCheckName } from "./check-templates.js";

export function CheckTemplates({ onInsert, checks }) {
  const full = checks.length >= 32;
  const { t } = useLanguage();
  const [kind, setKind] = useState("rows");
  const [fields, setFields] = useState({
    table: "",
    column: "",
    minimum: "1",
    hours: "24",
    columns: "",
  });
  const [attempted, setAttempted] = useState(false);
  const update = (key, value) =>
    setFields((current) => ({ ...current, [key]: value }));
  let generated = null,
    error = "";
  try {
    generated = checkTemplate(kind, fields);
  } catch (e) {
    error = e.message;
  }
  function insert() {
    setAttempted(true);
    if (generated && !full) {
      onInsert({
        name: availableCheckName(t(generated.nameKey), checks),
        sql: generated.sql,
      });
      setAttempted(false);
    }
  }
  return (
    <details className="check-templates">
      <summary>{t("templateStart")}</summary>
      <p className="help-text">{t("templateHelp")}</p>
      <div className="field-grid">
        <label className="field">
          {t("templateType")}
          <select
            aria-label={t("templateType")}
            value={kind}
            onChange={(e) => setKind(e.target.value)}
          >
            <option value="rows">{t("templateRows")}</option>
            <option value="minimum">{t("templateMinimum")}</option>
            <option value="recent">{t("templateRecent")}</option>
            <option value="columns">{t("templateColumns")}</option>
          </select>
        </label>
        <label className="field">
          {t("templateTable")}
          <input
            value={fields.table}
            placeholder="public.orders"
            onChange={(e) => update("table", e.target.value)}
          />
        </label>
        {kind === "minimum" && (
          <label className="field">
            {t("templateMinimumRows")}
            <input
              type="text"
              inputMode="numeric"
              value={fields.minimum}
              onChange={(e) => update("minimum", e.target.value)}
            />
          </label>
        )}
        {kind === "recent" && (
          <>
            <label className="field">
              {t("templateTimestamp")}
              <input
                value={fields.column}
                placeholder="created_at"
                onChange={(e) => update("column", e.target.value)}
              />
            </label>
            <label className="field">
              {t("templateHours")}
              <input
                type="text"
                inputMode="numeric"
                value={fields.hours}
                onChange={(e) => update("hours", e.target.value)}
              />
            </label>
          </>
        )}
        {kind === "columns" && (
          <label className="field">
            {t("templateColumnNames")}
            <input
              value={fields.columns}
              placeholder="id, created_at"
              onChange={(e) => update("columns", e.target.value)}
            />
          </label>
        )}
      </div>
      {kind === "recent" && (
        <p className="help-text">{t("templateRecentHelp")}</p>
      )}
      {generated && (
        <pre className="template-preview">
          <code>{generated.sql}</code>
        </pre>
      )}
      {attempted && error && (
        <p className="notice error-notice" role="alert">
          {t(error)}
        </p>
      )}
      <button type="button" className="button" onClick={insert} disabled={full}>
        {t("templateInsert")}
      </button>
    </details>
  );
}
