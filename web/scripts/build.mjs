import * as esbuild from "esbuild";
import * as fs from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = fileURLToPath(new URL("../", import.meta.url));
const projectRoot = path.resolve(webRoot, "..");
const outputRoot = path.join(projectRoot, "internal", "webui", "dist");
const entryPoint = path.join(webRoot, "src", "main.jsx");
const styleMarker = "<!-- app:styles -->";
const scriptMarker = "<!-- app:script -->";

// The only URLs injected into the template are our hashed bundle filenames.
// Fail instead of producing a page with missing/ambiguous or unsafe entry tags.
export function renderPage(template, javascript, stylesheet) {
  for (const [url, extension] of [
    [javascript, "js"],
    [stylesheet, "css"],
  ]) {
    if (!new RegExp(`^/assets/[a-zA-Z0-9_-]+\\.${extension}$`).test(url)) {
      throw new Error("Unexpected bundle URL");
    }
  }
  if (
    template.split(styleMarker).length !== 2 ||
    template.split(scriptMarker).length !== 2
  ) {
    throw new Error("HTML must contain one style marker and one script marker");
  }
  return template
    .replace(styleMarker, `<link rel="stylesheet" href="${stylesheet}" />`)
    .replace(
      scriptMarker,
      `<script type="module" src="${javascript}"></script>`,
    );
}

async function prepareOutput() {
  await fs.mkdir(outputRoot, { recursive: true });
  // Resolve physical paths before recursive removal. Refuse redirected output
  // directories, including a dist/ or parent symlink outside this checkout.
  const actual = await fs.realpath(outputRoot);
  const expected = path.join(
    await fs.realpath(projectRoot),
    "internal",
    "webui",
    "dist",
  );
  if (path.relative(expected, actual) !== "")
    throw new Error("Refusing unexpected output directory");
  for (const entry of await fs.readdir(outputRoot)) {
    if (entry !== ".gitkeep")
      await fs.rm(path.join(outputRoot, entry), {
        recursive: true,
        force: true,
      });
  }
}

async function writeAtomic(filename, contents) {
  await fs.mkdir(path.dirname(filename), { recursive: true });
  const temporary = `${filename}.tmp`;
  await fs.writeFile(temporary, contents);
  await fs.rename(temporary, filename);
}

function assetURL(filename) {
  const relative = path
    .relative(outputRoot, filename)
    .split(path.sep)
    .join("/");
  if (!relative.startsWith("assets/") || relative.includes(".."))
    throw new Error("Unexpected output path");
  return `/${relative}`;
}

export async function buildSite({ watch = false } = {}) {
  await prepareOutput();
  const template = await fs.readFile(path.join(webRoot, "index.html"), "utf8");
  const publish = {
    name: "publish-static-assets",
    setup(build) {
      build.onEnd(async (result) => {
        if (result.errors.length) return;
        const entry = Object.entries(result.metafile.outputs).find(
          ([filename, info]) =>
            filename.endsWith(".js") &&
            info.entryPoint &&
            path.resolve(webRoot, info.entryPoint) === entryPoint,
        );
        if (!entry || !entry[1].cssBundle)
          throw new Error("JavaScript or stylesheet bundle missing");
        for (const file of result.outputFiles) {
          const relative = path.relative(outputRoot, file.path);
          if (relative.startsWith("..") || path.isAbsolute(relative))
            throw new Error("Output escaped asset directory");
          await writeAtomic(file.path, file.contents);
        }
        await fs.cp(path.join(webRoot, "public"), outputRoot, {
          recursive: true,
          filter: async (source) => {
            if ((await fs.lstat(source)).isSymbolicLink())
              throw new Error("Public assets must not be symlinks");
            return true;
          },
        });
        // Publish HTML last. During watch rebuilds, retain older hashed assets
        // so already-open tabs do not lose their fonts or imported resources.
        const html = renderPage(
          template,
          assetURL(path.resolve(webRoot, entry[0])),
          assetURL(path.resolve(webRoot, entry[1].cssBundle)),
        );
        await writeAtomic(path.join(outputRoot, "index.html"), html);
        console.log(
          `Built ${path.relative(projectRoot, outputRoot)} (${result.outputFiles.length} assets)`,
        );
      });
    },
  };
  const options = {
    absWorkingDir: webRoot,
    entryPoints: { app: entryPoint },
    outdir: path.join(outputRoot, "assets"),
    publicPath: "/assets",
    entryNames: "[name]-[hash]",
    assetNames: "[name]-[hash]",
    bundle: true,
    format: "esm",
    platform: "browser",
    target: "es2022",
    jsx: "automatic",
    minify: !watch,
    define: {
      "process.env.NODE_ENV": JSON.stringify(
        watch ? "development" : "production",
      ),
    },
    loader: {
      ".woff": "file",
      ".woff2": "file",
      ".ttf": "file",
      ".otf": "file",
      ".png": "file",
      ".svg": "file",
      ".jpg": "file",
      ".webp": "file",
    },
    metafile: true,
    write: false,
    plugins: [publish],
  };
  if (!watch) {
    await esbuild.build(options);
    return;
  }
  const context = await esbuild.context(options);
  await context.watch();
  console.log(
    "Watching imported frontend files. Serve with: restivra serve --assets-dir internal/webui/dist",
  );
  // The only listener is the existing Go API. This process opens no HTTP port.
  const stop = async () => {
    await context.dispose();
    process.exit(0);
  };
  process.once("SIGINT", stop);
  process.once("SIGTERM", stop);
}

if (
  process.argv[1] &&
  path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)
) {
  const arguments_ = process.argv.slice(2);
  if (
    arguments_.some((value) => value !== "--watch") ||
    arguments_.length > 1
  ) {
    console.error("Usage: node scripts/build.mjs [--watch]");
    process.exitCode = 2;
  } else {
    try {
      await buildSite({ watch: arguments_.includes("--watch") });
    } catch (error) {
      console.error(error.message);
      process.exitCode = 1;
    }
  }
}
