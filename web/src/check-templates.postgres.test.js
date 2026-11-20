import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { randomBytes } from "node:crypto";
import { setTimeout } from "node:timers/promises";
import { checkTemplate } from "./check-templates.js";

test(
  "generated checks evaluate restored-data edge cases on PostgreSQL 17",
  {
    skip: process.env.RESTIVRA_INTEGRATION !== "1",
    timeout: 90000,
  },
  async () => {
    const name = `restivra-template-${randomBytes(8).toString("hex")}`;
    const docker = (...args) =>
      spawnSync("docker", args, { encoding: "utf8", timeout: 20000 });
    const sql = (query) => {
      const result = docker(
        "exec",
        name,
        "psql",
        "-X",
        "-At",
        "-U",
        "postgres",
        "-v",
        "ON_ERROR_STOP=1",
        "-c",
        query,
      );
      assert.equal(result.status, 0, result.stderr || result.error?.message);
      return result.stdout.trim();
    };
    try {
      const start = docker(
        "run",
        "--detach",
        "--name",
        name,
        "--network",
        "none",
        "--memory",
        "384m",
        "--cpus",
        "1",
        "--pids-limit",
        "128",
        "--env",
        "POSTGRES_HOST_AUTH_METHOD=trust",
        "postgres:17-bookworm",
      );
      assert.equal(start.status, 0, start.stderr);
      let ready = false;
      for (let count = 0; count < 100; count++) {
        if (
          docker(
            "exec",
            name,
            "sh",
            "-c",
            'test "$(cat /proc/1/comm)" = postgres && pg_isready -q -U postgres',
          ).status === 0
        ) {
          ready = true;
          break;
        }
        await setTimeout(250);
      }
      assert.ok(ready, "PostgreSQL did not become ready");
      sql(
        "CREATE TABLE orders(id integer, created_at timestamptz, extra text)",
      );
      const check = (kind, fields = {}) =>
        sql(
          `BEGIN READ ONLY; ${checkTemplate(kind, { table: "orders", ...fields }).sql}; ROLLBACK;`,
        ).split("\n")[1];
      assert.equal(check("rows"), "f", "empty table must fail");
      assert.equal(
        check("recent", { column: "created_at", hours: "24" }),
        "f",
        "empty timestamp must fail",
      );
      sql(
        "INSERT INTO orders VALUES (1, NULL, 'extra'), (2, CURRENT_TIMESTAMP - INTERVAL '48 hours', 'extra')",
      );
      assert.equal(check("rows"), "t");
      assert.equal(check("minimum", { minimum: "2" }), "t");
      assert.equal(check("minimum", { minimum: "3" }), "f");
      assert.equal(
        check("recent", { column: "created_at", hours: "24" }),
        "f",
        "stale and null timestamps must fail",
      );
      sql("UPDATE orders SET created_at = CURRENT_TIMESTAMP WHERE id=2");
      assert.equal(check("recent", { column: "created_at", hours: "24" }), "t");
      sql(
        "UPDATE orders SET created_at = CURRENT_TIMESTAMP + INTERVAL '1 day' WHERE id=2",
      );
      assert.equal(
        check("recent", { column: "created_at", hours: "24" }),
        "f",
        "future data must not certify freshness",
      );
      sql("UPDATE orders SET created_at = NULL");
      assert.equal(
        check("recent", { column: "created_at", hours: "24" }),
        "f",
        "all-null timestamps must fail",
      );
      assert.equal(
        check("columns", { columns: "id,created_at" }),
        "t",
        "extra columns are allowed",
      );
      assert.equal(check("columns", { columns: "id,missing" }), "f");
    } finally {
      const cleanup = docker("rm", "--force", "--volumes", name);
      assert.equal(
        cleanup.status,
        0,
        "could not remove template test container",
      );
    }
  },
);
