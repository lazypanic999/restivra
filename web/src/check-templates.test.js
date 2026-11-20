import test from "node:test";
import assert from "node:assert/strict";
import { checkTemplate, availableCheckName } from "./check-templates.js";

test("repeated insertions keep names unique without renaming existing checks", () => {
  const checks = [
    { name: "Table contains rows" },
    { name: "Table contains rows (2)" },
    { name: "Table contains rows (4)" },
  ];
  assert.equal(
    availableCheckName("Table contains rows", checks),
    "Table contains rows (3)",
  );
  assert.equal(
    availableCheckName("Required columns exist", checks),
    "Required columns exist",
  );
});

test("templates quote the schema and reject future timestamps in recent-data checks", () => {
  assert.equal(
    checkTemplate("rows", { table: "orders" }).sql,
    'SELECT EXISTS (SELECT 1 FROM "public"."orders")',
  );
  const minimum = checkTemplate("minimum", {
    table: "Billing.Orders",
    minimum: "250",
  });
  assert.match(minimum.sql, /count\(\*\) >= 250/);
  assert.match(minimum.sql, /"Billing"\."Orders"/);
  const recent = checkTemplate("recent", {
    table: "orders",
    column: "created_at",
    hours: "24",
  });
  assert.match(recent.sql, /coalesce/);
  assert.match(recent.sql, /BETWEEN CURRENT_TIMESTAMP/);
  assert.match(recent.sql, /5 minutes/);
});
test("untrusted identifiers and numeric fragments cannot become template SQL", () => {
  for (const table of [
    "orders; DROP TABLE x",
    "public.orders --",
    "a.b.c",
    'public."orders"',
    "x' OR true",
    "",
    "a".repeat(64),
  ])
    assert.throws(
      () => checkTemplate("rows", { table }),
      /templateInvalidIdentifier/,
    );
  for (const hours of ["0", "8761", "1.5", "1'", "NaN", "Infinity", "-1"])
    assert.throws(
      () =>
        checkTemplate("recent", {
          table: "orders",
          column: "created_at",
          hours,
        }),
      /templateInvalidNumber/,
    );
  assert.throws(
    () =>
      checkTemplate("recent", {
        table: "orders",
        column: "x); SELECT",
        hours: "1",
      }),
    /templateInvalidIdentifier/,
  );
});
test("required-column checks test inclusion and reject duplicate or injected names", () => {
  const query = checkTemplate("columns", {
    table: "public.orders",
    columns: "id, created_at",
  }).sql;
  assert.match(query, /count\(\*\) = 2/);
  assert.match(query, /table_schema = 'public'/);
  assert.match(query, /column_name IN \('id', 'created_at'\)/);
  assert.throws(
    () => checkTemplate("columns", { table: "orders", columns: "id,id" }),
    /templateInvalidColumns/,
  );
  assert.throws(
    () =>
      checkTemplate("columns", {
        table: "orders",
        columns: "id,created_at') OR true",
      }),
    /templateInvalidIdentifier/,
  );
});
