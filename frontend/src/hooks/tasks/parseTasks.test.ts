import { describe, it, expect } from "vitest";
import { parseTasks } from "./parseTasks";

describe("parseTasks", () => {
  it("returns tasks array when payload has task entity", () => {
    const input = '{"entityType":"task","data":[{"id":"1","title":"a","notes":"","category":"normal","order":0,"done":false}]}';
    const result = parseTasks(input);
    expect(result).toHaveLength(1);
    expect(result[0]).toMatchObject({ id: "1" });
  });

  it("returns empty array for invalid payloads", () => {
    expect(parseTasks("null")).toEqual([]);
    expect(parseTasks("{}" as any)).toEqual([]);
    expect(parseTasks('{"entityType":"user-settings","data":{}}')).toEqual([]);
    expect(parseTasks("not-json")).toEqual([]);
  });
});

