import { describe, expect, it } from "vitest";
import { getVirtualRange } from "./getVirtualRange";

describe("getVirtualRange", () => {
  it.each([0, 2_100_000, 4_199_480])(
    "garde peu de lignes montees avec 100 000 entrees, scrollTop=%s",
    (scrollTop) => {
      const range = getVirtualRange({
        itemCount: 100_000,
        rowHeight: 42,
        viewportHeight: 520,
        scrollTop,
        overscan: 8,
      });

      expect(range.endIndex - range.startIndex).toBeLessThanOrEqual(29);
      expect(range.totalHeight).toBe(4_200_000);
      expect(range.startIndex).toBeGreaterThanOrEqual(0);
      expect(range.endIndex).toBeLessThanOrEqual(100_000);
    },
  );
});
