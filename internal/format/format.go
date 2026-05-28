// Package format renders TOR dice results into presentation forms
// (HTML with TOR-themed CSS span classes, plain text, etc.) so consumers
// don't each have to re-implement the same display logic.
//
// The MCP itself stays pure — these formatters are OPT-IN via the
// `format` parameter on the tool call. Default behavior returns
// structured JSON only.
//
// Class scheme (must match the CSS in dvystrcil/homelab
// architecture/spikes/tolkien-lorebook.css):
//
//   .fdie1 .. .fdie10           — Feat Die numeric faces (1-10)
//   .fdieeye   (alias .fdie11)  — Eye of Sauron rune
//   .fdiegandalf (alias .fdie12) — Gandalf rune
//   .sdie1 .. .sdie11           — Success Die face (1-5 pips, 6 = success tengwar,
//                                  7-11 = doubled-pip faces in some variants)
//
// Tracked: dvystrcil/homelab#261 (architectural decision to put
// formatting in the MCP so the dice-roller web app, n8n workflows,
// and any future TOR-aware consumer get the gilded display "for free").
package format

import (
	"fmt"
	"strings"

	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/tor"
)

// Style enumerates the supported output formats. Callers pass a
// string in the tool argument; UnmarshalStyle normalizes it.
type Style string

const (
	StyleNone    Style = "none"     // default: no formatted field in response
	StyleHTMLTOR Style = "html_tor" // HTML with TOR-themed span classes
	StylePlain   Style = "plain"    // plain text, no markup
	StyleMarkdown Style = "markdown" // markdown-flavored
)

// UnmarshalStyle accepts the input arg's string form and returns the
// canonical Style, defaulting to StyleNone if empty / unrecognized.
func UnmarshalStyle(s string) Style {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "html", "html_tor":
		return StyleHTMLTOR
	case "plain", "text":
		return StylePlain
	case "markdown", "md":
		return StyleMarkdown
	case "", "none":
		return StyleNone
	default:
		return StyleNone
	}
}

// FeatSpan renders a Feat-die face as an inline HTML <span> with the
// TOR-themed class. Numeric faces 1-10 use .fdieN with the digit as
// text. Eye (11) and Gandalf (12) use .fdieeye / .fdiegandalf with
// short labels ("Eye" / "G").
func FeatSpan(face int) string {
	switch {
	case face == 11:
		return `<span class="fdieeye">Eye</span>`
	case face == 12:
		return `<span class="fdiegandalf">G</span>`
	case face >= 1 && face <= 10:
		return fmt.Sprintf(`<span class="fdie%d">%d</span>`, face, face)
	default:
		// Out of range — return plain. Callers can decide whether
		// this is an error or just display a number bare.
		return fmt.Sprintf("%d", face)
	}
}

// SuccessSpan renders one Success-die face as an inline HTML <span>.
// Faces are 1-11 in the standard CSS class set.
func SuccessSpan(face int) string {
	if face >= 1 && face <= 11 {
		return fmt.Sprintf(`<span class="sdie%d">%d</span>`, face, face)
	}
	return fmt.Sprintf("%d", face)
}

// TORCheckHTML renders a complete TOR check result as inline HTML.
// Layout: "<feat-span> + <succ-span> <succ-span> ... = <total> vs TN <tn>"
// plus a verbal success/failure tail.
func TORCheckHTML(res tor.CheckResult) string {
	var b strings.Builder
	b.WriteString(FeatSpan(res.FeatDie))
	if len(res.SuccessDice) > 0 {
		b.WriteString(" + ")
		parts := make([]string, len(res.SuccessDice))
		for i, d := range res.SuccessDice {
			parts[i] = SuccessSpan(d)
		}
		b.WriteString(strings.Join(parts, " "))
	}
	if res.Input.TargetNumber > 0 {
		fmt.Fprintf(&b, " = %d vs TN %d", res.Total, res.Input.TargetNumber)
		if res.Succeeds {
			fmt.Fprintf(&b, " — succeeded by %d", res.Margin)
		} else {
			fmt.Fprintf(&b, " — failed by %d", -res.Margin)
		}
	} else {
		fmt.Fprintf(&b, " = %d", res.Total)
	}
	if res.EyeOfSauron {
		b.WriteString(" (Eye of Sauron!)")
	}
	if res.GandalfRune {
		b.WriteString(" (Gandalf rune!)")
	}
	if res.MiserableEye {
		b.WriteString(" — Miserable Eye triggers")
	}
	return b.String()
}

// TORCheckPlain renders a TOR check result as plain text (no markup).
// Suitable for terminals, logs, or HTML-unaware consumers.
func TORCheckPlain(res tor.CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Feat %s", featTextOnly(res.FeatDie))
	if len(res.SuccessDice) > 0 {
		fmt.Fprintf(&b, " + Success %s", joinInts(res.SuccessDice, ", "))
	}
	if res.Input.TargetNumber > 0 {
		fmt.Fprintf(&b, " = %d vs TN %d (%s by %d)",
			res.Total, res.Input.TargetNumber, succWord(res.Succeeds), abs(res.Margin))
	} else {
		fmt.Fprintf(&b, " = %d", res.Total)
	}
	if res.EyeOfSauron {
		b.WriteString(", Eye of Sauron")
	}
	if res.GandalfRune {
		b.WriteString(", Gandalf rune")
	}
	if res.MiserableEye {
		b.WriteString(", MISERABLE EYE")
	}
	return b.String()
}

// TORCheckMarkdown renders a TOR check result with bold and emoji.
// Useful for Discord / Slack / non-HTML chat surfaces.
func TORCheckMarkdown(res tor.CheckResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🎲 **Feat %s**", featTextOnly(res.FeatDie))
	if len(res.SuccessDice) > 0 {
		fmt.Fprintf(&b, " + **Success %s**", joinInts(res.SuccessDice, ", "))
	}
	if res.Input.TargetNumber > 0 {
		fmt.Fprintf(&b, " = **%d** vs TN %d", res.Total, res.Input.TargetNumber)
		if res.Succeeds {
			fmt.Fprintf(&b, " ✅ (margin +%d)", res.Margin)
		} else {
			fmt.Fprintf(&b, " ❌ (failed by %d)", -res.Margin)
		}
	} else {
		fmt.Fprintf(&b, " = **%d**", res.Total)
	}
	if res.EyeOfSauron {
		b.WriteString(" 👁️")
	}
	if res.GandalfRune {
		b.WriteString(" 🧙")
	}
	if res.MiserableEye {
		b.WriteString(" — Miserable Eye")
	}
	return b.String()
}

// TORCheck dispatches to the right renderer for a given Style.
// Returns the empty string for StyleNone.
func TORCheck(res tor.CheckResult, style Style) string {
	switch style {
	case StyleHTMLTOR:
		return TORCheckHTML(res)
	case StylePlain:
		return TORCheckPlain(res)
	case StyleMarkdown:
		return TORCheckMarkdown(res)
	default:
		return ""
	}
}

// ---- helpers ----

func featTextOnly(face int) string {
	switch face {
	case 11:
		return "Eye"
	case 12:
		return "Gandalf"
	default:
		return fmt.Sprintf("%d", face)
	}
}

func joinInts(xs []int, sep string) string {
	parts := make([]string, len(xs))
	for i, x := range xs {
		parts[i] = fmt.Sprintf("%d", x)
	}
	return strings.Join(parts, sep)
}

func succWord(b bool) string {
	if b {
		return "succeeded"
	}
	return "failed"
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
