/// <reference lib="webworker" />

type GoRuntime = {
  importObject: WebAssembly.Imports;
  run: (instance: WebAssembly.Instance) => Promise<void>;
};

type GoConstructor = new () => GoRuntime;

type GoGlobal = typeof globalThis & {
  Go?: GoConstructor;
};

type SyncAccessHandle = {
  getSize: () => number;
  write: (buffer: BufferSource, options?: { at?: number }) => number;
  truncate: (size: number) => void;
  flush: () => void;
  close: () => void;
};

type OpfsFileHandle = {
  createSyncAccessHandle: () => Promise<SyncAccessHandle>;
  getFile: () => Promise<File>;
};

type OpfsDirectory = {
  getFileHandle: (name: string, options?: { create?: boolean }) => Promise<OpfsFileHandle>;
};

type Entry = {
  key: string;
  value: string;
};

type RedisResult = {
  ok: boolean;
  value?: string;
  entries?: Entry[];
  error?: string;
};

type PersistedOperation = {
  type: "SET" | "DELETE";
  key: string;
  value?: string;
  expiresAt?: number;
};

type BatchResult = {
  results: RedisResult[];
  writes: PersistedOperation[];
};

type WorkerRequest =
  | {
      id: "init";
      type: "init";
      reset: boolean;
    }
  | {
      id: string;
      type: "execute";
      commands: string[];
    }
  | {
      id: string;
      type: "flush";
    }
  | {
      id: string;
      type: "clear";
    };

const scope = self as DedicatedWorkerGlobalScope & {
  wasmRedisExecute?: (request: string) => string;
  wasmRedisLoadSnapshot?: (snapshot: string) => string;
  wasmRedisDumpSnapshot?: () => string;
  wasmRedisConfigure?: (configuration: string) => string;
  wasmRedisSweepExpired?: () => string;
};

const encoder = new TextEncoder();
const goRuntimeUrl = "/wasm_exec.js";
const flushIntervalMs = readPositiveEnv("VITE_WASMREDIS_FLUSH_INTERVAL_MS", 1000);
const snapshotIntervalMs = readPositiveEnv("VITE_WASMREDIS_SNAPSHOT_INTERVAL_MS", 120000);
const maxBufferSize = readPositiveEnv("VITE_WASMREDIS_MAX_BUFFER_SIZE", 1000);
const expirationSweepIntervalMs = readPositiveEnv("VITE_WASMREDIS_EXPIRATION_SWEEP_INTERVAL_MS", 1000);
const defaultTTLSeconds = readNonNegativeEnv("VITE_WASMREDIS_DEFAULT_TTL_SECONDS", 0);
const btreeDegree = readPositiveEnv("VITE_WASMREDIS_BTREE_ORDER", 8, 2);

let opfsDirectory: OpfsDirectory | null = null;
let pendingAof: PersistedOperation[] = [];
let storageLock = Promise.resolve();
let ready: Promise<void> | null = null;

scope.addEventListener("message", async (event: MessageEvent) => {
  const request = parseRequest(event.data);
  if (request instanceof Error) {
    scope.postMessage({
      id: readRequestId(event.data),
      ok: false,
      error: request.message,
    });
    return;
  }

  if (request.type === "init") {
    if (!ready) {
      ready = start(request.reset);
    }
    return;
  }

  if (!ready) {
    scope.postMessage({
      id: request.id,
      ok: false,
      error: "worker not initialized",
    });
    return;
  }

  await ready;

  try {
    if (request.type === "flush") {
      await flushAof();
      scope.postMessage({
        id: request.id,
        ok: true,
      });
      return;
    }

    if (request.type === "clear") {
      await clearDatabase();
      scope.postMessage({
        id: request.id,
        ok: true,
      });
      return;
    }

    const result = executeGo(request.commands);
    enqueueWrites(result.writes);

    scope.postMessage({
      id: request.id,
      ok: true,
      data: result,
    });
  } catch (error) {
    scope.postMessage({
      id: request.id,
      ok: false,
      error: error instanceof Error ? error.message : "unknown worker error",
    });
  }
});

async function start(resetDatabase: boolean): Promise<void> {
  try {
    await loadWasm();
    configureGoEngine();
    if (resetDatabase) {
      await clearDatabase();
    } else {
      await restoreFromOpfs();
    }

    setInterval(() => {
      void flushAof();
    }, flushIntervalMs);

    setInterval(() => {
      void saveSnapshot();
    }, snapshotIntervalMs);

    setInterval(() => {
      sweepExpired();
    }, expirationSweepIntervalMs);

    scope.postMessage({ type: "ready" });
  } catch (error) {
    scope.postMessage({
      type: "ready-error",
      error: error instanceof Error ? error.message : "unknown startup error",
    });
  }
}

