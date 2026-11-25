import test from "node:test";
import assert from "node:assert/strict";
import { median, summarize, sparkPoints, mapLimited } from "./trends.js";

// Build API-shaped items, newest first, from oldest-first specs.
const items = (specs) =>
  specs
    .map(([status, bytes, ms, sha], i) => ({ id: `r${i}`, status, backup_bytes: bytes, duration_ms: ms, backup_sha256: sha ?? `h${i}` }))
    .reverse();

test("median handles odd, even and empty inputs", () => {
  assert.equal(median([3, 1, 2]), 2);
  assert.equal(median([4, 1, 3, 2]), 2.5);
  assert.equal(median([]), null);
});

test("unfinished runs are ignored and order becomes chronological", () => {
  const s = summarize([
    { id: "c", status: "running" },
    { id: "b", status: "failed", backup_bytes: 10, duration_ms: 5 },
    { id: "a", status: "passed", backup_bytes: 10, duration_ms: 5 },
  ]);
  assert.deepEqual(s.runs.map((r) => r.id), ["a", "b"]);
  assert.equal(s.passRate, 0.5);
});

test("a sharply smaller archive is flagged first", () => {
  const s = summarize(items([["passed", 100, 10], ["passed", 104, 10], ["passed", 98, 10], ["passed", 101, 10], ["passed", 30, 10]]));
  assert.equal(s.signals[0].kind, "archiveShrank");
  assert.equal(s.signals[0].percent, 70);
  assert.equal(s.signals[0].sample, 4);
});

test("no size baseline from fewer than three earlier archives", () => {
  const s = summarize(items([["passed", 100, 10], ["passed", 100, 10], ["passed", 10, 10]]));
  assert.equal(s.signals.length, 0);
});

test("consecutive failures, unchanged archives and slow restores", () => {
  const s = summarize(
    items([
      ["passed", 100, 10, "x"],
      ["passed", 100, 10, "y"],
      ["passed", 100, 10, "z"],
      ["failed", 100, 11, "z"],
      ["error", 100, 40, "z"],
    ]),
  );
  assert.deepEqual(s.signals.map((x) => x.kind), ["failingStreak", "archiveUnchanged", "restoreSlower"]);
  assert.equal(s.signals[0].count, 2);
  assert.equal(s.signals[1].count, 3);
  assert.equal(s.signals[2].times, 4);
});

test("a single failure or an empty history raises nothing", () => {
  assert.deepEqual(summarize(items([["passed", 10, 10], ["failed", 10, 10]])).signals, []);
  const empty = summarize([]);
  assert.equal(empty.passRate, null);
  assert.equal(empty.latestBytes, null);
});

test("sparkline points stay inside the box", () => {
  const pts = sparkPoints([5, 10, 0], 100, 20, 2);
  assert.deepEqual(pts, [[2, 10], [50, 2], [98, 18]]);
  assert.deepEqual(sparkPoints([7, 7], 100, 20), [[2, 10], [98, 10]]);
  assert.deepEqual(sparkPoints([], 100, 20), []);
});

test("mapLimited keeps order and the concurrency limit", async () => {
  let active = 0, peak = 0;
  const out = await mapLimited([1, 2, 3, 4, 5], 2, async (n) => {
    active++; peak = Math.max(peak, active);
    await new Promise((r) => setTimeout(r, 5 * (6 - n)));
    active--; return n * 2;
  });
  assert.deepEqual(out, [2, 4, 6, 8, 10]);
  assert.equal(peak, 2);
});
