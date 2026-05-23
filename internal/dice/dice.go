// Package dice provides generic dice-expression parsing (NdM±K),
// rolling, and uniformity guarantees. The TOR-specific rules live
// in package tor, which composes on top of this.
//
// Design contract:
//
//   - Production RNG seeded from crypto/rand (NewRand()). Never use
//     time.Now() seeding — predictable and biased.
//
//   - All rolling goes through RollWith(spec, *rand.Rand), so tests
//     can inject a deterministic rand.New(rand.NewSource(seed)) and
//     production calls inject NewRand(). One code path, two seed
//     sources.
//
//   - χ² uniformity is tested for d6 / d12 / d20 in dice_test.go.
//     The whole point of this MCP is to be a BETTER source of
//     random than the LLM; that invariant is enforced via test.
package dice

import (
	cryptoRand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
)

// Spec is a parsed dice expression: NdM±K (e.g. "2d6+1").
type Spec struct {
	N        int // number of dice
	Sides    int // sides per die
	Modifier int // flat additive (signed); 0 if omitted
}

// Result is the outcome of rolling a Spec.
type Result struct {
	Spec     Spec  // echo of input
	Rolls    []int // individual dice values (length N, each in [1, Sides])
	Modifier int   // echo of Spec.Modifier (convenience)
	Total    int   // sum(Rolls) + Modifier
}

// specRe matches "[N]dM[±K]" with N defaulting to 1 if absent.
// Captures:
//
//	1: N (optional)
//	2: M (required)
//	3: ±K (optional, with sign)
var specRe = regexp.MustCompile(`^([0-9]+)?d([0-9]+)([+-][0-9]+)?$`)

// ParseSpec parses a dice expression. Returns an error whose message
// includes the offending input for diagnosability.
func ParseSpec(s string) (Spec, error) {
	if s == "" {
		return Spec{}, fmt.Errorf("dice: empty spec")
	}
	m := specRe.FindStringSubmatch(s)
	if m == nil {
		return Spec{}, fmt.Errorf("dice: invalid spec %q (want NdM±K, e.g. 2d6+1)", s)
	}

	// N (default 1 if omitted)
	n := 1
	if m[1] != "" {
		v, err := strconv.Atoi(m[1])
		if err != nil {
			return Spec{}, fmt.Errorf("dice: invalid N in spec %q: %w", s, err)
		}
		n = v
	}
	if n <= 0 {
		return Spec{}, fmt.Errorf("dice: N must be > 0 in spec %q", s)
	}

	// Sides
	sides, err := strconv.Atoi(m[2])
	if err != nil {
		return Spec{}, fmt.Errorf("dice: invalid sides in spec %q: %w", s, err)
	}
	if sides <= 0 {
		return Spec{}, fmt.Errorf("dice: sides must be > 0 in spec %q", s)
	}

	// Modifier (optional)
	mod := 0
	if m[3] != "" {
		v, err := strconv.Atoi(m[3])
		if err != nil {
			return Spec{}, fmt.Errorf("dice: invalid modifier in spec %q: %w", s, err)
		}
		mod = v
	}

	return Spec{N: n, Sides: sides, Modifier: mod}, nil
}

// RollWith rolls the given spec using the provided *rand.Rand source.
// Production calls pass NewRand(); tests pass rand.New(rand.NewSource(seed))
// for reproducibility.
func RollWith(spec Spec, r *rand.Rand) Result {
	rolls := make([]int, spec.N)
	sum := 0
	for i := 0; i < spec.N; i++ {
		v := r.Intn(spec.Sides) + 1 // [1, Sides]
		rolls[i] = v
		sum += v
	}
	return Result{
		Spec:     spec,
		Rolls:    rolls,
		Modifier: spec.Modifier,
		Total:    sum + spec.Modifier,
	}
}

// Roll is the convenience entry point for production callers — uses
// NewRand() under the hood. For deterministic / tested behavior, use
// RollWith and supply your own seeded Rand.
func Roll(spec Spec) Result {
	return RollWith(spec, NewRand())
}

// NewRand returns a *rand.Rand seeded from crypto/rand.Reader. Production
// rolls use this so the RNG is unpredictable across process restarts
// (vs time.Now() seeding which leaks startup time into the output).
//
// On the (vanishingly rare) chance crypto/rand fails, we fall back to
// the default rand source rather than panicking — better to return a
// noisier roll than to refuse to roll at all.
func NewRand() *rand.Rand {
	var seedBytes [8]byte
	_, err := cryptoRand.Read(seedBytes[:])
	if err != nil {
		// Fallback to default source — degraded but functional.
		return rand.New(rand.NewSource(rand.Int63()))
	}
	seed := int64(binary.LittleEndian.Uint64(seedBytes[:]))
	return rand.New(rand.NewSource(seed))
}
