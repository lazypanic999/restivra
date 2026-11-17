import test from "node:test";
import assert from "node:assert/strict";
import { renderPage } from "./build.mjs";

const template =
  '<html lang="en"><head><!-- app:styles --></head><body><div id="root"></div><!-- app:script --></body></html>';

test("HTML references the generated external assets and preserves language/root markup", () => {
  const html = renderPage(
    template,
    "/assets/app-ABC123.js",
    "/assets/app-DEF456.css",
  );
  assert.match(html, /<html lang="en">/);
  assert.match(html, /<div id="root"><\/div>/);
  assert.match(
    html,
    /<link rel="stylesheet" href="\/assets\/app-DEF456.css" \/>/,
  );
  assert.match(
    html,
    /<script type="module" src="\/assets\/app-ABC123.js"><\/script>/,
  );
  assert.doesNotMatch(html, /app:script|app:styles|\/src\//);
});

test("missing or duplicate template markers fail instead of shipping a broken page", () => {
  assert.throws(() =>
    renderPage("<html/>", "/assets/app-A.js", "/assets/app-B.css"),
  );
  assert.throws(() =>
    renderPage(
      template + "<!-- app:script -->",
      "/assets/app-A.js",
      "/assets/app-B.css",
    ),
  );
});

test("bundle references cannot inject markup or external origins", () => {
  for (const unsafe of [
    "//example.com/x.js",
    "/assets/../app.js",
    '/assets/app.js" onload="x',
    "/src/main.jsx",
  ]) {
    assert.throws(() => renderPage(template, unsafe, "/assets/app-B.css"));
  }
});
