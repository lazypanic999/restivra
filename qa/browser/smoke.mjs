// Run against an isolated synthetic installation, never a real customer's data.
import { chromium } from "playwright";
import assert from "node:assert/strict";
import fs from "node:fs/promises";

const base = process.env.RESTIVRA_TEST_BASE || "http://127.0.0.1:7474";
const address = new URL(base);
assert.ok(["localhost", "127.0.0.1"].includes(address.hostname) && address.protocol === "http:");
const token = (await fs.readFile("/run/restivra-test-token", "utf8")).trim();
assert.ok(token.length >= 32);
const artifacts = "/artifacts";
const result = { status: "failed", synthetic_data: true, checks: [], started_at: new Date().toISOString() };
const browser = await chromium.launch({ headless: true, args: ["--no-sandbox", "--disable-dev-shm-usage"] });
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
const page = await context.newPage();
const pageErrors = [];
page.on("pageerror", (error) => pageErrors.push(error.message));

async function checked(name, action) {
  await action();
  result.checks.push(name);
  console.log(`Passed: ${name}`);
}
async function heading(name) { await page.getByRole("heading", { name, exact: true }).waitFor(); }
async function noPageOverflow() {
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), "page overflows viewport");
}

try {
  await checked("default_english_and_real_sign_in", async () => {
    await page.goto(`${base}/?view=setup`);
    await heading("Sign in");
    assert.equal(await page.locator("html").getAttribute("lang"), "en");
    await page.locator('input[name="access-token"]').fill(token);
    await page.getByRole("button", { name: "Sign in", exact: true }).click();
    await heading("System check");
    await page.getByText("Running version", { exact: true }).waitFor();
    assert.equal(pageErrors.length, 0);
  });
  await checked("sanitized_support_download_from_authenticated_ui", async () => {
    const downloadEvent = page.waitForEvent("download");
    await page.getByRole("link", { name: "Download support summary", exact: true }).click();
    const download = await downloadEvent;
    await download.saveAs(`${artifacts}/support-summary.json`);
    const text = await fs.readFile(`${artifacts}/support-summary.json`, "utf8");
    const support = JSON.parse(text);
    assert.equal(support.format, "restivra.support.v1");
    assert.equal(support.database, "online");
    assert.equal(support.worker, "online");
    assert.equal(support.authentication_enabled, true);
    assert.ok(!text.includes(token) && !text.includes("BROWSER_PRIVATE_CANARY") && !text.includes("fixture.dump"));
    result.build = support.build;
    await page.screenshot({ path: `${artifacts}/system-check-en.png`, fullPage: true });
  });
  await checked("turkish_setup_and_mobile_layout", async () => {
    await page.getByRole("combobox", { name: "Interface language" }).selectOption("tr");
    await heading("Sistem kontrolü");
    assert.equal(await page.locator("html").getAttribute("lang"), "tr");
    await page.screenshot({ path: `${artifacts}/system-check-tr.png`, fullPage: true });
    await page.setViewportSize({ width: 390, height: 844 });
    await noPageOverflow();
    await page.screenshot({ path: `${artifacts}/system-check-mobile.png`, fullPage: true });
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.getByRole("combobox", { name: "Arayüz dili" }).selectOption("en");
  });
  await checked("server_project_search_and_cursor_navigation", async () => {
    await page.getByRole("navigation", { name: "Restivra", exact: true }).getByRole("button", { name: "Projects", exact: true }).click();
    const search = page.getByRole("searchbox", { name: "Search projects", exact: true });
    await search.fill("BROWSER_PRIVATE_CANARY");
    await page.getByText("Page 1 · 610 matching projects", { exact: true }).waitFor();
    assert.equal(await page.locator("table tbody tr").count(), 50);
    const first = await page.locator("table tbody tr").first().innerText();
    const navigation = page.getByRole("navigation", { name: "Project pages", exact: true });
    await navigation.getByRole("button", { name: "Older", exact: true }).click();
    await page.getByText("Page 2 · 610 matching projects", { exact: true }).waitFor();
    assert.notEqual(await page.locator("table tbody tr").first().innerText(), first);
    await page.screenshot({ path: `${artifacts}/project-page-two.png` });
    await search.fill("BROWSER_OLDEST_TARGET");
    await page.getByText("Page 1 · 1 matching projects", { exact: true }).waitFor();
    assert.equal(await page.locator("table tbody tr").count(), 1);
    await page.getByRole("button", { name: /^Edit:/ }).click();
    await page.getByRole("dialog").waitFor();
    assert.ok((await page.getByRole("dialog").locator("input").first().inputValue()).includes("BROWSER_OLDEST_TARGET"));
    await page.screenshot({ path: `${artifacts}/old-project-settings.png` });
    await page.keyboard.press("Escape");
    await search.fill("BROWSER_PRIVATE_CANARY");
    await page.getByRole("combobox", { name: "Project view", exact: true }).selectOption("archived");
    await page.getByText("Page 1 · 10 matching projects", { exact: true }).waitFor();
    assert.equal(await page.locator("table tbody tr").count(), 10);
  });
  await checked("projects_route_survives_reload", async () => {
    assert.equal(new URL(page.url()).searchParams.get("view"), "projects");
    await page.reload();
    await page.getByRole("searchbox", { name: "Search projects", exact: true }).waitFor();
    await noPageOverflow();
    assert.equal(pageErrors.length, 0);
  });
  result.status = "passed";
} catch (error) {
  result.failure = error.message.replaceAll(token, "[redacted]");
  await page.screenshot({ path: `${artifacts}/failure.png`, fullPage: true }).catch(() => {});
  process.exitCode = 1;
} finally {
  result.page_errors = pageErrors.map((message) => message.replaceAll(token, "[redacted]"));
  result.finished_at = new Date().toISOString();
  await fs.writeFile(`${artifacts}/browser-summary.json`, JSON.stringify(result, null, 2) + "\n");
  await context.close();
  await browser.close();
}
