// Package mcpsrv wires the rpg-dice-mcp tools into an MCP server.
// Each tool is statically defined here — unlike mcp-server-docker which
// derives tools from SKILL.md files, this server has a fixed,
// known-at-compile-time tool catalog.
package mcpsrv

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/dice"
	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/tor"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterAll attaches every dice-mcp tool to the given server.
// Returns the ordered list of registered tool names, useful for
// startup logging and the integration test.
func RegisterAll(server *mcp.Server) []string {
	names := []string{}
	for _, t := range []struct {
		name string
		add  func(*mcp.Server)
	}{
		{"roll", addRoll},
		{"roll_tor_check", addRollTORCheck},
		{"roll_tor_combat", addRollTORCombat},
	} {
		t.add(server)
		names = append(names, t.name)
	}
	return names
}

// ---- helpers ----

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

func jsonResult(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err))
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: string(b)}},
	}
}

func errorResult(msg string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{&mcp.TextContent{Text: msg}},
	}
}

// decodeArgs unmarshals the tool call's JSON arguments into the given
// pointer. Returns a CallToolResult with IsError on failure (so the
// handler can `if result := decodeArgs(...); result != nil { return result, nil }`).
func decodeArgs(req *mcp.CallToolRequest, dst any) *mcp.CallToolResult {
	if len(req.Params.Arguments) == 0 {
		// caller may want empty args; that's their call to validate.
		return nil
	}
	if err := json.Unmarshal(req.Params.Arguments, dst); err != nil {
		return errorResult(fmt.Sprintf("decode arguments: %v", err))
	}
	return nil
}

// ---- tool: roll (generic) ----

func addRoll(server *mcp.Server) {
	tool := &mcp.Tool{
		Name: "roll",
		Description: "Roll a generic dice expression (NdM±K). " +
			"Use this for any non-system-specific roll: 2d6+1, 1d20, 3d8-2, d12. " +
			"For The One Ring tests, prefer roll_tor_check which encodes the system rules.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"spec": {
					Type:        "string",
					Description: "Dice expression, e.g. \"2d6+1\", \"1d20\", \"d12\", \"3d8-2\".",
				},
			},
			Required: []string{"spec"},
		},
	}
	server.AddTool(tool, handleRoll)
}

func handleRoll(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		Spec string `json:"spec"`
	}
	if e := decodeArgs(req, &args); e != nil {
		return e, nil
	}
	if args.Spec == "" {
		return errorResult("missing required argument: spec"), nil
	}
	spec, err := dice.ParseSpec(args.Spec)
	if err != nil {
		return errorResult(err.Error()), nil
	}
	res := dice.Roll(spec)
	return jsonResult(map[string]any{
		"spec":     args.Spec,
		"rolls":    res.Rolls,
		"modifier": res.Modifier,
		"total":    res.Total,
	}), nil
}

// ---- tool: roll_tor_check ----

func addRollTORCheck(server *mcp.Server) {
	tool := &mcp.Tool{
		Name: "roll_tor_check",
		Description: "Resolve a The One Ring check. Rolls the Feat die (d12) plus " +
			"skill_rating × Success dice (d6), interprets Eye/Gandalf special faces, " +
			"applies weariness if set, and reports succeeds/margin against the TN. " +
			"The Loremaster calls this for NPC/Loremaster-side rolls (per the dice-resolution skill). " +
			"Player-side rolls are physical — the Loremaster only frames them.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"skill_rating": {
					Type:        "integer",
					Description: "Rating of the relevant skill or attribute (>= 0). Determines Success dice count.",
					Minimum:     ptrFloat(0),
				},
				"target_number": {
					Type:        "integer",
					Description: "TN to meet or beat for success.",
				},
				"weariness": {
					Type:        "boolean",
					Description: "When true, Success die rolls of 1-3 are treated as 0.",
				},
				"miserable": {
					Type:        "boolean",
					Description: "When true, an Eye result sets MiserableEye in the response for Hope/Shadow bookkeeping.",
				},
			},
			Required: []string{"skill_rating", "target_number"},
		},
	}
	server.AddTool(tool, handleRollTORCheck)
}

