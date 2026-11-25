import test from "node:test";
import assert from "node:assert/strict";
import { attentionWarnings } from "./warnings.js";

test("standing disclaimers are ignored and actionable warnings sorted", () => {
  const report = {
    warnings: [
      "This verifies a logical restore and selected checks, not full application recovery, ownership, grants or point-in-time recovery.",
      "File modification time is not a verified backup creation time. Compare SHA-256 values across runs to detect unchanged backups.",
      "No custom checks configured; application data was not validated.",
      "The tested backup is identical to the previous run; backup freshness was not established.",
      "The backup is 70% smaller than the median of the previous 4 runs; it may be incomplete.",
    ],
  };
  assert.deepEqual(attentionWarnings(report), [
    { key: "warnShrank", severity: 3, values: { percent: "70" } },
    { key: "warnIdentical", severity: 2 },
    { key: "warnNoChecks", severity: 1 },
  ]);
});

test("missing reports and odd values produce nothing", () => {
  assert.deepEqual(attentionWarnings(undefined), []);
  assert.deepEqual(attentionWarnings({}), []);
  assert.deepEqual(attentionWarnings({ warnings: [null, 3, "unrelated"] }), []);
});
