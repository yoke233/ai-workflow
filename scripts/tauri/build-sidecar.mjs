import { execFileSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(fileURLToPath(new URL("../..", import.meta.url)));

const run = (cmd, args, options = {}) =>
  execFileSync(cmd, args, {
    cwd: repoRoot,
    stdio: "inherit",
    ...options,
  });

const parseRustHostTriple = () => {
  const output = execFileSync("rustc", ["-Vv"], {
    cwd: repoRoot,
    stdio: ["ignore", "pipe", "pipe"],
    encoding: "utf8",
  });
  const line = output
    .split(/\r?\n/)
    .map((x) => x.trim())
    .find((x) => x.startsWith("host:"));
  if (!line) {
    throw new Error("无法从 `rustc -Vv` 解析 host target triple");
  }
  const triple = line.replace(/^host:\s+/, "").trim();
  if (!triple) {
    throw new Error("解析到的 host target triple 为空");
  }
  return triple;
};

const targetTriple = process.env.TAURI_SIDE_CAR_TARGET_TRIPLE?.trim() || parseRustHostTriple();
const ext = targetTriple.includes("windows") ? ".exe" : "";
const outDir = join(repoRoot, "src-tauri", "binaries");
mkdirSync(outDir, { recursive: true });

const outPath = join(outDir, `ai-flow-${targetTriple}${ext}`);
console.log(`Building Go sidecar -> ${outPath}`);

run("go", ["build", "-o", outPath, "./cmd/ai-flow"]);
