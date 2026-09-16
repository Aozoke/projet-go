import { FormEvent, memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { runBenchmark } from "./benchmark/runBenchmark";
import { measureScrollFps } from "./benchmark/measureScrollFps";
import { FilterField, FilterOperator, initWasmRedis, WasmRedis } from "./sdk/wasmRedis";
import {
  clearEntries,
  removeEntry,
  replaceEntries,
  StoreEntry,
  upsertEntry,
  useEntry,
  useEntryKeys,
} from "./store/entryStore";
import { getVirtualRange } from "./virtual/getVirtualRange";

type Schema = Record<string, string | number>;

const rowHeight = readNumberEnv("VITE_WASMREDIS_VIRTUAL_ROW_HEIGHT", 42);
const overscan = readNumberEnv("VITE_WASMREDIS_VIRTUAL_OVERSCAN", 8);
const virtualViewportHeight = readNumberEnv("VITE_WASMREDIS_VIRTUAL_VIEWPORT_HEIGHT", 520);
const demoEntryCount = readNumberEnv("VITE_WASMREDIS_DEMO_ENTRY_COUNT", 100_000);
const demoChunkSize = readNumberEnv("VITE_WASMREDIS_DEMO_CHUNK_SIZE", 1_000);
const benchmarkIterations = readNumberEnv("VITE_WASMREDIS_BENCHMARK_ITERATIONS", 30);
const scrollBenchmarkDurationMs = readNumberEnv("VITE_WASMREDIS_SCROLL_BENCHMARK_DURATION_MS", 1000);

export function App() {
  const dbRef = useRef<WasmRedis<Schema> | null>(null);
  const virtualListRef = useRef<HTMLDivElement | null>(null);
  const entryKeys = useEntryKeys();
  const [ready, setReady] = useState(false);
  const [key, setKey] = useState("");
  const [value, setValue] = useState("");
  const [ttl, setTTL] = useState("");
  const [filterField, setFilterField] = useState<FilterField>("value");
  const [filterOperator, setFilterOperator] = useState<FilterOperator>("contains");
  const [filterValue, setFilterValue] = useState("");
  const [status, setStatus] = useState("Chargement du moteur...");
  const [benchmarkLines, setBenchmarkLines] = useState<string[]>([]);
  const [seeding, setSeeding] = useState(false);

  const loadEntries = useCallback(async () => {
    const db = dbRef.current;
    if (!db) {
      return;
    }

    const nextEntries = await db.list();
    replaceEntries(nextEntries.map((entry) => ({ key: entry.key, value: String(entry.value) })));
  }, []);

  useEffect(() => {
    let alive = true;

    initWasmRedis<Schema>()
      .then(async (db) => {
        if (!alive) {
          db.destroy();
          return;
        }

        dbRef.current = db;
        setReady(true);
        setStatus("Moteur pret");
        await loadEntries();
      })
      .catch((error: Error) => setStatus(error.message));

    return () => {
      alive = false;
      dbRef.current?.destroy();
    };
  }, [loadEntries]);

  const saveEntry = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const db = dbRef.current;
    const cleanKey = key.trim();
    if (!db || cleanKey === "") {
      return;
    }

    const ttlSeconds = Number(ttl);
    const options = Number.isInteger(ttlSeconds) && ttlSeconds > 0 ? { ex: ttlSeconds } : undefined;
    await db.set(cleanKey, value, options);
    upsertEntry({ key: cleanKey, value });
    setKey("");
    setValue("");
    setTTL("");
    setStatus(options ? `Sauve pour ${ttlSeconds}s : ${cleanKey}` : `Sauve : ${cleanKey}`);

    if (options) {
      window.setTimeout(() => void loadEntries(), ttlSeconds * 1000 + 200);
    }
  };

  const deleteEntry = useCallback(async (entryKey: string) => {
    if (!dbRef.current) {
      return;
    }
    await dbRef.current.delete(entryKey);
    removeEntry(entryKey);
    setStatus(`Supprime : ${entryKey}`);
  }, []);

  const editEntry = useCallback((entry: StoreEntry) => {
    setKey(entry.key);
    setValue(entry.value);
  }, []);

  const filterEntries = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const db = dbRef.current;
    if (!db || filterValue.trim() === "") {
      return;
    }

    try {
      const wantedValue = isRangeOperator(filterOperator) ? Number(filterValue) : filterValue;
      if (typeof wantedValue === "number" && !Number.isFinite(wantedValue)) {
        setStatus("La valeur du filtre doit etre un nombre");
        return;
      }
      const results = await db.get().where(filterField, filterOperator, wantedValue).exec();
      replaceEntries(results.map((entry) => ({ key: entry.key, value: String(entry.value) })));
      setStatus(`${results.length} resultat(s)`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Filtre invalide");
    }
  };

  const seedDemo = async () => {
    const db = dbRef.current;
    if (!db || seeding) {
      return;
    }

    setSeeding(true);
    try {
      setStatus(`Creation de ${formatCount(demoEntryCount)} entrees...`);
      for (let start = 0; start < demoEntryCount; start += demoChunkSize) {
        const end = Math.min(start + demoChunkSize, demoEntryCount);
        const commands = Array.from({ length: end - start }, (_, offset) =>
          db.cmd.set(`demo:${start + offset}`, start + offset),
        );
        await db.batch(commands);
        setStatus(`Creation... ${formatCount(end)}/${formatCount(demoEntryCount)}`);
      }

      setStatus("Persistance et chargement de la liste...");
      await db.flush();
      await loadEntries();
      setStatus(`${formatCount(demoEntryCount)} entrees pretes`);
    } catch (error) {
      setStatus(error instanceof Error ? error.message : "Erreur pendant le seed");
    } finally {
      setSeeding(false);
    }
  };

  const runBenchmarkNow = async () => {
    const db = dbRef.current;
    if (!db) {
      return;
    }

    setStatus("Benchmark en cours...");
    const report = await runBenchmark(db, benchmarkIterations);
    const fps = virtualListRef.current
      ? await measureScrollFps(virtualListRef.current, scrollBenchmarkDurationMs)
      : null;
    setBenchmarkLines([
      ...report.lines,
      fps === null ? "FPS scroll : liste trop courte" : `FPS scroll : ${fps.toFixed(1)}`,
    ]);
    await loadEntries();
    setStatus(`Benchmark termine (${report.iterations} iterations)`);
  };

  const clearDatabase = async () => {
    if (!dbRef.current) {
      return;
    }
    await dbRef.current.clear();
    clearEntries();
    setBenchmarkLines([]);
    setStatus("Base videe");
  };

  return (
    <main className="app-shell">
      <header className="topbar">
        <div>
          <p className="eyebrow">WasmRedis</p>
          <h1>Redis lite dans le navigateur</h1>
        </div>
        <div className="status">{status}</div>
      </header>

      <section className="editor">
        <form onSubmit={saveEntry} className="entry-form">
          <label>
            Cle
            <input value={key} onChange={(event) => setKey(event.target.value)} placeholder="name" />
          </label>
          <label>
            Valeur
            <input value={value} onChange={(event) => setValue(event.target.value)} placeholder="matt" />
          </label>
          <label>
            TTL en secondes
            <input type="number" min="1" value={ttl} onChange={(event) => setTTL(event.target.value)} placeholder="optionnel" />
          </label>
          <button disabled={!ready || seeding || key.trim() === ""} type="submit">Sauver</button>
          <button disabled={!ready || seeding} type="button" onClick={seedDemo}>
            {seeding ? "Seed en cours..." : `Seed ${formatCount(demoEntryCount)}`}
          </button>
          <button disabled={!ready || seeding} type="button" onClick={() => dbRef.current?.flush()}>Flush</button>
          <button disabled={!ready || seeding} type="button" className="danger" onClick={clearDatabase}>Vider</button>
        </form>
      </section>

      <section className="filter-section">
        <form className="filter-form" onSubmit={filterEntries}>
          <select
            value={filterField}
            onChange={(event) => {
              const nextField = event.target.value as FilterField;
              setFilterField(nextField);
              if (nextField === "key" && isRangeOperator(filterOperator)) {
                setFilterOperator("contains");
              }
            }}
          >
            <option value="key">Cle</option>
            <option value="value">Valeur</option>
          </select>
          <select value={filterOperator} onChange={(event) => setFilterOperator(event.target.value as FilterOperator)}>
            <option value="equals">equals</option>
            <option value="contains">contains</option>
            <option disabled={filterField === "key"} value=">">&gt;</option>
            <option disabled={filterField === "key"} value=">=">&gt;=</option>
            <option disabled={filterField === "key"} value="<">&lt;</option>
            <option disabled={filterField === "key"} value="<=">&lt;=</option>
          </select>
          <input value={filterValue} onChange={(event) => setFilterValue(event.target.value)} placeholder="Valeur du filtre" />
          <button disabled={!ready || seeding || filterValue.trim() === ""} type="submit">Filtrer</button>
          <button disabled={!ready || seeding} type="button" onClick={loadEntries}>Tout afficher</button>
        </form>
      </section>

      {benchmarkLines.length > 0 && (
        <section className="benchmark-panel">
          {benchmarkLines.map((line) => <div key={line}>{line}</div>)}
        </section>
      )}

      <section className="table-section">
        <div className="table-header">
          <strong>{entryKeys.length.toLocaleString("fr-FR")} entrees</strong>
          <div className="table-actions">
            <button disabled={!ready || seeding} type="button" onClick={runBenchmarkNow}>Benchmark</button>
            <button disabled={!ready || seeding} type="button" onClick={loadEntries}>Recharger</button>
          </div>
        </div>
        <VirtualList
          containerRef={virtualListRef}
          entryKeys={entryKeys}
          onEdit={editEntry}
          onDelete={deleteEntry}
        />
      </section>
    </main>
  );
}

