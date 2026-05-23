// Package dice tests — written FIRST per the TDD discipline in
// dvystrcil/homelab#201 AC2.
//
// Two layers:
//
//   - Hermetic unit tests for spec parsing + deterministic rolls
//     (seed-controlled), using `rand.NewSource(int64)` so the test
//     output is reproducible.
//
//   - A statistical χ² uniformity test that asserts the production
//     RNG (crypto-seeded math/rand) produces uniform distributions
//     across the common die sizes. Catches biased-RNG regressions
//     before they ship — the WHOLE POINT of this MCP is to be a
//     BETTER RNG than the LLM.
package dice

import (
	"math"
	"math/rand"
	"strings"
	"testing"
)

// ---- spec parsing ----

func TestParseSpec_SimpleNd(t *testing.T) {
	cases := []struct {
		in        string
		wantN     int
		wantSides int
		wantMod   int
	}{
		{"1d6", 1, 6, 0},
		{"2d6", 2, 6, 0},
		{"3d8", 3, 8, 0},
		{"1d12", 1, 12, 0},
		{"4d20", 4, 20, 0},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			s, err := ParseSpec(tc.in)
			if err != nil {
				t.Fatalf("ParseSpec(%q) error: %v", tc.in, err)
			}
			if s.N != tc.wantN || s.Sides != tc.wantSides || s.Modifier != tc.wantMod {
				t.Errorf("ParseSpec(%q) = {%d, %d, %d}; want {%d, %d, %d}",
					tc.in, s.N, s.Sides, s.Modifier, tc.wantN, tc.wantSides, tc.wantMod)
			}
		})
	}
}

func TestParseSpec_ImpliedSingleDie(t *testing.T) {
	// "d20" should mean "1d20"
	s, err := ParseSpec("d20")
	if err != nil {
		t.Fatalf("ParseSpec(\"d20\") error: %v", err)
	}
	if s.N != 1 || s.Sides != 20 {
		t.Errorf("ParseSpec(\"d20\") = {N=%d, Sides=%d}; want {N=1, Sides=20}", s.N, s.Sides)
	}
}

func TestParseSpec_WithModifier(t *testing.T) {
	cases := []struct {
		in      string
		wantMod int
	}{
		{"2d6+1", 1},
		{"1d20+5", 5},
		{"3d8-2", -2},
		{"1d12+0", 0},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			s, err := ParseSpec(tc.in)
			if err != nil {
				t.Fatalf("ParseSpec(%q) error: %v", tc.in, err)
			}
			if s.Modifier != tc.wantMod {
				t.Errorf("ParseSpec(%q).Modifier = %d; want %d", tc.in, s.Modifier, tc.wantMod)
			}
		})
	}
}

func TestParseSpec_InvalidRejected(t *testing.T) {
	bad := []string{"", "garbage", "1d", "d", "0d6", "2d0", "-1d6", "2d-6", "2 d 6", "2x6"}
	for _, b := range bad {
		t.Run(b, func(t *testing.T) {
			if _, err := ParseSpec(b); err == nil {
				t.Errorf("ParseSpec(%q) expected error; got nil", b)
			}
		})
	}
}

// ---- deterministic rolls (seed-controlled) ----

func TestRollWithSeed_Reproducible(t *testing.T) {
	spec, _ := ParseSpec("2d6+1")
	r := rand.New(rand.NewSource(42))
	got1 := RollWith(spec, r)
	// Same seed, fresh RNG, must produce the same result
	r2 := rand.New(rand.NewSource(42))
	got2 := RollWith(spec, r2)
	if got1.Total != got2.Total {
		t.Errorf("RollWith(seed=42) not reproducible: got %d vs %d", got1.Total, got2.Total)
	}
	if len(got1.Rolls) != 2 {
		t.Errorf("RollWith 2d6 returned %d rolls; want 2", len(got1.Rolls))
	}
	for _, r := range got1.Rolls {
		if r < 1 || r > 6 {
			t.Errorf("RollWith 2d6 roll out of range: %d", r)
		}
	}
}

// ---- statistical uniformity (the WHOLE POINT — catch biased RNG) ----

// TestRollUniform_d6 asserts that 10000 rolls of 1d6 are uniformly
// distributed across 1-6 (χ² test at p > 0.05).
// If this fails, the RNG is biased and the MCP is no better than the LLM.
func TestRollUniform_d6(t *testing.T) {
	chiSquaredUniformity(t, 6, 10000)
}

func TestRollUniform_d12(t *testing.T) {
	chiSquaredUniformity(t, 12, 10000)
}

func TestRollUniform_d20(t *testing.T) {
	chiSquaredUniformity(t, 20, 10000)
}

// chiSquaredUniformity rolls `trials` of 1d<sides> and asserts the
// distribution is uniform via a χ² goodness-of-fit test. Uses the
// production NewRand() (NOT a fixed seed) so this catches bias in
// the production RNG seeding path.
func chiSquaredUniformity(t *testing.T, sides, trials int) {
	t.Helper()
	r := NewRand()
	counts := make([]int, sides+1) // 1-indexed
	spec, _ := ParseSpec("1d" + itoa(sides))
	for i := 0; i < trials; i++ {
		got := RollWith(spec, r)
		v := got.Rolls[0]
		counts[v]++
	}
	expected := float64(trials) / float64(sides)
	chiSq := 0.0
	for i := 1; i <= sides; i++ {
		diff := float64(counts[i]) - expected
		chiSq += diff * diff / expected
	}
	// Critical χ² value for df = sides-1 at α = 0.001 (very lenient
	// to avoid flakes — we want to catch real bias, not noise).
	// Computed offline; values for common df:
	//   df=5  (d6):  α=0.001 → 20.515
	//   df=11 (d12): α=0.001 → 31.264
	//   df=19 (d20): α=0.001 → 43.820
	var critical float64
	switch sides {
	case 6:
		critical = 20.515
	case 12:
		critical = 31.264
	case 20:
		critical = 43.820
	default:
		t.Fatalf("no critical value for d%d", sides)
	}
	if chiSq > critical {
		t.Errorf("d%d non-uniform: χ²=%.3f > critical %.3f at α=0.001 (counts: %v)",
			sides, chiSq, critical, counts[1:])
	}
	if math.IsNaN(chiSq) || math.IsInf(chiSq, 0) {
		t.Fatalf("d%d χ² is NaN/Inf: %f", sides, chiSq)
	}
}

// itoa minimal local helper to avoid importing strconv in hot path
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var s []byte
	for n > 0 {
		s = append([]byte{byte('0' + n%10)}, s...)
		n /= 10
	}
	if neg {
		s = append([]byte{'-'}, s...)
	}
	return string(s)
}

// ---- modifier application ----

func TestRoll_AppliesModifier(t *testing.T) {
	spec, _ := ParseSpec("2d6+3")
	r := rand.New(rand.NewSource(1))
	got := RollWith(spec, r)
	sum := 0
	for _, v := range got.Rolls {
		sum += v
	}
	if got.Total != sum+3 {
		t.Errorf("Total %d != sum(%d) + modifier(3)", got.Total, sum)
	}
	if got.Modifier != 3 {
		t.Errorf("Result.Modifier = %d; want 3", got.Modifier)
	}
}

// ---- error display ----

func TestParseSpec_ErrorMentionsSpec(t *testing.T) {
	_, err := ParseSpec("not-a-spec")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "not-a-spec") {
		t.Errorf("error %q should include the bad spec for diagnosability", err.Error())
	}
}
