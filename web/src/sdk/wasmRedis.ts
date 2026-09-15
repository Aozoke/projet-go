import { z } from "zod";
import WasmRedisWorker from "../worker/wasmRedis.worker.ts?worker";

export type RedisKey<Schema> = Extract<keyof Schema, string>;

export type Entry<Schema> = {
  key: RedisKey<Schema>;
  value: Schema[RedisKey<Schema>];
};

export type RedisCommand = string;
export type FilterField = "key" | "value";
export type FilterOperator = "equals" | "contains" | ">" | ">=" | "<" | "<=";
export type FilterOperatorFor<Value> = Value extends number
  ? "equals" | ">" | ">=" | "<" | "<="
  : Value extends string
    ? "equals" | "contains"
    : "equals";
export type SetOptions = { ex?: number };

export type RestoreMetrics = {
  snapshotMs: number;
  aofMs: number;
  totalMs: number;
  aofOperationCount: number;
};

export type RedisResult = {
  ok: boolean;
  value?: string;
  entries?: Array<{ key: string; value: string }>;
  error?: string;
};

export type WhereMethod<Schema extends Record<string, unknown>> = {
  <Key extends RedisKey<Schema>>(
    field: Key,
    operator: FilterOperatorFor<Schema[Key]>,
    value: Schema[Key],
  ): WhereQuery<Schema>;
  (field: FilterField, operator: FilterOperator, value: string | number): WhereQuery<Schema>;
};

export type WhereCommandMethod<Schema extends Record<string, unknown>> = {
  <Key extends RedisKey<Schema>>(
    field: Key,
    operator: FilterOperatorFor<Schema[Key]>,
    value: Schema[Key],
  ): RedisCommand;
  (field: FilterField, operator: FilterOperator, value: string | number): RedisCommand;
};

export type WhereQuery<Schema extends Record<string, unknown>> = {
  where: WhereMethod<Schema>;
  exec: () => Promise<Entry<Schema>[]>;
};

export type RedisGet<Schema extends Record<string, unknown>> = {
  <Key extends RedisKey<Schema>>(key: Key): Promise<Schema[Key] | null>;
  (): WhereQuery<Schema>;
};

export type WasmRedis<Schema extends Record<string, unknown>> = {
  set: <Key extends RedisKey<Schema>>(key: Key, value: Schema[Key], options?: SetOptions) => Promise<void>;
  get: RedisGet<Schema>;
  delete: <Key extends RedisKey<Schema>>(key: Key) => Promise<void>;
  list: () => Promise<Entry<Schema>[]>;
  batch: (commands: RedisCommand[]) => Promise<RedisResult[]>;
  flush: () => Promise<void>;
  clear: () => Promise<void>;
  metrics: () => Promise<RestoreMetrics>;
  cmd: {
    set: <Key extends RedisKey<Schema>>(key: Key, value: Schema[Key], options?: SetOptions) => RedisCommand;
    get: <Key extends RedisKey<Schema>>(key: Key) => RedisCommand;
    delete: <Key extends RedisKey<Schema>>(key: Key) => RedisCommand;
    where: WhereCommandMethod<Schema>;
  };
  destroy: () => void;
};

type Filter = {
  field: string;
  operator: FilterOperator;
  value: unknown;
};

const redisResultSchema = z.object({
  ok: z.boolean(),
  value: z.string().optional(),
  entries: z.array(z.object({ key: z.string(), value: z.string() })).optional(),
  error: z.string().optional(),
});

const workerResponseSchema = z.object({
  id: z.string(),
  ok: z.boolean(),
  data: z
    .object({
      results: z.array(redisResultSchema).optional(),
      metrics: z
        .object({
          snapshotMs: z.number(),
          aofMs: z.number(),
          totalMs: z.number(),
          aofOperationCount: z.number().int().nonnegative(),
        })
        .optional(),
    })
    .optional(),
  error: z.string().optional(),
});

const executeRequestSchema = z.object({
  id: z.string(),
  type: z.literal("execute"),
  commands: z.array(z.string().min(1)).min(1),
});

