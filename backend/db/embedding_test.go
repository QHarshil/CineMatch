package db

import (
	"math"
	"testing"
)

func TestVectorToStringRoundTrip(t *testing.T) {
	vec := []float32{0.5, -0.25, 1.0, 0.0, 0.123456}
	got, err := parseVectorString(vectorToString(vec))
	if err != nil {
		t.Fatalf("round trip parse failed: %v", err)
	}
	if len(got) != len(vec) {
		t.Fatalf("length mismatch: got %d want %d", len(got), len(vec))
	}
	for i := range vec {
		if math.Abs(float64(got[i]-vec[i])) > 1e-6 {
			t.Errorf("element %d: got %v want %v", i, got[i], vec[i])
		}
	}
}

func TestRecencyWeightedMeanEmpty(t *testing.T) {
	if v := recencyWeightedMean(nil, map[string][]float32{}); v != nil {
		t.Errorf("no movies should yield nil, got %v", v)
	}
	if v := recencyWeightedMean([]string{"a"}, map[string][]float32{}); v != nil {
		t.Errorf("unresolved embeddings should yield nil, got %v", v)
	}
}

func TestRecencyWeightedMeanIsUnitLength(t *testing.T) {
	emb := map[string][]float32{"a": {3, 0, 0}, "b": {0, 4, 0}}
	got := recencyWeightedMean([]string{"a", "b"}, emb)

	var norm float64
	for _, v := range got {
		norm += float64(v) * float64(v)
	}
	if math.Abs(math.Sqrt(norm)-1.0) > 1e-5 {
		t.Errorf("expected unit-length vector, got norm %v", math.Sqrt(norm))
	}
}

func TestRecencyWeightedMeanFavorsRecent(t *testing.T) {
	// "a" is newest (full weight); "b" is older (decayed). The blended vector
	// should lean toward a's axis.
	emb := map[string][]float32{"a": {1, 0}, "b": {0, 1}}
	got := recencyWeightedMean([]string{"a", "b"}, emb)
	if got[0] <= got[1] {
		t.Errorf("expected the most recent movie to dominate, got %v", got)
	}
}

func TestRecencyWeightedMeanSkipsMissing(t *testing.T) {
	// A movie with no stored embedding is skipped rather than zero-filled.
	emb := map[string][]float32{"a": {1, 0}}
	got := recencyWeightedMean([]string{"missing", "a"}, emb)
	if got == nil || math.Abs(float64(got[0])-1.0) > 1e-5 || math.Abs(float64(got[1])) > 1e-5 {
		t.Errorf("expected normalized [1,0] from the single valid movie, got %v", got)
	}
}
