export const projectTransferLimit = 2 * 1024 * 1024;
const format = "restivra.project.v1";
const projectKeys = [
  "name",
  "backup_pattern",
  "source",
  "config",
  "schedule_time",
  "timezone",
];
const configKeys = [
  "timeout_seconds",
  "max_backup_bytes",
  "memory_mb",
  "data_mb",
  "checks",
];

function object(value, keys) {
  if (
    !value ||
    typeof value !== "object" ||
    Array.isArray(value) ||
    Object.keys(value).some((key) => !keys.includes(key))
  )
    throw Error("transferInvalid");
}
function string(value, limit) {
  if (
    typeof value !== "string" ||
    new TextEncoder().encode(value).length > limit ||
    value.includes("\0")
  )
    throw Error("transferInvalid");
}
function settings(value) {
  object(value, projectKeys);
  string(value.name, 120);
  string(value.backup_pattern, 1024);
  string(value.schedule_time, 5);
  string(value.timezone, 100);
  object(value.source, ["kind", "profile", "bucket"]);
  if (!["local", "s3"].includes(value.source.kind))
    throw Error("transferInvalid");
  for (const key of ["profile", "bucket"])
    if (value.source[key] !== undefined) string(value.source[key], 100);
  object(value.config, configKeys);
  for (const key of configKeys.filter((key) => key !== "checks"))
    if (!Number.isSafeInteger(value.config[key]))
      throw Error("transferInvalid");
  if (!Array.isArray(value.config.checks) || value.config.checks.length > 32)
    throw Error("transferInvalid");
  for (const check of value.config.checks) {
    object(check, ["name", "sql"]);
    string(check.name, 120);
    string(check.sql, 8192);
  }
  return structuredClone(value);
}

// Explicit projection: histories, IDs, reports, endpoints and keys do not travel.
// SQL is included on purpose and the UI identifies the export as private.
export function exportProject(project) {
  const value = {
    name: project.name,
    backup_pattern: project.backup_pattern,
    source: project.source || { kind: "local" },
    config: Object.fromEntries(
      configKeys.map((key) => [key, project.config[key]]),
    ),
    schedule_time: project.schedule_time,
    timezone: project.timezone,
  };
  const output =
    JSON.stringify({ format, project: settings(value) }, null, 2) + "\n";
  if (new TextEncoder().encode(output).length > projectTransferLimit)
    throw Error("transferInvalid");
  return output;
}

export function importProject(text) {
  if (
    typeof text !== "string" ||
    new TextEncoder().encode(text).length > projectTransferLimit
  )
    throw Error("transferInvalid");
  let parsed;
  try {
    parsed = JSON.parse(text);
  } catch {
    throw Error("transferInvalid");
  }
  object(parsed, ["format", "project"]);
  if (parsed.format !== format) throw Error("transferInvalid");
  const result = settings(parsed.project);
  // Never restore IDs or silently reactivate someone else's daily schedule.
  result.schedule_time = "";
  return result;
}
