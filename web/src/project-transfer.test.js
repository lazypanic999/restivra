import test from "node:test";
import assert from "node:assert/strict";
import {
  exportProject,
  importProject,
  projectTransferLimit,
} from "./project-transfer.js";

const fixture = () => ({
  id: "old-instance-id",
  name: "Customer archive",
  backup_pattern: "daily/*.dump",
  source: { kind: "s3", profile: "offsite", bucket: "customer-backups" },
  schedule_time: "03:00",
  timezone: "Europe/Istanbul",
  config: {
    timeout_seconds: 180,
    max_backup_bytes: 268435456,
    memory_mb: 1536,
    data_mb: 512,
    checks: [{ name: "Rows", sql: "SELECT count(*)>0 FROM public.customers" }],
  },
  last_job: { report: { private: "not-exported" } },
});

test("portable settings preserve editable SQL and source selection without IDs or history", () => {
  const source = fixture();
  const encoded = exportProject(source);
  assert.ok(!encoded.includes("old-instance-id"));
  assert.ok(!encoded.includes("not-exported"));
  const imported = importProject(encoded);
  assert.equal(imported.schedule_time, "");
  assert.equal(source.schedule_time, "03:00");
  assert.equal(imported.config.checks[0].sql, source.config.checks[0].sql);
  assert.deepEqual(imported.source, source.source);
  assert.equal(imported.id, undefined);
  imported.config.checks[0].sql = "SELECT false";
  assert.notEqual(imported.config.checks[0].sql, source.config.checks[0].sql);
});
test("transfer rejects credentials, unknown versions and malformed shapes before the editor", () => {
  const valid = JSON.parse(exportProject(fixture()));
  for (const mutate of [
    (v) => {
      v.format = "restivra.project.v2";
    },
    (v) => {
      v.project.source.secret_access_key = "private-sentinel";
    },
    (v) => {
      v.project.source.endpoint = "https://private.invalid";
    },
    (v) => {
      v.project.id = "old";
    },
    (v) => {
      v.project.config.checks = null;
    },
    (v) => {
      v.project.config.checks = [null];
    },
    (v) => {
      v.project.config.timeout_seconds = "180";
    },
    (v) => {
      v.project.config.checks = Array(33).fill({
        name: "x",
        sql: "SELECT true",
      });
    },
    (v) => {
      v.project.source.kind = "unsupported";
    },
    (v) => {
      v.project.name = "ş".repeat(61);
    },
    (v) => {
      v.project.config.checks[0].sql = "x".repeat(8193);
    },
    (v) => {
      v.project.source = null;
    },
    (v) => {
      v.project.config.password = "private-sentinel";
    },
  ]) {
    const candidate = structuredClone(valid);
    mutate(candidate);
    assert.throws(
      () => importProject(JSON.stringify(candidate)),
      /^Error: transferInvalid$/,
    );
  }
  for (const raw of [
    "null",
    "[]",
    "{}",
    "not JSON",
    JSON.stringify({ __proto__: null }),
    " ".repeat(projectTransferLimit + 1),
  ])
    assert.throws(() => importProject(raw), /transferInvalid/);
  assert.throws(
    () => importProject('{"format":"restivra.project.v1","__proto__":{}}'),
    /transferInvalid/,
  );
  const withSecret = fixture();
  withSecret.source.access_key_id = "secret";
  assert.throws(() => exportProject(withSecret), /transferInvalid/);
});
test("local-only settings round-trip without introducing remote profiles", () => {
  const source = fixture();
  delete source.source;
  source.config.checks = [];
  assert.deepEqual(importProject(exportProject(source)).source, {
    kind: "local",
  });
});