func handleRollTORCheck(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		SkillRating  int  `json:"skill_rating"`
		TargetNumber int  `json:"target_number"`
		Weariness    bool `json:"weariness"`
		Miserable    bool `json:"miserable"`
	}
	if e := decodeArgs(req, &args); e != nil {
		return e, nil
	}
	if args.SkillRating < 0 {
		return errorResult(fmt.Sprintf("skill_rating must be >= 0, got %d", args.SkillRating)), nil
	}
	res := tor.ResolveCheck(tor.CheckInput{
		SkillRating:  args.SkillRating,
		TargetNumber: args.TargetNumber,
		Weariness:    args.Weariness,
		Miserable:    args.Miserable,
	})
	return jsonResult(map[string]any{
		"feat_die":         res.FeatDie,
		"gandalf_rune":     res.GandalfRune,
		"eye_of_sauron":    res.EyeOfSauron,
		"success_dice":     res.SuccessDice,
		"effective_dice":   res.EffectiveSuccessDice,
		"total":            res.Total,
		"succeeds":         res.Succeeds,
		"margin":           res.Margin,
		"miserable_eye":    res.MiserableEye,
		"target_number":    args.TargetNumber,
	}), nil
}

// ---- tool: roll_tor_combat ----

func addRollTORCombat(server *mcp.Server) {
	tool := &mcp.Tool{
		Name: "roll_tor_combat",
		Description: "Resolve a TOR combat attack roll. Thin wrapper over roll_tor_check " +
			"using combat-flavored field names (attacker_skill / defender_tn). " +
			"In TOR Strider Mode the player rolls most combat checks; use this when " +
			"the Loremaster genuinely needs to roll for an NPC's offensive action.",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"attacker_skill": {
					Type:        "integer",
					Description: "Rating of the attacker's weapon skill (Sword, Spear, Bow, etc.). Success dice count.",
					Minimum:     ptrFloat(0),
				},
				"defender_tn": {
					Type:        "integer",
					Description: "Defender's effective Parry TN.",
				},
				"weariness": {
					Type:        "boolean",
					Description: "When true, attacker's Success dice 1-3 count as 0.",
				},
				"miserable": {
					Type:        "boolean",
					Description: "When true, Eye results trigger MiserableEye flag.",
				},
			},
			Required: []string{"attacker_skill", "defender_tn"},
		},
	}
	server.AddTool(tool, handleRollTORCombat)
}

func handleRollTORCombat(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	var args struct {
		AttackerSkill int  `json:"attacker_skill"`
		DefenderTN    int  `json:"defender_tn"`
		Weariness     bool `json:"weariness"`
		Miserable     bool `json:"miserable"`
	}
	if e := decodeArgs(req, &args); e != nil {
		return e, nil
	}
	if args.AttackerSkill < 0 {
		return errorResult(fmt.Sprintf("attacker_skill must be >= 0, got %d", args.AttackerSkill)), nil
	}
	res := tor.ResolveCombat(tor.CombatInput{
		AttackerSkill: args.AttackerSkill,
		DefenderTN:    args.DefenderTN,
		Weariness:     args.Weariness,
		Miserable:     args.Miserable,
	})
	return jsonResult(map[string]any{
		"feat_die":      res.FeatDie,
		"gandalf_rune":  res.GandalfRune,
		"eye_of_sauron": res.EyeOfSauron,
		"success_dice":  res.SuccessDice,
		"total":         res.Total,
		"hits":          res.Succeeds,
		"margin":        res.Margin,
		"miserable_eye": res.MiserableEye,
		"defender_tn":   args.DefenderTN,
	}), nil
}

// ---- schema helpers ----

func ptrFloat(v float64) *float64 { return &v }
