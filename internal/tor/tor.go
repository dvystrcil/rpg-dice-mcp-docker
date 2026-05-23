// Package tor encodes The One Ring 2e check resolution rules on top
// of package dice. The split is deliberate: dice provides the RNG and
// uniformity guarantees; tor provides the game-specific interpretation.
//
// Mechanics (TOR 2e, Free League — see test file for the full table):
//
//   - Feat die: d12 with two special faces.
//     - 11 → Eye of Sauron (counts as 0 in the total, may cost Hope
//       on a Miserable check).
//     - 12 → Gandalf rune (automatic success regardless of TN).
//
//   - Success dice: d6, count = skill or attribute rating. Numeric
//     value contributes to the total.
//
//   - Weariness: success die rolls of 1, 2, 3 are treated as 0
//     (don't contribute). Reflected in EffectiveSuccessDice.
//
//   - Miserable: when an Eye fires on a Miserable check, MiserableEye
//     is set; caller decrements Hope per the Core Rules.
package tor

import (
	"fmt"
	"math/rand"

	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/dice"
)

// CheckInput is the API surface for a TOR check.
type CheckInput struct {
	// SkillRating is the rating used to determine number of Success dice.
	// Must be >= 0. Skill 0 produces a Feat-die-only check.
	SkillRating int

	// TargetNumber is the difficulty to beat (>= TN succeeds).
	TargetNumber int

	// Weariness: when true, Success die rolls of 1-3 are treated as 0.
	Weariness bool

	// Miserable: when true, an Eye result sets MiserableEye in the result
	// for the caller's bookkeeping (Hope decrement, etc.).
	Miserable bool
}

// CheckResult is the outcome of resolving a CheckInput.
type CheckResult struct {
	Input CheckInput // echo of the input

	FeatDie     int // raw Feat die value, 1-12
	GandalfRune bool // FeatDie == 12
	EyeOfSauron bool // FeatDie == 11

	SuccessDice          []int // raw d6 values, length = SkillRating
	EffectiveSuccessDice []int // weariness-adjusted (1-3 → 0 if Weary)

	// Total: FeatDie contribution + sum(EffectiveSuccessDice).
	// FeatDie contribution: numeric value (1-10), 0 (Eye), or 0 (Gandalf
	// — the Gandalf rune contributes 0 to the total but auto-succeeds).
	Total int

	// Succeeds: true if Total >= TN, OR GandalfRune (automatic).
	Succeeds bool

	// Margin: Total - TN. Negative on miss. Reported even on Gandalf
	// (where it's the post-hoc margin) and Eye (post-hoc).
	Margin int

	// MiserableEye: true iff Input.Miserable AND EyeOfSauron. Caller
	// handles Hope/Shadow consequences.
	MiserableEye bool
}

// ResolveCheckWith rolls a TOR check using the provided *rand.Rand. The
// production entry point ResolveCheck uses dice.NewRand(); this signature
// allows tests to inject seeded sources.
func ResolveCheckWith(in CheckInput, r *rand.Rand) CheckResult {
	if in.SkillRating < 0 {
		panic(fmt.Sprintf("tor: SkillRating must be >= 0, got %d", in.SkillRating))
	}

	// Feat die: 1d12
	feat := r.Intn(12) + 1

	// Success dice: SkillRating × d6
	successes := make([]int, in.SkillRating)
	for i := range successes {
		successes[i] = r.Intn(6) + 1
	}

	return interpret(in, feat, successes)
}

// ResolveCheck is the production entry point. Uses crypto/rand-seeded
// dice.NewRand() under the hood.
func ResolveCheck(in CheckInput) CheckResult {
	return ResolveCheckWith(in, dice.NewRand())
}

// interpret is the pure rules-application function — no RNG. Given a
// feat die value and a slice of success dice values, computes the
// CheckResult. Exposed for testing (so we can assert rules behavior
// against synthetic inputs without depending on RNG output).
func interpret(in CheckInput, feat int, successes []int) CheckResult {
	if feat < 1 || feat > 12 {
		panic(fmt.Sprintf("tor: feat die out of range [1,12]: %d", feat))
	}

	gandalf := feat == 12
	eye := feat == 11

	// Effective Success dice (weariness-adjusted).
	effective := make([]int, len(successes))
	for i, v := range successes {
		if in.Weariness && v >= 1 && v <= 3 {
			effective[i] = 0
		} else {
			effective[i] = v
		}
	}

	// Feat contribution to Total: numeric for 1-10, 0 for Eye, 0 for
	// Gandalf (Gandalf auto-succeeds, contribution doesn't matter).
	featContribution := feat
	if eye || gandalf {
		featContribution = 0
	}

	sumSuccess := 0
	for _, v := range effective {
		sumSuccess += v
	}
	total := featContribution + sumSuccess

	succeeds := gandalf || total >= in.TargetNumber
	margin := total - in.TargetNumber

	return CheckResult{
		Input:                in,
		FeatDie:              feat,
		GandalfRune:          gandalf,
		EyeOfSauron:          eye,
		SuccessDice:          successes,
		EffectiveSuccessDice: effective,
		Total:                total,
		Succeeds:             succeeds,
		Margin:               margin,
		MiserableEye:         eye && in.Miserable,
	}
}

// ---- Combat sister tool ----

// CombatInput is a thin wrapper over CheckInput for combat-shape calls.
// In TOR Strider Mode the player rolls almost all combat checks; this
// is for the times the Loremaster does roll for an NPC's action.
type CombatInput struct {
	AttackerSkill int  // Success dice count
	DefenderTN    int  // The target's effective TN (Parry or equivalent)
	Weariness     bool
	Miserable     bool
}

// ResolveCombatWith mirrors ResolveCheckWith but with combat-flavored
// field names. Same resolution rules.
func ResolveCombatWith(in CombatInput, r *rand.Rand) CheckResult {
	return ResolveCheckWith(CheckInput{
		SkillRating:  in.AttackerSkill,
		TargetNumber: in.DefenderTN,
		Weariness:    in.Weariness,
		Miserable:    in.Miserable,
	}, r)
}

// ResolveCombat is the production entry point.
func ResolveCombat(in CombatInput) CheckResult {
	return ResolveCombatWith(in, dice.NewRand())
}
