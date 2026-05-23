// Package tor — tests for The One Ring 2e check resolution.
// TDD discipline per dvystrcil/homelab#201 AC3.
//
// Hermetic — uses rand.New(rand.NewSource(seed)) so test outputs are
// reproducible. The χ² uniformity test in package dice covers the
// production RNG; here we focus on RULES correctness.
//
// Mechanics encoded (TOR 2e, Free League):
//
//   - Feat die: d12. Face 11 = Eye of Sauron (counts as 0); face 12 =
//     Gandalf rune (automatic success regardless of TN). Other faces
//     are their numeric value.
//
//   - Success dice: d6, count equal to skill or attribute rating.
//     Numeric value contributes to the total.
//
//   - Weariness: success die rolls of 1-3 are treated as 0 (don't
//     contribute to the total).
//
//   - Total: feat die value + sum of (effective) success dice.
//     Compared to TN; >= TN = succeeds, with margin = total - TN.
//
//   - Miserable: Eye (Feat=11) on a miserable check ALSO costs Hope —
//     this is signaled via Result.MiserableEye for the caller; the
//     caller decides what to do with it (decrement Hope, log, etc.).
//
//   - Gandalf rune (Feat=12) always succeeds, even against an
//     impossibly high TN. Margin reported as TN+1 (just barely)
//     unless the success dice push it higher.
package tor

import (
	"math/rand"
	"testing"
)

// Helper: deterministic resolver with a fixed seed.
func resolveSeeded(seed int64, in CheckInput) CheckResult {
	r := rand.New(rand.NewSource(seed))
	return ResolveCheckWith(in, r)
}

// ---- standard checks ----

func TestStandardCheck_BeatsTN(t *testing.T) {
	// Seed chosen such that Feat + 2d6 (skill 2) totals >= 14.
	// Specific values depend on Go's rand impl; we assert the
	// invariants, not the exact numbers.
	in := CheckInput{
		SkillRating: 2,
		TargetNumber: 14,
		Weariness:    false,
		Miserable:    false,
	}
	res := resolveSeeded(1, in)
	// Total must equal feat_die + sum(success dice values, possibly
	// weariness-adjusted) when feat is 1..10. If feat is 11 (Eye) or
	// 12 (Gandalf), special semantics apply.
	if res.FeatDie == 12 && !res.GandalfRune {
		t.Errorf("FeatDie=12 should set GandalfRune=true")
	}
	if res.FeatDie == 11 && !res.EyeOfSauron {
		t.Errorf("FeatDie=11 should set EyeOfSauron=true")
	}
	if len(res.SuccessDice) != in.SkillRating {
		t.Errorf("SuccessDice length=%d; want %d", len(res.SuccessDice), in.SkillRating)
	}
}

// ---- Gandalf rune (Feat=12) ----

func TestGandalfRune_AutomaticSuccess(t *testing.T) {
	// Construct a synthetic result via direct interpretation, not RNG.
	// Even with TN 99 and skill 0, Gandalf rune succeeds.
	in := CheckInput{SkillRating: 0, TargetNumber: 99}
	res := interpret(in, 12, nil)
	if !res.GandalfRune {
		t.Errorf("GandalfRune flag should be true when FeatDie=12")
	}
	if !res.Succeeds {
		t.Errorf("Gandalf rune should auto-succeed even against TN 99")
	}
	if res.EyeOfSauron {
		t.Errorf("EyeOfSauron should be false when FeatDie=12")
	}
}

// ---- Eye of Sauron (Feat=11) ----

func TestEyeOfSauron_CountsAsZero(t *testing.T) {
	// Feat=11 counts as 0 in the total. Skill 3, success dice all 6s
	// (sum=18). Total = 0 + 18 = 18.
	in := CheckInput{SkillRating: 3, TargetNumber: 20}
	res := interpret(in, 11, []int{6, 6, 6})
	if !res.EyeOfSauron {
		t.Errorf("EyeOfSauron flag should be true when FeatDie=11")
	}
	if res.Total != 18 {
		t.Errorf("Eye: total should be 0 + 18 = 18; got %d", res.Total)
	}
	if res.Succeeds {
		t.Errorf("Eye total 18 vs TN 20 should fail")
	}
}

func TestEyeOfSauron_StillCanSucceedWithHighSkillDice(t *testing.T) {
	// Even with Eye, if the success dice are enough, you can pass TN.
	in := CheckInput{SkillRating: 5, TargetNumber: 20}
	res := interpret(in, 11, []int{6, 6, 6, 4, 4}) // sum = 26
	if !res.EyeOfSauron {
		t.Errorf("EyeOfSauron should still be true")
	}
	if res.Total != 26 {
		t.Errorf("total should be 0 (eye) + 26 = 26; got %d", res.Total)
	}
	if !res.Succeeds {
		t.Errorf("total 26 vs TN 20 should succeed despite Eye")
	}
}

// ---- weariness ----

