import { WasmRedis } from "../sdk/wasmRedis";

type BenchmarkSchema = Record<string, string>;

export type BenchmarkReport = {
  iterations: number;
  lines: string[];
};

export async function runBenchmark(
  db: WasmRedis<BenchmarkSchema>,
  iterations: number,
): Promise<BenchmarkReport> {
  const prefix = `benchmark:${Date.now()}`;
  const sequentialKeys = Array.from({ length: iterations }, (_, index) => `${prefix}:single:${index}`);
  const batchKeys = Array.from({ length: iterations }, (_, index) => `${prefix}:batch:${index}`);

  const setTimes = await measureMany(sequentialKeys, (key, index) => db.set(key, String(index)));
  const getTimes = await measureMany(sequentialKeys, (key) => db.get(key));
  await db.batch(sequentialKeys.map((key) => db.cmd.delete(key)));

  const batchStart = performance.now();
  await db.batch(batchKeys.map((key, index) => db.cmd.set(key, String(index))));
  const batchDuration = performance.now() - batchStart;

  const queryRuns = Math.min(10, iterations);
  const rangeTimes = await measureRepeated(queryRuns, () => db.get().where("value", ">=", iterations / 2).exec());
  const containsTimes = await measureRepeated(queryRuns, () => db.get().where("value", "contains", "1").exec());

  await db.batch(batchKeys.map((key) => db.cmd.delete(key)));

  return {
    iterations,
    lines: [
      formatLatency("SET un par un", setTimes),
      formatLatency("GET par cle", getTimes),
      formatLatency("GET WHERE range (B-Tree)", rangeTimes),
      formatLatency("GET WHERE contains", containsTimes),
      `Batch de ${iterations} SET : ${batchDuration.toFixed(2)} ms au total`,
      `Gain batch : ${(sum(setTimes) / Math.max(batchDuration, 0.01)).toFixed(1)}x sur ce test`,
    ],
  };
}

async function measureMany(
  keys: string[],
  operation: (key: string, index: number) => Promise<unknown>,
): Promise<number[]> {
  const times: number[] = [];
  for (let index = 0; index < keys.length; index++) {
    const start = performance.now();
    await operation(keys[index], index);
    times.push(performance.now() - start);
  }
  return times;
}

async function measureRepeated(count: number, operation: () => Promise<unknown>): Promise<number[]> {
  const times: number[] = [];
  for (let index = 0; index < count; index++) {
    const start = performance.now();
    await operation();
    times.push(performance.now() - start);
  }
  return times;
}

function formatLatency(name: string, times: number[]): string {
  return `${name} : p50 ${percentile(times, 50).toFixed(2)} ms | p95 ${percentile(times, 95).toFixed(2)} ms`;
}

function percentile(values: number[], wantedPercentile: number): number {
  const sorted = [...values].sort((left, right) => left - right);
  const index = Math.min(sorted.length - 1, Math.ceil((wantedPercentile / 100) * sorted.length) - 1);
  return sorted[Math.max(0, index)] ?? 0;
}

function sum(values: number[]): number {
  return values.reduce((total, value) => total + value, 0);
}