async function loadWasm(): Promise<void> {
  await loadGoRuntime();

  const GoClass = (globalThis as GoGlobal).Go;
  if (!GoClass) {
    throw new Error("go runtime not loaded");
  }

  const go = new GoClass();
  const response = await fetch("/wasmredis.wasm");
  const bytes = await response.arrayBuffer();
  const wasm = await WebAssembly.instantiate(bytes, go.importObject);

  void go.run(wasm.instance);
  await waitForGoBridge();
}

async function loadGoRuntime(): Promise<void> {
  if ((globalThis as GoGlobal).Go) {
    return;
  }

  const response = await fetch(goRuntimeUrl);
  if (!response.ok) {
    throw new Error("go runtime not found");
  }

  const runtimeCode = await response.text();
  new Function(runtimeCode)();
}

async function waitForGoBridge(): Promise<void> {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (
      scope.wasmRedisExecute &&
      scope.wasmRedisLoadSnapshot &&
      scope.wasmRedisDumpSnapshot &&
      scope.wasmRedisConfigure &&
      scope.wasmRedisSweepExpired
    ) {
      return;
    }

    await new Promise((resolve) => setTimeout(resolve, 10));
  }

  throw new Error("wasm bridge not ready");
}

function configureGoEngine(): void {
  if (!scope.wasmRedisConfigure) {
    throw new Error("wasm configure function missing");
  }

  const response = JSON.parse(
    scope.wasmRedisConfigure(JSON.stringify({ defaultTTLSeconds, btreeDegree })),
  ) as unknown;
  if (!isRecord(response) || response.ok !== true) {
    throw new Error("invalid Go engine configuration");
  }
}

function sweepExpired(): void {
  if (!scope.wasmRedisSweepExpired) {
    return;
  }

  const result = parseBatchResult(JSON.parse(scope.wasmRedisSweepExpired()));
  enqueueWrites(result.writes);
}

function executeGo(commands: string[]): BatchResult {
  if (!scope.wasmRedisExecute) {
    throw new Error("wasm execute function missing");
  }

  const response = scope.wasmRedisExecute(JSON.stringify({ commands }));
  return parseBatchResult(JSON.parse(response));
}

async function restoreFromOpfs(): Promise<void> {
  const snapshot = await readOpfsText("snapshot.json");
  if (snapshot.trim() !== "" && scope.wasmRedisLoadSnapshot) {
    scope.wasmRedisLoadSnapshot(snapshot);
  }

  const aof = await readOpfsText("aof.log");
  const commands = aof
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean)
    .map((line) => JSON.parse(line) as PersistedOperation)
    .map(operationToCommand);

  if (commands.length > 0) {
    executeGo(commands);
  }
}

function enqueueWrites(writes: PersistedOperation[]): void {
  if (writes.length === 0) {
    return;
  }

  pendingAof = pendingAof.concat(writes);

  if (pendingAof.length >= maxBufferSize) {
    void flushAof();
  }
}

function flushAof(): Promise<void> {
  return withStorageLock(writePendingAof);
}

function saveSnapshot(): Promise<void> {
  if (!scope.wasmRedisDumpSnapshot) {
    return Promise.resolve();
  }

  return withStorageLock(async () => {
    await writePendingAof();
    const snapshot = scope.wasmRedisDumpSnapshot?.() ?? "{}";
    await writeOpfsText("snapshot.json", snapshot);
    await writeOpfsText("aof.log", "");
  });
}

function clearDatabase(): Promise<void> {
  return withStorageLock(async () => {
    pendingAof = [];

    if (scope.wasmRedisLoadSnapshot) {
      scope.wasmRedisLoadSnapshot("{}");
    }

    await writeOpfsText("snapshot.json", "{}");
    await writeOpfsText("aof.log", "");
  });
}

async function writePendingAof(): Promise<void> {
  if (pendingAof.length === 0) {
    return;
  }

  const writes = pendingAof;
  pendingAof = [];
  const lines = writes.map((operation) => JSON.stringify(operation)).join("\n") + "\n";
  await appendOpfsText("aof.log", lines);
}

function withStorageLock(task: () => Promise<void>): Promise<void> {
  storageLock = storageLock.catch(() => undefined).then(task);
  return storageLock;
}

