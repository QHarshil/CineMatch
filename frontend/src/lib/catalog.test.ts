import { describe, expect, it } from "vitest";
import type { SupabaseClient } from "@supabase/supabase-js";
import { becauseYouLiked, browseTitles, catalogGenres, nearestTitles } from "./catalog";
import { arrival } from "@/test/fixtures";
import type { Movie } from "@/types/movie";

type Call = [method: string, ...args: unknown[]];

/**
 * A Supabase client whose queries record their calls and resolve to whatever
 * `answer` returns for them, so tests can assert on the query and its result.
 */
function fakeClient(answer: (table: string, calls: Call[]) => unknown) {
  const queries: { table: string; calls: Call[] }[] = [];
  const query = (table: string) => {
    const calls: Call[] = [];
    queries.push({ table, calls });
    const builder: object = new Proxy(
      {},
      {
        get(_target, method: string) {
          if (method === "then") {
            return (resolve: (value: unknown) => unknown) =>
              Promise.resolve({ data: answer(table, calls) }).then(resolve);
          }
          return (...args: unknown[]) => {
            calls.push([method, ...args]);
            return builder;
          };
        },
      },
    );
    return builder;
  };
  const rpc = (fn: string, args: unknown) => {
    const builder = query(`rpc:${fn}`);
    queries[queries.length - 1].calls.push(["args", args]);
    return builder;
  };
  return { db: { from: query, rpc } as unknown as SupabaseClient, queries };
}

const movie = (id: string, genres: string[], vote = 7): Movie => ({
  ...arrival,
  id,
  title: id,
  genres,
  vote_average: vote,
});
const has = (calls: Call[], method: string, ...args: unknown[]) =>
  calls.some(([m, ...a]) => m === method && JSON.stringify(a) === JSON.stringify(args));

describe("catalog", () => {
  it("lists each genre once, sorted", async () => {
    const { db } = fakeClient(() => [{ genres: ["Drama", "Crime"] }, { genres: ["Crime"] }, { genres: null }]);
    expect(await catalogGenres(db)).toEqual(["Crime", "Drama"]);
  });

  it("filters browse pages by genre only when one is chosen", async () => {
    const { db, queries } = fakeClient(() => []);
    await browseTitles(db, { sort: "newest", offset: 30, limit: 30 });
    await browseTitles(db, { genre: "Horror", sort: "a_z", offset: 0, limit: 30 });
    expect(has(queries[0].calls, "order", "release_year", { ascending: false })).toBe(true);
    expect(has(queries[0].calls, "range", 30, 59)).toBe(true);
    expect(queries[0].calls.some(([m]) => m === "contains")).toBe(false);
    expect(has(queries[1].calls, "contains", "genres", ["Horror"])).toBe(true);
  });

  it("leaves the seed out of its nearest titles", async () => {
    const seed = { title: "Arrival", embedding: "[0.1]" };
    const { db } = fakeClient((table) =>
      table === "movies" ? seed : [movie("seed", ["Drama"]), movie("a", ["Drama"]), movie("b", ["Drama"])],
    );
    const { seedTitle, neighbors } = await nearestTitles(db, "seed", 2);
    expect(seedTitle).toBe("Arrival");
    expect(neighbors.map((n) => n.id)).toEqual(["a", "b"]);
  });

  it("shows each title once across because-you-liked rows", async () => {
    const liked = [movie("liked-1", ["Drama", "Crime"], 8), movie("liked-2", ["Crime"], 7)];
    const { db } = fakeClient((table, calls) => {
      if (table === "interactions") return [{ movie_id: "liked-1" }, { movie_id: "liked-2" }];
      if (calls.some(([m]) => m === "in")) return liked;
      // Every genre query returns an overlapping set, including a liked title.
      return [movie("liked-2", ["Crime"]), movie("shared", ["Crime"]), movie(`only-${calls.length}`, ["Crime"])];
    });
    const sections = await becauseYouLiked(db, "user-1");
    const shown = sections.flatMap((s) => s.similarMovies.map((m) => m.id));
    expect(new Set(shown).size).toBe(shown.length);
    expect(shown).not.toContain("liked-2");
    expect(sections[0].likedMovie.id).toBe("liked-1");
  });

  it("returns no rows for a user with no likes", async () => {
    const { db, queries } = fakeClient(() => []);
    expect(await becauseYouLiked(db, "user-1")).toEqual([]);
    expect(queries).toHaveLength(1);
  });
});