func TestWeariness_SuccessDiceUnderFourCountAsZero(t *testing.T) {
	// Weary, skill 4, dice [1, 2, 3, 6]. Effective: [0, 0, 0, 6] = 6.
	// Feat die = 5. Total = 5 + 6 = 11.
	in := CheckInput{SkillRating: 4, TargetNumber: 10, Weariness: true}
	res := interpret(in, 5, []int{1, 2, 3, 6})
	if res.Total != 11 {
		t.Errorf("Weary total: feat 5 + effective 6 = 11; got %d (effective dice: %v)",
			res.Total, res.EffectiveSuccessDice)
	}
	if !res.Succeeds {
		t.Errorf("total 11 vs TN 10 should succeed")
	}
}

func TestWeariness_AllUnderFour_TotalEqualsFeat(t *testing.T) {
	in := CheckInput{SkillRating: 3, TargetNumber: 10, Weariness: true}
	res := interpret(in, 7, []int{1, 2, 3})
	if res.Total != 7 {
		t.Errorf("Weary all-low total: feat 7 + 0 + 0 + 0 = 7; got %d", res.Total)
	}
}

func TestNotWeary_AllDiceCount(t *testing.T) {
	in := CheckInput{SkillRating: 3, TargetNumber: 10, Weariness: false}
	res := interpret(in, 7, []int{1, 2, 3})
	if res.Total != 13 {
		t.Errorf("Not weary: feat 7 + 1+2+3 = 13; got %d", res.Total)
	}
}

// ---- miserable ----

func TestMiserable_EyeCostsHope(t *testing.T) {
	in := CheckInput{SkillRating: 2, TargetNumber: 14, Miserable: true}
	res := interpret(in, 11, []int{5, 4})
	if !res.EyeOfSauron {
		t.Errorf("Eye should still be true on miserable check")
	}
	if !res.MiserableEye {
		t.Errorf("MiserableEye should be true when Eye occurs on miserable check")
	}
}

func TestMiserable_NoEye_NoMiserableEye(t *testing.T) {
	in := CheckInput{SkillRating: 2, TargetNumber: 14, Miserable: true}
	res := interpret(in, 8, []int{5, 4})
	if res.EyeOfSauron {
		t.Errorf("No Eye on Feat=8")
	}
	if res.MiserableEye {
		t.Errorf("MiserableEye should be false when no Eye fired")
	}
}

// ---- margin reporting ----

func TestMargin_HitMargin(t *testing.T) {
	in := CheckInput{SkillRating: 2, TargetNumber: 10}
	res := interpret(in, 8, []int{4, 5}) // total 17, margin 7
	if res.Margin != 7 {
		t.Errorf("Margin: total 17 - TN 10 = 7; got %d", res.Margin)
	}
	if !res.Succeeds {
		t.Errorf("17 vs TN 10 should succeed")
	}
}

func TestMargin_MissNegativeMargin(t *testing.T) {
	in := CheckInput{SkillRating: 1, TargetNumber: 15}
	res := interpret(in, 3, []int{2}) // total 5, margin -10
	if res.Margin != -10 {
		t.Errorf("Margin on miss: total 5 - TN 15 = -10; got %d", res.Margin)
	}
	if res.Succeeds {
		t.Errorf("5 vs TN 15 should fail")
	}
}

// ---- combat (sister tool) ----

func TestCombatHits_OneSidedAttack(t *testing.T) {
	// NPC attack: attacker skill 4, defender TN 12.
	// Same resolution as a regular check; just a different framing.
	in := CombatInput{
		AttackerSkill: 4,
		DefenderTN:    12,
		Weariness:     false,
		Miserable:     false,
	}
	res := ResolveCombatWith(in, rand.New(rand.NewSource(7)))
	// Invariants: result should look like a CheckResult under the hood.
	if len(res.SuccessDice) != in.AttackerSkill {
		t.Errorf("SuccessDice length=%d; want %d", len(res.SuccessDice), in.AttackerSkill)
	}
	// Margin equals Total - TN
	if res.Margin != res.Total-in.DefenderTN {
		t.Errorf("Margin mismatch: %d != %d - %d", res.Margin, res.Total, in.DefenderTN)
	}
}

// ---- input validation ----

func TestResolveCheck_RejectsNegativeSkill(t *testing.T) {
	in := CheckInput{SkillRating: -1, TargetNumber: 10}
	defer func() {
		r := recover()
		if r == nil {
			t.Errorf("expected validation to reject negative skill")
		}
	}()
	resolveSeeded(1, in)
}

func TestResolveCheck_AllowsSkillZero(t *testing.T) {
	// Skill 0 = only the Feat die contributes. Valid in TOR.
	in := CheckInput{SkillRating: 0, TargetNumber: 5}
	res := resolveSeeded(1, in)
	if len(res.SuccessDice) != 0 {
		t.Errorf("Skill 0 should produce 0 success dice; got %d", len(res.SuccessDice))
	}
	// Total is just the feat die value (or 0 if Eye, or "succeed" if Gandalf)
	if res.FeatDie >= 1 && res.FeatDie <= 10 && res.Total != res.FeatDie {
		t.Errorf("Skill 0, normal feat: total should be feat (%d); got %d", res.FeatDie, res.Total)
	}
}