type VirtualListProps = {
  containerRef: React.RefObject<HTMLDivElement | null>;
  entryKeys: string[];
  onEdit: (entry: StoreEntry) => void;
  onDelete: (key: string) => void;
};

const VirtualList = memo(function VirtualList({ containerRef, entryKeys, onEdit, onDelete }: VirtualListProps) {
  const [scrollTop, setScrollTop] = useState(0);
  const [height, setHeight] = useState(520);

  useEffect(() => {
    const element = containerRef.current;
    if (!element) {
      return;
    }

    const observer = new ResizeObserver(() => setHeight(element.clientHeight));
    observer.observe(element);
    setHeight(element.clientHeight);
    return () => observer.disconnect();
  }, []);

  const { startIndex, endIndex, totalHeight } = getVirtualRange({
    itemCount: entryKeys.length,
    rowHeight,
    viewportHeight: height,
    scrollTop,
    overscan,
  });
  const visibleKeys = useMemo(() => entryKeys.slice(startIndex, endIndex), [entryKeys, startIndex, endIndex]);

  return (
    <div
      ref={containerRef}
      className="virtual-list"
      style={{ height: virtualViewportHeight }}
      onScroll={(event) => setScrollTop(event.currentTarget.scrollTop)}
    >
      <div className="virtual-spacer" style={{ height: totalHeight }}>
        {visibleKeys.map((entryKey, index) => (
          <EntryRow
            key={entryKey}
            entryKey={entryKey}
            top={(startIndex + index) * rowHeight}
            height={rowHeight}
            onEdit={onEdit}
            onDelete={onDelete}
          />
        ))}
      </div>
    </div>
  );
});

