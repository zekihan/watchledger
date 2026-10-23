import { execFileSync } from "node:child_process";
import { lstat, readFile } from "node:fs/promises";
import { lint } from "markdownlint/promise";

const candidates = execFileSync(
  "git",
  ["ls-files", "--cached", "--others", "--exclude-standard", "-z", "--", "*.md"],
  { encoding: "utf8" },
)
  .split("\0")
  .filter(Boolean);
const files = [];
for (const file of new Set(candidates)) {
  try {
    if ((await lstat(file)).isFile()) files.push(file);
  } catch (error) {
    if (error.code !== "ENOENT") throw error;
  }
}
const config = JSON.parse(await readFile(".markdownlint.json", "utf8"));
const result = await lint({ files, config });
const errors = Object.values(result).reduce((total, issues) => total + issues.length, 0);
if (errors > 0) {
  console.error(result.toString());
  process.exitCode = 1;
} else {
  console.log(`Markdown lint passed: ${files.length} files.`);
}