const flushRequestSchema = z.object({ id: z.string(), type: z.literal("flush") });
const clearRequestSchema = z.object({ id: z.string(), type: z.literal("clear") });
const metricsRequestSchema = z.object({ id: z.string(), type: z.literal("metrics") });
const initRequestSchema = z.object({
  id: z.literal("init"),
  type: z.literal("init"),
  reset: z.boolean(),
});

const setOptionsSchema = z.object({ ex: z.number().int().positive().optional() });
const keySchema = z.string().min(1).regex(/^\S+$/, "a key cannot contain spaces");
const filterSchema = z.object({
  field: keySchema,
  operator: z.enum(["equals", "contains", ">", ">=", "<", "<="]),
  value: z.union([z.string(), z.number().finite(), z.boolean()]),
});

type WorkerResponse = z.infer<typeof workerResponseSchema>;

export async function initWasmRedis<Schema extends Record<string, unknown>>(): Promise<WasmRedis<Schema>> {
  const worker = new WasmRedisWorker();
  const resetDatabase = shouldResetBrowserDatabase();
  let nextId = 0;
  const pending = new Map<string, { resolve: (value: WorkerResponse) => void; reject: (error: Error) => void }>();

  const ready = new Promise<void>((resolve, reject) => {
    worker.addEventListener("message", (event: MessageEvent) => {
      if (event.data?.type === "ready") {
        resolve();
      }
      if (event.data?.type === "ready-error") {
        reject(new Error(event.data.error));
      }
    });
  });

  worker.postMessage(initRequestSchema.parse({ id: "init", type: "init", reset: resetDatabase }));

  worker.addEventListener("message", (event: MessageEvent) => {
    const parsed = workerResponseSchema.safeParse(event.data);
    if (!parsed.success) {
      return;
    }

    const handler = pending.get(parsed.data.id);
    if (handler) {
      pending.delete(parsed.data.id);
      handler.resolve(parsed.data);
    }
  });

  worker.addEventListener("error", (event) => {
    for (const handler of pending.values()) {
      handler.reject(new Error(event.message));
    }
    pending.clear();
  });

  await ready;

  const request = async (message: unknown, id: string): Promise<WorkerResponse> => {
    return new Promise<WorkerResponse>((resolve, reject) => {
      pending.set(id, { resolve, reject });
      worker.postMessage(message);
    });
  };

  const send = async (commands: RedisCommand[]): Promise<RedisResult[]> => {
    const id = String(nextId++);
    const message = executeRequestSchema.parse({ id, type: "execute", commands });
    const response = await request(message, id);
    if (!response.ok) {
      throw new Error(response.error ?? "worker error");
    }
    return response.data?.results ?? [];
  };

  const flush = async (): Promise<void> => {
    const id = String(nextId++);
    const response = await request(flushRequestSchema.parse({ id, type: "flush" }), id);
    if (!response.ok) {
      throw new Error(response.error ?? "flush failed");
    }
  };

  const clear = async (): Promise<void> => {
    const id = String(nextId++);
    const response = await request(clearRequestSchema.parse({ id, type: "clear" }), id);
    if (!response.ok) {
      throw new Error(response.error ?? "clear failed");
    }
  };

  const metrics = async (): Promise<RestoreMetrics> => {
    const id = String(nextId++);
    const response = await request(metricsRequestSchema.parse({ id, type: "metrics" }), id);
    if (!response.ok || !response.data?.metrics) {
      throw new Error(response.error ?? "metrics failed");
    }
    return response.data.metrics;
  };

  const whereCommand = (field: string, operator: FilterOperator, value: unknown): RedisCommand => {
    const filter = filterSchema.parse({ field, operator, value });
    return `GET WHERE ${filter.field} ${filter.operator} ${quoteValue(serializeValue(filter.value))}`;
  };

  const cmd = {
    set: <Key extends RedisKey<Schema>>(key: Key, value: Schema[Key], options: SetOptions = {}) => {
      const validKey = keySchema.parse(key);
      const validOptions = setOptionsSchema.parse(options);
      const ttl = validOptions.ex === undefined ? "" : ` EX ${validOptions.ex}`;
      return `SET ${validKey} ${quoteValue(serializeValue(value))}${ttl}`;
    },
    get: <Key extends RedisKey<Schema>>(key: Key) => `GET ${keySchema.parse(key)}`,
    delete: <Key extends RedisKey<Schema>>(key: Key) => `DELETE ${keySchema.parse(key)}`,
    where: whereCommand as WhereCommandMethod<Schema>,
  };

  const list = async (): Promise<Entry<Schema>[]> => {
    const [result] = await send(["ALL"]);
    assertOK(result);
    return decodeEntries<Schema>(result.entries ?? []);
  };

  const createWhereQuery = (filters: Filter[] = []): WhereQuery<Schema> => {
    const where = ((field: string, operator: FilterOperator, value: unknown) => {
      const filter = filterSchema.parse({ field, operator, value });
      return createWhereQuery([...filters, filter]);
    }) as WhereMethod<Schema>;

    return {
      where,
      exec: async () => {
        if (filters.length === 0) {
          return list();
        }

        const results = await send(
          filters.map((filter) => whereCommand(filter.field, filter.operator, filter.value)),
        );
        results.forEach(assertOK);

        const schemaFieldMode = filters.every(
          (filter) => filter.field !== "key" && filter.field !== "value",
        );
        if (schemaFieldMode) {
          if (results.some((result) => (result.entries ?? []).length === 0)) {
            return [];
          }

          const uniqueEntries = new Map<string, { key: string; value: string }>();
          results
            .flatMap((result) => result.entries ?? [])
            .forEach((entry) => uniqueEntries.set(entry.key, entry));
          return decodeEntries<Schema>([...uniqueEntries.values()]);
        }

        const firstEntries = results[0]?.entries ?? [];
        const otherKeys = results
          .slice(1)
          .map((result) => new Set((result.entries ?? []).map((entry) => entry.key)));
        const entries = firstEntries.filter((entry) =>
          otherKeys.every((keys) => keys.has(entry.key)),
        );
        return decodeEntries<Schema>(entries);
      },
    };
  };

  const get = ((key?: RedisKey<Schema>) => {
    if (key === undefined) {
      return createWhereQuery();
    }

    return send([cmd.get(key)]).then(([result]) => {
      if (!result?.ok) {
        return null;
      }
      return deserializeValue(result.value) as Schema[typeof key];
    });
  }) as RedisGet<Schema>;

  return {
    set: async (key, value, options = {}) => {
      const [result] = await send([cmd.set(key, value, options)]);
      assertOK(result);
    },
    get,
    delete: async (key) => {
      const [result] = await send([cmd.delete(key)]);
      assertOK(result);
    },
    list,
    batch: send,
    flush,
    clear,
    metrics,
    cmd,
    destroy: () => worker.terminate(),
  };
}