type EntryRowProps = {
  entryKey: string;
  top: number;
  height: number;
  onEdit: (entry: StoreEntry) => void;
  onDelete: (key: string) => void;
};

const EntryRow = memo(function EntryRow({ entryKey, top, height, onEdit, onDelete }: EntryRowProps) {
  const entry = useEntry(entryKey);
  const renderCount = useRef(0);
  renderCount.current += 1;

  if (!entry) {
    return null;
  }

  return (
    <div className="entry-row" style={{ height, transform: `translateY(${top}px)` }}>
      <span className="cell key-cell">{entry.key}</span>
      <span className="cell value-cell">{entry.value}</span>
      <span className="render-count">{renderCount.current}</span>
      <button type="button" onClick={() => onEdit(entry)}>Editer</button>
      <button type="button" className="danger" onClick={() => onDelete(entry.key)}>Supprimer</button>
    </div>
  );
});

function readNumberEnv(name: string, fallback: number): number {
  const env = import.meta.env as Record<string, string | undefined>;
  const value = Number(env[name]);
  return Number.isFinite(value) && value > 0 ? value : fallback;
}

function formatCount(value: number): string {
  if (value >= 1000 && value % 1000 === 0) {
    return `${value / 1000}k`;
  }
  return value.toLocaleString("fr-FR");
}

function isRangeOperator(operator: FilterOperator): boolean {
  return operator === ">" || operator === ">=" || operator === "<" || operator === "<=";
}
