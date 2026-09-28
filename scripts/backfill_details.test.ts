import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { buildPatch, runtimeFromDetails } from "./backfill_details.ts";

const baseRow = {
  id: "7f1c2a4e-0000-4000-8000-000000000001",
  tmdb_id: 1399,
  media_type: "tv" as const,
  title: "Game of Thrones",
  runtime: null,
  original_language: null,
  backdrop_path: null,
};

describe("runtimeFromDetails", () => {
  const cases: Array<[string, Parameters<typeof runtimeFromDetails>[0], number | null]> = [
    ["movie runtime", { runtime: 148 }, 148],
    ["series episode runtime", { episode_run_time: [58, 60] }, 58],
    ["series falls back to last episode", { episode_run_time: [], last_episode_to_air: { runtime: 45 } }, 45],
    ["zero is treated as unknown", { runtime: 0 }, null],
    ["nothing reported", {}, null],
  ];
  for (const [name, details, expected] of cases) {
    it(name, () => assert.equal(runtimeFromDetails(details), expected));
  }
});

describe("buildPatch", () => {
  it("fills every missing field", () => {
    const patch = buildPatch(baseRow, {
      episode_run_time: [57],
      original_language: "en",
      backdrop_path: "/backdrop.jpg",
    });
    assert.deepEqual(patch, { runtime: 57, original_language: "en", backdrop_path: "/backdrop.jpg" });
  });

  it("never overwrites values already stored", () => {
    const row = { ...baseRow, runtime: 60, original_language: "en", backdrop_path: "/kept.jpg" };
    const patch = buildPatch(row, { runtime: 99, original_language: "ko", backdrop_path: "/new.jpg" });
    assert.deepEqual(patch, {});
  });
});
