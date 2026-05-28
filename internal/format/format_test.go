package format

import (
	"strings"
	"testing"

	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/tor"
)

func TestUnmarshalStyle(t *testing.T) {
	cases := []struct {
		in   string
		want Style
	}{
		{"", StyleNone},
		{"none", StyleNone},
		{"html", StyleHTMLTOR},
		{"html_tor", StyleHTMLTOR},
		{"HTML_TOR", StyleHTMLTOR},
		{"plain", StylePlain},
		{"text", StylePlain},
		{"markdown", StyleMarkdown},
		{"md", StyleMarkdown},
		{"garbage", StyleNone},
	}
	for _, c := range cases {
		if got := UnmarshalStyle(c.in); got != c.want {
			t.Errorf("UnmarshalStyle(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestFeatSpan(t *testing.T) {
	cases := []struct {
		face int
		want string
	}{
		{1, `<span class="fdie1">1</span>`},
		{7, `<span class="fdie7">7</span>`},
		{10, `<span class="fdie10">10</span>`},
		{11, `<span class="fdieeye">Eye</span>`},
		{12, `<span class="fdiegandalf">G</span>`},
		{0, "0"},   // out of range — bare digit
		{13, "13"}, // out of range — bare digit
	}
	for _, c := range cases {
		if got := FeatSpan(c.face); got != c.want {
			t.Errorf("FeatSpan(%d) = %q, want %q", c.face, got, c.want)
		}
	}
}

func TestSuccessSpan(t *testing.T) {
	if got := SuccessSpan(4); got != `<span class="sdie4">4</span>` {
		t.Errorf("SuccessSpan(4) = %q", got)
	}
	if got := SuccessSpan(11); got != `<span class="sdie11">11</span>` {
		t.Errorf("SuccessSpan(11) = %q", got)
	}
	if got := SuccessSpan(0); got != "0" {
		t.Errorf("SuccessSpan(0) bare = %q", got)
	}
}

func TestTORCheckHTML_Basic(t *testing.T) {
	res := tor.CheckResult{
		Input:       tor.CheckInput{SkillRating: 2, TargetNumber: 14},
		FeatDie:     7,
		SuccessDice: []int{4, 6},
		Total:       17,
		Margin:      3,
		Succeeds:    true,
	}
	got := TORCheckHTML(res)
	wantParts := []string{
		`<span class="fdie7">7</span>`,
		`<span class="sdie4">4</span>`,
		`<span class="sdie6">6</span>`,
		"= 17 vs TN 14",
		"succeeded by 3",
	}
	for _, p := range wantParts {
		if !strings.Contains(got, p) {
			t.Errorf("TORCheckHTML missing %q\n  got: %s", p, got)
		}
	}
}

func TestTORCheckHTML_Eye(t *testing.T) {
	res := tor.CheckResult{
		Input:        tor.CheckInput{SkillRating: 1, TargetNumber: 12, Miserable: true},
		FeatDie:      11,
		EyeOfSauron:  true,
		SuccessDice:  []int{4},
		Total:        4,
		Margin:       -8,
		Succeeds:     false,
		MiserableEye: true,
	}
	got := TORCheckHTML(res)
	if !strings.Contains(got, `<span class="fdieeye">Eye</span>`) {
		t.Errorf("expected fdieeye span\n  got: %s", got)
	}
	if !strings.Contains(got, "Eye of Sauron") {
		t.Errorf("expected verbal Eye annotation\n  got: %s", got)
	}
	if !strings.Contains(got, "Miserable Eye") {
		t.Errorf("expected Miserable Eye annotation\n  got: %s", got)
	}
}

func TestTORCheckHTML_Gandalf(t *testing.T) {
	res := tor.CheckResult{
		Input:       tor.CheckInput{SkillRating: 2, TargetNumber: 14},
		FeatDie:     12,
		GandalfRune: true,
		SuccessDice: []int{3, 5},
		Total:       8,
		Margin:      -6, // post-hoc; succeeds is true via Gandalf
		Succeeds:    true,
	}
	got := TORCheckHTML(res)
	if !strings.Contains(got, `<span class="fdiegandalf">G</span>`) {
		t.Errorf("expected fdiegandalf span\n  got: %s", got)
	}
	if !strings.Contains(got, "Gandalf rune") {
		t.Errorf("expected verbal Gandalf annotation\n  got: %s", got)
	}
}

func TestTORCheckHTML_NoTN(t *testing.T) {
	// When TN=0 (e.g. a quick test roll), don't render the "vs TN" tail.
	res := tor.CheckResult{
		Input:       tor.CheckInput{SkillRating: 0, TargetNumber: 0},
		FeatDie:     6,
		SuccessDice: nil,
		Total:       6,
	}
	got := TORCheckHTML(res)
	if !strings.Contains(got, "= 6") {
		t.Errorf("expected total\n  got: %s", got)
	}
	if strings.Contains(got, "vs TN") {
		t.Errorf("did not expect TN tail\n  got: %s", got)
	}
}

func TestTORCheckPlain(t *testing.T) {
	res := tor.CheckResult{
		Input:       tor.CheckInput{SkillRating: 2, TargetNumber: 14},
		FeatDie:     7,
		SuccessDice: []int{4, 6},
		Total:       17,
		Margin:      3,
		Succeeds:    true,
	}
	got := TORCheckPlain(res)
	if strings.Contains(got, "<span") {
		t.Errorf("plain should not contain HTML\n  got: %s", got)
	}
	for _, want := range []string{"Feat 7", "Success 4, 6", "= 17 vs TN 14", "succeeded"} {
		if !strings.Contains(got, want) {
			t.Errorf("plain missing %q\n  got: %s", want, got)
		}
	}
}

func TestTORCheckMarkdown(t *testing.T) {
	res := tor.CheckResult{
		Input:       tor.CheckInput{SkillRating: 1, TargetNumber: 14},
		FeatDie:     11,
		EyeOfSauron: true,
		SuccessDice: []int{3},
		Total:       3,
		Margin:      -11,
		Succeeds:    false,
	}
	got := TORCheckMarkdown(res)
	if strings.Contains(got, "<span") {
		t.Errorf("markdown should not contain HTML\n  got: %s", got)
	}
	for _, want := range []string{"🎲", "**Feat Eye**", "❌", "👁️"} {
		if !strings.Contains(got, want) {
			t.Errorf("markdown missing %q\n  got: %s", want, got)
		}
	}
}

func TestTORCheck_Dispatch(t *testing.T) {
	res := tor.CheckResult{FeatDie: 5, Total: 5}
	if TORCheck(res, StyleNone) != "" {
		t.Error("StyleNone should return empty")
	}
	if !strings.Contains(TORCheck(res, StyleHTMLTOR), "fdie5") {
		t.Error("StyleHTMLTOR should produce span")
	}
	if !strings.Contains(TORCheck(res, StylePlain), "Feat 5") {
		t.Error("StylePlain should produce plain text")
	}
	if !strings.Contains(TORCheck(res, StyleMarkdown), "🎲") {
		t.Error("StyleMarkdown should include emoji")
	}
}
