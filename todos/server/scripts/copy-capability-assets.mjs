/**
 * Post-build script: copies capability runtime assets into the Nitro output
 * directory so they are available at runtime.
 *
 * The capability handler uses `import.meta.url` to find the plugins directory.
 * After Nitro bundles, `import.meta.url` resolves to `.output/server/index.mjs`,
 * so `dirname(import.meta.url) + '/plugins/'` = `.output/server/plugins/`.
 *
 * Plugin JS modules use relative imports like `../../core-ChWXA2y7.js` to
 * reference shared chunks in the capability dist root. We must also copy
 * those shared JS files to `.output/server/` so the imports resolve.
 *
 * We also copy the project's `capabilities/` directory (instance configs) into
 * `.output/server/capabilities/` because the deploy server's `process.cwd()` is
 * NOT the project root—it's `/app/deploy-srv/`.
 */
import { cpSync, readdirSync, copyFileSync, existsSync, mkdirSync } from "node:fs";
import { resolve, dirname, extname } from "node:path";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const projectRoot = resolve(__dirname, "..");
const require = createRequire(import.meta.url);

let capabilityPkgEntry;
try {
  capabilityPkgEntry = require.resolve("@ks-open/capability/server");
} catch {
  console.log("[copy-capability-assets] @ks-open/capability not installed, skipping");
  process.exit(0);
}
const capabilityDistRoot = resolve(dirname(capabilityPkgEntry), "..");
const pluginsSrc = resolve(capabilityDistRoot, "plugins");

const capabilitiesSrc = resolve(projectRoot, "capabilities");
const serverDir = resolve(projectRoot, ".output", "server");
const pluginsDest = resolve(serverDir, "plugins");
const capabilitiesDest = resolve(serverDir, "capabilities");

console.log(`[copy-capability-assets] capability dist root: ${capabilityDistRoot}`);

// 1. Copy plugins directory (configs, manifests, and plugin JS modules)
console.log(`[copy-capability-assets] plugins source: ${pluginsSrc}`);
if (existsSync(pluginsSrc)) {
  mkdirSync(pluginsDest, { recursive: true });
  cpSync(pluginsSrc, pluginsDest, { recursive: true });
  console.log(`  -> Copied plugins to ${pluginsDest}`);
} else {
  console.error(`[copy-capability-assets] ERROR: plugins not found at ${pluginsSrc}`);
  process.exit(1);
}

// 2. Copy plugins into the externalized node_modules package.
//    Nitro externalizes @ks-open/capability to .output/server/node_modules/.
//    At runtime, import.meta.url resolves to the node_modules path, so the
//    dynamic plugin loader looks for plugins there, not in .output/server/plugins/.
const nmCapabilityDist = resolve(serverDir, "node_modules", "@ks-open", "capability", "dist");
if (existsSync(nmCapabilityDist)) {
  const nmPluginsDest = resolve(nmCapabilityDist, "plugins");
  mkdirSync(nmPluginsDest, { recursive: true });
  cpSync(pluginsSrc, nmPluginsDest, { recursive: true });
  console.log(`  -> Copied plugins to ${nmPluginsDest} (externalized package)`);
}

// 3. Copy shared chunks (core-*.js, handler-*.js, etc.) that plugins import
//    via relative paths like `../../core-ChWXA2y7.js`
const sharedFiles = readdirSync(capabilityDistRoot).filter(
  (f) => extname(f) === ".js" || extname(f) === ".mjs",
);
for (const file of sharedFiles) {
  copyFileSync(resolve(capabilityDistRoot, file), resolve(serverDir, file));
}
if (sharedFiles.length > 0) {
  console.log(`  -> Copied ${sharedFiles.length} shared chunk(s) to ${serverDir}`);
}

// 3b. Also copy shared chunks into the externalized node_modules dist root.
//     Plugins loaded from node_modules resolve relative imports (e.g.
//     ../../wps365-S0qq6oY9.js) against the package's dist/ directory,
//     not .output/server/.
if (existsSync(nmCapabilityDist)) {
  for (const file of sharedFiles) {
    copyFileSync(
      resolve(capabilityDistRoot, file),
      resolve(nmCapabilityDist, file),
    );
  }
  if (sharedFiles.length > 0) {
    console.log(`  -> Copied ${sharedFiles.length} shared chunk(s) to ${nmCapabilityDist}`);
  }
}

// 4. Copy capabilities instance configs
if (existsSync(capabilitiesSrc)) {
  mkdirSync(capabilitiesDest, { recursive: true });
  cpSync(capabilitiesSrc, capabilitiesDest, { recursive: true });
  console.log(`  -> Copied capabilities to ${capabilitiesDest}`);
} else {
  console.log(`[copy-capability-assets] No capabilities/ directory, skipping`);
}

console.log("[copy-capability-assets] Done");
