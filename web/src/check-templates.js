function identifier(value) {
  const text = String(value || "").trim();
  if (!/^[A-Za-z_][A-Za-z0-9_]{0,62}$/.test(text))
    throw Error("templateInvalidIdentifier");
  return text;
}
function relation(value) {
  const parts = String(value || "")
    .trim()
    .split(".");
  if (parts.length === 1) parts.unshift("public");
  if (parts.length !== 2) throw Error("templateInvalidIdentifier");
  const [schema, table] = parts.map(identifier);
  return { schema, table, sql: `"${schema}"."${table}"` };
}
function integer(value, minimum, maximum) {
  if (!/^\d+$/.test(String(value))) throw Error("templateInvalidNumber");
  const number = Number(value);
  if (!Number.isSafeInteger(number) || number < minimum || number > maximum)
    throw Error("templateInvalidNumber");
  return number;
}

// No schema inspection or execution: generated checks remain editable.
export function availableCheckName(base, checks) {
  const names = new Set(checks.map((check) => check.name));
  if (!names.has(base)) return base;
  for (let suffix = 2; ; suffix++) {
    const candidate = `${base} (${suffix})`;
    if (!names.has(candidate)) return candidate;
  }
}

export function checkTemplate(
  kind,
  { table, column, minimum, hours, columns },
) {
  const target = relation(table);
  switch (kind) {
    case "rows":
      return {
        nameKey: "templateRows",
        sql: `SELECT count(*) > 0 FROM ${target.sql}`,
      };
    case "minimum":
      return {
        nameKey: "templateMinimum",
        sql: `SELECT count(*) >= ${integer(minimum, 0, 1000000000)} FROM ${target.sql}`,
      };
    case "recent": {
      const timestamp = identifier(column),
        age = integer(hours, 1, 8760);
      return {
        nameKey: "templateRecent",
        sql: `SELECT coalesce(max("${timestamp}") BETWEEN CURRENT_TIMESTAMP - INTERVAL '${age} hours' AND CURRENT_TIMESTAMP + INTERVAL '5 minutes', false) FROM ${target.sql}`,
      };
    }
    case "columns": {
      const names = String(columns || "")
        .split(",")
        .map(identifier);
      if (names.length > 32 || new Set(names).size !== names.length)
        throw Error("templateInvalidColumns");
      return {
        nameKey: "templateColumns",
        sql: `SELECT count(*) = ${names.length} FROM information_schema.columns WHERE table_schema = '${target.schema}' AND table_name = '${target.table}' AND column_name IN (${names.map((name) => `'${name}'`).join(", ")})`,
      };
    }
    default:
      throw Error("templateInvalidKind");
  }
}
