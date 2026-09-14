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
export type SetOptions = { ex?: number };

export type RedisResult = {
  ok: boolean;
  value?: string;
  entries?: Array<{ key: string; value: string }>;
  error?: string;
};

export type WhereQuery<Schema extends Record<string, unknown>> = {
  where: (field: FilterField, operator: FilterOperator, value: string | number) => WhereQuery<Schema>;
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
  cmd: {
    set: <Key extends RedisKey<Schema>>(key: Key, value: Schema[Key], options?: SetOptions) => RedisCommand;
    get: <Key extends RedisKey<Schema>>(key: Key) => RedisCommand;
    delete: <Key extends RedisKey<Schema>>(key: Key) => RedisCommand;
    where: (field: FilterField, operator: FilterOperator, value: string | number) => RedisCommand;
  };
  destroy: () => void;
};

type Filter = {
  field: FilterField;
  operator: FilterOperator;
  value: string | number;
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
  data: z.object({ results: z.array(redisResultSchema) }).optional(),
  error: z.string().optional(),
});

const executeRequestSchema = z.object({
  id: z.string(),
  type: z.literal("execute"),
  commands: z.array(z.string().min(1)).min(1),
});

const flushRequestSchema = z.object({ id: z.string(), type: z.literal("flush") });
const clearRequestSchema = z.object({ id: z.string(), type: z.literal("clear") });
const initRequestSchema = z.object({
  id: z.literal("init"),
  type: z.literal("init"),
  reset: z.boolean(),
});

const setOptionsSchema = z.object({ ex: z.number().int().positive().optional() });
const filterSchema = z.object({
  field: z.enum(["key", "value"]),
  operator: z.enum(["equals", "contains", ">", ">=", "<", "<="]),
  value: z.union([z.string(), z.number()]),
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

  const cmd = {
    set: <Key extends RedisKey<Schema>>(key: Key, value: Schema[Key], options: SetOptions = {}) => {
      const validOptions = setOptionsSchema.parse(options);
      const ttl = validOptions.ex === undefined ? "" : ` EX ${validOptions.ex}`;
      return `SET ${key} ${quoteValue(value)}${ttl}`;
    },
    get: <Key extends RedisKey<Schema>>(key: Key) => `GET ${key}`,
    delete: <Key extends RedisKey<Schema>>(key: Key) => `DELETE ${key}`,
    where: (field: FilterField, operator: FilterOperator, value: string | number) => {
      const filter = filterSchema.parse({ field, operator, value });
      return `GET WHERE ${filter.field} ${filter.operator} ${quoteValue(filter.value)}`;
    },
  };

  const list = async (): Promise<Entry<Schema>[]> => {
    const [result] = await send(["ALL"]);
    assertOK(result);
    return (result.entries ?? []) as Entry<Schema>[];
  };

  const createWhereQuery = (filters: Filter[] = []): WhereQuery<Schema> => ({
    where: (field, operator, value) => {
      const filter = filterSchema.parse({ field, operator, value });
      return createWhereQuery([...filters, filter]);
    },
    exec: async () => {
      if (filters.length === 0) {
        return list();
      }

      const results = await send(filters.map((filter) => cmd.where(filter.field, filter.operator, filter.value)));
      results.forEach(assertOK);
      const firstEntries = results[0]?.entries ?? [];
      const otherKeys = results.slice(1).map((result) => new Set((result.entries ?? []).map((entry) => entry.key)));
      const entries = firstEntries.filter((entry) => otherKeys.every((keys) => keys.has(entry.key)));
      return entries as Entry<Schema>[];
    },
  });

  const get = ((key?: RedisKey<Schema>) => {
    if (key === undefined) {
      return createWhereQuery();
    }

    return send([cmd.get(key)]).then(([result]) => {
      if (!result?.ok) {
        return null;
      }
      return result.value as Schema[typeof key];
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
    cmd,
    destroy: () => worker.terminate(),
  };
}

function quoteValue(value: unknown): string {
  const text = typeof value === "string" ? value : JSON.stringify(value);
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
