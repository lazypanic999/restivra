import test from "node:test";
import assert from "node:assert/strict";
import {
  emptyHistoryFilters,
  historyParams,
  historyFiltersFromSearch,
  historySearch,
  initialPage,
} from "./history.js";

test("project and setup views survive reload without accepting arbitrary views", () => {
  assert.equal(initialPage("?view=projects"), "projects");
  assert.equal(initialPage("?view=setup"), "setup");
  assert.equal(initialPage("?view=unknown"), "overview");
});

test("history links preserve literal queries, result and selected project", () => {
  const filters = {
    ...emptyHistoryFilters,
    q: "Ödeme %_ & archive",
    status: "failed",
    project_id: "a".repeat(32),
  };
  const search = historySearch(filters, "unrelated=keep");
  assert.equal(initialPage(search), "history");
  assert.deepEqual(historyFiltersFromSearch(search), filters);
  assert.equal(new URLSearchParams(search).get("unrelated"), "keep");
  const params = historyParams(filters);
  assert.equal(params.get("q"), filters.q);
  assert.equal(params.get("project_id"), filters.project_id);
  assert.equal(params.get("status"), "failed");
  assert.equal(params.has("cursor"), false);
  const cleared = historyFiltersFromSearch(
    historySearch(emptyHistoryFilters, search),
  );
  assert.deepEqual(cleared, emptyHistoryFilters);
});

test("history date range includes the whole local end day, including DST", () => {
  const previous = process.env.TZ;
  try {
    process.env.TZ = "America/New_York";
    for (const [day, hours] of [
      ["2026-03-08", 23],
      ["2026-11-01", 25],
    ]) {
      const params = historyParams({
        ...emptyHistoryFilters,
        from: day,
        to: day,
      });
      assert.equal(
        (new Date(params.get("until")) - new Date(params.get("since"))) /
          3600000,
        hours,
      );
    }
    process.env.TZ = "Europe/Istanbul";
    const params = historyParams({
      ...emptyHistoryFilters,
      from: "2026-11-20",
      to: "2026-11-20",
    });
    assert.equal(params.get("since"), "2026-11-19T21:00:00.000Z");
    assert.equal(params.get("until"), "2026-11-20T21:00:00.000Z");
  } finally {
    if (previous === undefined) delete process.env.TZ;
    else process.env.TZ = previous;
  }
});

test("invalid calendar dates and reversed ranges are rejected", () => {
  for (const filters of [
    { from: "2026-02-30" },
    { to: "invalid" },
    { from: "2026-12-02", to: "2026-12-01" },
    { from: "1899-01-01" },
  ]) {
    assert.throws(
      () => historyParams({ ...emptyHistoryFilters, ...filters }),
      /historyInvalidDates/,
    );
  }
});
