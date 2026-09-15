import { createWriteStream } from "node:fs";
import { once } from "node:events";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const entryCount = Number(process.argv[2] ?? 100_000);
if (!Number.isInteger(entryCount) || entryCount <= 0) {
  throw new Error("Le nombre d'entrees doit etre un entier positif");
}

const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const outputPath = resolve(projectRoot, "demo-data", `demo-${entryCount}.ndjson`);
const output = createWriteStream(outputPath, { encoding: "utf8" });

// L'écriture ligne par ligne évite de garder tout le dataset en RAM.
for (let index = 0; index < entryCount; index += 1) {
  const entry = JSON.stringify({ key: `demo:${index}`, value: index });
  if (!output.write(`${entry}\n`)) {
    await once(output, "drain");
  }
}

output.end();
await once(output, "finish");
console.log(`${entryCount} entrees ecrites dans ${outputPath}`);