async function getOpfsDirectory(): Promise<OpfsDirectory | null> {
  if (opfsDirectory) {
    return opfsDirectory;
  }

  const storage = navigator.storage as StorageManager & {
    getDirectory?: () => Promise<OpfsDirectory>;
  };

  if (!storage.getDirectory) {
    return null;
  }

  opfsDirectory = await storage.getDirectory();
  return opfsDirectory;
}

async function readOpfsText(fileName: string): Promise<string> {
  const directory = await getOpfsDirectory();
  if (!directory) {
    return "";
  }

  const fileHandle = await directory.getFileHandle(fileName, { create: true });
  const file = await fileHandle.getFile();
  return file.text();
}

async function writeOpfsText(fileName: string, text: string): Promise<void> {
  const directory = await getOpfsDirectory();
  if (!directory) {
    return;
  }

  const fileHandle = await directory.getFileHandle(fileName, { create: true });
  const accessHandle = await fileHandle.createSyncAccessHandle();

  try {
    const bytes = encoder.encode(text);
    accessHandle.truncate(0);
    accessHandle.write(bytes, { at: 0 });
    accessHandle.flush();
  } finally {
    accessHandle.close();
  }
}

async function appendOpfsText(fileName: string, text: string): Promise<void> {
  const directory = await getOpfsDirectory();
  if (!directory) {
    return;
  }

  const fileHandle = await directory.getFileHandle(fileName, { create: true });
  const accessHandle = await fileHandle.createSyncAccessHandle();

  try {
    const bytes = encoder.encode(text);
    accessHandle.write(bytes, { at: accessHandle.getSize() });
    accessHandle.flush();
  } finally {
    accessHandle.close();
  }
}

function operationToCommand(operation: PersistedOperation): string {
  if (operation.type === "DELETE") {
    return `DELETE ${operation.key}`;
  }

  let ttl = "";
  if (operation.expiresAt !== undefined) {
    const remainingSeconds = Math.ceil((operation.expiresAt - Date.now()) / 1000);
    if (remainingSeconds <= 0) {
      return `DELETE ${operation.key}`;
    }
    ttl = ` EX ${remainingSeconds}`;
  }

  return `SET ${operation.key} "${(operation.value ?? "").replaceAll('"', '\\"')}"${ttl}`;
}

function parseRequest(value: unknown): WorkerRequest | Error {
  if (!isRecord(value) || typeof value.id !== "string" || typeof value.type !== "string") {
    return new Error("invalid worker request");
  }

  if (value.type === "init" && value.id === "init" && typeof value.reset === "boolean") {
    return {
      id: "init",
      type: "init",
      reset: value.reset,
    };
  }

  if (value.type === "flush") {
    return {
      id: value.id,
      type: "flush",
    };
  }

  if (value.type === "clear") {
    return {
      id: value.id,
      type: "clear",
    };
  }

  if (value.type === "execute" && Array.isArray(value.commands) && value.commands.every(isNotEmptyString)) {
    return {
      id: value.id,
      type: "execute",
      commands: value.commands,
    };
  }

  return new Error("invalid worker request");
}

function parseBatchResult(value: unknown): BatchResult {
  if (!isRecord(value) || !Array.isArray(value.results) || !Array.isArray(value.writes)) {
    throw new Error("invalid wasm result");
  }

  if (!value.results.every(isRedisResult) || !value.writes.every(isPersistedOperation)) {
    throw new Error("invalid wasm result");
  }

  return {
    results: value.results,
    writes: value.writes,
  };
}

function readRequestId(value: unknown): string {
  if (isRecord(value) && typeof value.id === "string") {
    return value.id;
  }

  return "unknown";
}

function isRedisResult(value: unknown): value is RedisResult {
  return isRecord(value) && typeof value.ok === "boolean";
}

function isPersistedOperation(value: unknown): value is PersistedOperation {
  return (
    isRecord(value) &&
    (value.type === "SET" || value.type === "DELETE") &&
    typeof value.key === "string" &&
    (value.value === undefined || typeof value.value === "string") &&
    (value.expiresAt === undefined || typeof value.expiresAt === "number")
  );
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isNotEmptyString(value: unknown): value is string {
  return typeof value === "string" && value.trim() !== "";
}

function readPositiveEnv(name: string, fallback: number, minimum = 1): number {
  const env = import.meta.env as Record<string, string | undefined>;
  const value = Number(env[name]);
  return Number.isFinite(value) && value >= minimum ? value : fallback;
}

function readNonNegativeEnv(name: string, fallback: number): number {
  const env = import.meta.env as Record<string, string | undefined>;
  const value = Number(env[name]);
  return Number.isFinite(value) && value >= 0 ? value : fallback;
}
