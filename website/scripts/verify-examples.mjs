// Verifies every Tread program in src/data/examples.ts against a real
// `tread` binary: stdout+stderr and the exit code must match exactly.
// Usage: npm run verify:examples   (uses `tread` on PATH, or Tread=/path/to/tread)
import { execFileSync, spawnSync } from "node:child_process";
import { mkdtempSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
const tread = process.env.TREAD || "tread";
const dataPath = new URL("../src/data/examples.ts", import.meta.url).href;
const { EXAMPLES, HERO_EXAMPLE, CLI_HELP } = await import(dataPath);
const dir = mkdtempSync(join(tmpdir(), "tread-site-"));
let bad = 0;
for (const ex of [HERO_EXAMPLE, ...EXAMPLES]) {
  writeFileSync(join(dir, ex.file), ex.code + "\n");
  const r = spawnSync(tread, [ex.command, ex.file], { cwd: dir, encoding: "utf8" });
  const got = (r.stdout + r.stderr).replace(/\n+$/, "");
  const ok = got === ex.output && r.status === ex.exitCode;
  if (!ok) { bad++; console.log(`MISMATCH ${ex.id} (exit ${r.status}, want ${ex.exitCode})\n--want\n${ex.output}\n--got\n${got}`); }
  else console.log(`ok  ${ex.id.padEnd(14)} tread ${ex.command} ${ex.file}  exit ${r.status}`);
}
const help = execFileSync(tread, ["help"], { encoding: "utf8" }).replace(/\n+$/, "");
console.log(help === CLI_HELP ? "ok  help text matches" : "MISMATCH help"); if (help !== CLI_HELP) bad++;
console.log(bad ? `${bad} FAILURES` : "ALL EXAMPLES VERIFIED");
process.exit(bad ? 1 : 0);
