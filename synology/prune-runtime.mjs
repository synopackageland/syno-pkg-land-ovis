// Keep only the node_modules packages the upstream server entrypoints import.
// Usage: bun prune-runtime.mjs <app-dir> <metafile.json>
// The metafile comes from `bun build server/index.ts --metafile` over the same
// tree; frontend packages are already bundled into dist/ and are not needed.
import { readFileSync, readdirSync, rmSync } from "node:fs";
import { join } from "node:path";

const [app, metafile] = process.argv.slice(2);
const keep = new Set();
for (const input of Object.keys(JSON.parse(readFileSync(metafile, "utf8")).inputs)) {
  const match = /node_modules\/((?:@[^/]+\/)?[^/]+)\//.exec(input);
  if (match) keep.add(match[1]);
}
if (keep.size === 0) throw new Error("metafile lists no runtime packages");
const modules = join(app, "node_modules");
let removed = 0;
for (const entry of readdirSync(modules)) {
  if (entry.startsWith(".")) continue;
  const names = entry.startsWith("@") ? readdirSync(join(modules, entry)).map((name) => `${entry}/${name}`) : [entry];
  for (const name of names) {
    if (!keep.has(name)) { rmSync(join(modules, name), { recursive: true, force: true }); removed++; }
  }
  if (entry.startsWith("@") && readdirSync(join(modules, entry)).length === 0) rmSync(join(modules, entry), { recursive: true });
}
console.error(`runtime packages kept: ${[...keep].sort().join(", ")} (${removed} removed)`);
