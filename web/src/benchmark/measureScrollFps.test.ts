import { describe, expect, it } from "vitest";
import { calculateFps } from "./measureScrollFps";

describe("calculateFps", () => {
  it("calcule environ 60 images par seconde", () => {
    expect(calculateFps([0, 16.67, 33.34, 50.01])).toBeCloseTo(60, 0);
  });
});