export function serializeValue(value: unknown): string {
  if (typeof value === "string") {
    return `s:${value}`;
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return `n:${value}`;
  }
  if (typeof value === "boolean") {
    return `b:${value}`;
  }
  const json = JSON.stringify(value);
  if (json === undefined) {
    throw new Error("unsupported Redis value");
  }
  return `j:${json}`;
}

export function deserializeValue(value: string | undefined): unknown {
  if (value === undefined) {
    return undefined;
  }
  if (value.startsWith("s:")) {
    return value.slice(2);
  }
  if (value.startsWith("n:")) {
    return Number(value.slice(2));
  }
  if (value.startsWith("b:")) {
    return value.slice(2) === "true";
  }
  if (value.startsWith("j:")) {
    return JSON.parse(value.slice(2));
  }
  return value;
}

function decodeEntries<Schema extends Record<string, unknown>>(
  entries: Array<{ key: string; value: string }>,
): Entry<Schema>[] {
  return entries.map((entry) => ({ key: entry.key, value: deserializeValue(entry.value) })) as Entry<Schema>[];
}

function quoteValue(text: string): string {
  return `"${text.replaceAll("\\", "\\\\").replaceAll('"', '\\"')}"`;
}

function assertOK(result: RedisResult | undefined): void {
  if (!result?.ok) {
    throw new Error(result?.error ?? "redis command failed");
  }
}

function shouldResetBrowserDatabase(): boolean {
  return new URLSearchParams(window.location.search).get("reset") === "1";
}
