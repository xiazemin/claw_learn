#!/usr/bin/env node
/**
 * Cross-platform zip utility that always uses forward-slash paths,
 * ensuring compatibility with Linux containers regardless of build OS.
 *
 * Usage: node scripts/zip-dir.mjs <source-dir> <output-zip>
 */
import { createWriteStream, mkdirSync, unlinkSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import archiver from "archiver";

const [sourceDir, outputZip, destPrefix] = process.argv.slice(2);
if (!sourceDir || !outputZip) {
  console.error("Usage: node zip-dir.mjs <source-dir> <output-zip> [dest-prefix]");
  process.exit(1);
}

const absSource = resolve(sourceDir);
const absOutput = resolve(outputZip);

mkdirSync(dirname(absOutput), { recursive: true });
if (existsSync(absOutput)) unlinkSync(absOutput);

const output = createWriteStream(absOutput);
const archive = archiver("zip", { zlib: { level: 9 } });

archive.on("error", (err) => {
  throw err;
});
output.on("close", () => {
  console.log(`[zip-dir] Created ${absOutput} (${archive.pointer()} bytes)`);
});

archive.pipe(output);
archive.directory(absSource, destPrefix || false);
await archive.finalize();
