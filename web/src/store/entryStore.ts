import { useCallback, useSyncExternalStore } from "react";

export type StoreEntry = {
  key: string;
  value: string;
};

type Listener = () => void;

let entries = new Map<string, StoreEntry>();
let keys: string[] = [];
const listListeners = new Set<Listener>();
const rowListeners = new Map<string, Set<Listener>>();

export function replaceEntries(nextEntries: StoreEntry[]): void {
  const previousEntries = entries;
  const nextEntriesByKey = new Map<string, StoreEntry>();
  const nextKeys = new Array<string>(nextEntries.length);

  nextEntries.forEach((entry, index) => {
    nextEntriesByKey.set(entry.key, entry);
    nextKeys[index] = entry.key;
  });

  entries = nextEntriesByKey;
  keys = nextKeys.sort();

  listListeners.forEach((listener) => listener());
  rowListeners.forEach((_, key) => {
    const previous = previousEntries.get(key);
    const next = entries.get(key);
    if (previous?.value !== next?.value) {
      notifyRow(key);
    }
  });
}

export function upsertEntry(entry: StoreEntry): void {
  const isNew = !entries.has(entry.key);
  entries.set(entry.key, entry);
  notifyRow(entry.key);

  if (isNew) {
    keys = [...keys, entry.key].sort((left, right) => left.localeCompare(right));
    listListeners.forEach((listener) => listener());
  }
}

export function removeEntry(key: string): void {
  if (!entries.delete(key)) {
    return;
  }

  keys = keys.filter((currentKey) => currentKey !== key);
  notifyRow(key);
  listListeners.forEach((listener) => listener());
}

export function clearEntries(): void {
  replaceEntries([]);
}

export function useEntryKeys(): string[] {
  return useSyncExternalStore(subscribeEntryKeys, getEntryKeys, getEntryKeys);
}

export function useEntry(key: string): StoreEntry | undefined {
  const subscribe = useCallback((listener: Listener) => subscribeEntry(key, listener), [key]);
  const getSnapshot = useCallback(() => getEntry(key), [key]);
  return useSyncExternalStore(subscribe, getSnapshot, getSnapshot);
}

export function subscribeEntryKeys(listener: Listener): () => void {
  listListeners.add(listener);
  return () => listListeners.delete(listener);
}

export function getEntryKeys(): string[] {
  return keys;
}

export function getEntry(key: string): StoreEntry | undefined {
  return entries.get(key);
}

export function subscribeEntry(key: string, listener: Listener): () => void {
  const listeners = rowListeners.get(key) ?? new Set<Listener>();
  listeners.add(listener);
  rowListeners.set(key, listeners);

  return () => {
    listeners.delete(listener);
    if (listeners.size === 0) {
      rowListeners.delete(key);
    }
  };
}

function notifyRow(key: string): void {
  rowListeners.get(key)?.forEach((listener) => listener());
}
