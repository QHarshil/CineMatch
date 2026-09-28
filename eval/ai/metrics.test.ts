import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { mean, ndcgAt, percentile, precisionAt, reciprocalRank } from "./metrics.ts";

const relevant = new Set(["a", "c"]);

describe("ranking metrics", () => {
  it("reciprocal rank uses the first relevant position", () => {
    assert.equal(reciprocalRank(["x", "c", "a"], relevant), 0.5);
    assert.equal(reciprocalRank(["x", "y"], relevant), 0);
    assert.equal(reciprocalRank(["x", "y", "a"], relevant, 2), 0);
  });

  it("precision counts missing slots as misses", () => {
    assert.equal(precisionAt(["a", "b", "c"], relevant, 4), 0.5);
    assert.equal(precisionAt(["a"], relevant, 10), 0.1);
  });

  it("nDCG is 1 for an ideal ranking and lower when relevant items sink", () => {
    assert.equal(ndcgAt(["a", "c", "x"], relevant, 3), 1);
    const sunk = ndcgAt(["x", "y", "a"], relevant, 3);
    assert.ok(sunk > 0 && sunk < 0.5);
    assert.equal(ndcgAt(["x"], new Set(), 3), 0);
  });
});

describe("summary statistics", () => {
  it("nearest-rank percentiles", () => {
    const values = [5, 1, 4, 2, 3];
    assert.equal(percentile(values, 50), 3);
    assert.equal(percentile(values, 95), 5);
    assert.equal(percentile([], 50), 0);
  });

  it("mean of an empty list is 0", () => {
    assert.equal(mean([]), 0);
    assert.equal(mean([1, 2, 3]), 2);
  });
});
