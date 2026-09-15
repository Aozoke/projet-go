import { describe, expect, it } from "vitest";
import { deserializeValue, serializeValue } from "./wasmRedis";

describe("SDK value codec", () => {
  it.each([
    ["hello world", "hello world"],
    [25, 25],
    [true, true],
    [{ name: "matt" }, { name: "matt" }],
  ])("conserve le type de %j", (value, expected) => {
    expect(deserializeValue(serializeValue(value))).toEqual(expected);
  });

  it("echappe les guillemets sans perdre le texte", () => {
    const value = 'il dit "bonjour"';
    expect(deserializeValue(serializeValue(value))).toBe(value);
  });
});
