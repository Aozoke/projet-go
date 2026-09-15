import { beforeEach, describe, expect, it } from "vitest";
import {
  clearEntries,
  replaceEntries,
  subscribeEntry,
  subscribeEntryKeys,
  upsertEntry,
} from "./entryStore";

describe("entryStore", () => {
  beforeEach(() => clearEntries());

  it("notifie seulement la ligne modifiee", () => {
    replaceEntries([
      { key: "a", value: "1" },
      { key: "b", value: "2" },
    ]);

    let listUpdates = 0;
    let rowAUpdates = 0;
    let rowBUpdates = 0;
    const unsubscribeList = subscribeEntryKeys(() => listUpdates++);
    const unsubscribeA = subscribeEntry("a", () => rowAUpdates++);
    const unsubscribeB = subscribeEntry("b", () => rowBUpdates++);

    upsertEntry({ key: "a", value: "3" });

    expect(rowAUpdates).toBe(1);
    expect(rowBUpdates).toBe(0);
    expect(listUpdates).toBe(0);

    unsubscribeList();
    unsubscribeA();
    unsubscribeB();
  });
});
