# rpg-dice-mcp

Go MCP server for tabletop RPG dice rolls. The One Ring 2e first;
D&D / Traveller / Call of Cthulhu / etc. as plug-in packages later.

## Why this exists

LLMs are famously bad at generating random numbers — they bias toward
training-data distribution shape (d20 rolls cluster 14-17; "pick 1-10"
picks 7 ~30% of the time). Worse, they often *elide* mechanical
resolution entirely in narrative-heavy contexts, narrating outcomes
without rolling.

Empirical evidence from a past One Ring campaign on this homelab
(qwen3.6:35b, chat `491ccb24-...`): 56 assistant turns, **only 10**
explicitly framed the Feat die, vs **32 combat-narration moments**.
A ~3:1 ratio of combat scenes to actual mechanical resolution.

This MCP fixes both halves of the problem:

1. **Real RNG** — `crypto/rand`-seeded `math/rand`, χ² uniformity
   verified by test for d6 / d12 / d20.
2. **Forcing function** — when the model HAS to call
   `roll_tor_check`, the tool call appears in the response's
   `tool_calls` field; the mechanics happened, verifiably. No more
   skipped rolls.

See [dvystrcil/homelab#201](https://github.com/dvystrcil/homelab/issues/201)
for the full scoping (TDD discipline, 11 acceptance criteria,
three-repo split rationale).

## Tools exposed

| Tool | Args | Returns |
|---|---|---|
| `roll` | `spec: "NdM±K"` | rolls, modifier, total |
| `roll_tor_check` | `skill_rating, target_number, weariness?, miserable?` | feat_die, gandalf_rune, eye_of_sauron, success_dice, effective_dice, total, succeeds, margin, miserable_eye |
| `roll_tor_combat` | `attacker_skill, defender_tn, weariness?, miserable?` | feat_die, gandalf_rune, eye_of_sauron, success_dice, total, hits, margin, miserable_eye |

More TOR-side tools (`roll_eye_awareness`, `roll_shadow_test`) and other
systems land in follow-up PRs. Each new system is its own
`internal/<system>/` package composing on `internal/dice`.

## Mechanics encoded (TOR 2e)

- **Feat die**: d12. Face `11` = Eye of Sauron (counts as 0 in totals).
  Face `12` = Gandalf rune (automatic success regardless of TN).
- **Success dice**: d6 × skill_rating. Sum contributes to total.
- **Weariness**: Success die rolls 1-3 are treated as 0.
- **Miserable**: Eye result sets `miserable_eye` for the caller's
  Hope/Shadow bookkeeping.
- **Total**: feat_die_contribution + sum(effective_success_dice).
  Compared to TN; >= TN = succeeds. Gandalf auto-succeeds regardless.

Resolution rules live in `internal/tor/tor.go`; rules tests are in
`internal/tor/tor_test.go`.

## Run locally

```bash
# Build
go build -o /tmp/rpg-dice-mcp ./cmd/rpg-dice-mcp

# Stdio (works with Claude Code's MCP stdio bridge)
/tmp/rpg-dice-mcp

# HTTP streamable transport
/tmp/rpg-dice-mcp -http :8080
# health probe
curl -s http://localhost:8080/healthz

# Test
go test -race -cover ./...
```

## Trust audit

Run `bin/mcp-trust-audit.py --mcp rpg-dice-mcp` after deploy per the
homelab MCP trust model. Stdio bridge + cluster-internal-only Service.

## Repo layout

```
rpg-dice-mcp-docker/        ← this repo (build: Go + Dockerfile + CI)
rpg-dice-mcp/               ← deploy repo (kustomize manifests)
argocd-projects/rpg-dice-mcp/  ← Argo Application entry
```

Three-repo split per
[dvystrcil/homelab project_mcp_repo_split_pattern](https://github.com/dvystrcil/homelab).

## License

MIT — see `LICENSE`.
