import test from "node:test";
import assert from "node:assert/strict";
import { messages, readLanguage, text, translateMessage } from "./i18n.js";
test("English is the default and only an explicit Turkish preference changes it", () => {
  assert.equal(readLanguage({ getItem: () => null }), "en");
  assert.equal(readLanguage({ getItem: () => "tr" }), "tr");
  assert.equal(readLanguage({ getItem: () => "invalid" }), "en");
  assert.equal(
    readLanguage({
      getItem: () => {
        throw Error("blocked");
      },
    }),
    "en",
  );
});
test("Every interface message has both languages with matching placeholders", () => {
  for (const [key, pair] of Object.entries(messages)) {
    assert.equal(pair.length, 2, key);
    assert.ok(
      pair.every((value) => typeof value === "string" && value.trim()),
      key,
    );
    const placeholders = (value) =>
      [...value.matchAll(/\{(\w+)\}/g)].map((match) => match[1]).sort();
    assert.deepEqual(placeholders(pair[0]), placeholders(pair[1]), key);
  }
  assert.equal(
    text("en", "summary", { projects: 2, passed: 3, failed: 0, active: 1 }),
    "2 projects / 3 passed / 0 failed / 1 active",
  );
  assert.equal(text("tr", "runTest"), "Testi çalıştır");
});
test("Stored messages switch language without modifying user-provided identifiers", () => {
  assert.equal(
    translateMessage("one or more verification checks did not pass", "tr"),
    "bir veya daha fazla doğrulama kontrolü geçmedi",
  );
  assert.equal(
    translateMessage("geçersiz proje kimliği", "en"),
    "invalid project ID",
  );
  const message =
    'check "Customer orders" must be a single SELECT without semicolons, comments or backslashes';
  assert.match(translateMessage(message, "tr"), /"Customer orders"/);
});
