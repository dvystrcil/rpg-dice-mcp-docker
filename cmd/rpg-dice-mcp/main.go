// rpg-dice-mcp — Go MCP server for tabletop RPG dice.
//
// Built to fix the LLM-as-RNG problem documented in dvystrcil/homelab#201:
// language models can't generate uniformly-random numbers and tend to
// elide mechanical resolution in narrative-heavy contexts (the past TOR
// campaign's 32:10 combat-narrations-to-feat-die-framings ratio). This
// MCP gives the model a real tool to call — when the tool call appears
// in the response, the mechanics happened. No more skipped rolls.
//
// Two layers:
//   - Generic dice rolling: tool "roll" takes an "NdM±K" spec.
//   - The One Ring 2e specific: "roll_tor_check" + "roll_tor_combat"
//     encode Feat-die / Eye / Gandalf / weariness / miserable.
//
// More TOR-side tools (eye-awareness, shadow tests) and other systems
// (D&D, Traveller, CoC) follow in later PRs.
//
// This file is intentionally thin — flag parsing + assembly. All
// real logic lives in internal/{dice,tor,mcpsrv}.
package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/dvystrcil/rpg-dice-mcp-docker/internal/mcpsrv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	httpAddr := flag.String("http",
		envOr("HTTP_ADDR", ""),
		"If set, serve MCP over HTTP at this address (e.g. ':8080'). Otherwise use stdio.")
	flag.Parse()

	log.Printf("Starting rpg-dice-mcp")

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "rpg-dice-mcp",
		Version: "0.1.0",
	}, nil)

	registered := mcpsrv.RegisterAll(server)
	log.Printf("Registered %d tool(s): %v", len(registered), registered)

	if *httpAddr != "" {
		// Streamable HTTP transport — for in-cluster deployment.
		// The handler factory returns the same server for every request
		// (single-tenant; dice rolls are stateless).
		mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
			return server
		}, nil)

		// Separate /healthz route so kubelet liveness/readiness probes
		// don't trip over the MCP streamable handler — it doesn't 200
		// on bare GET / (it expects MCP protocol traffic).
		// See dvystrcil/mcp-server-docker/cmd/mcp-server/main.go for
		// the same pattern + rationale.
		mux := http.NewServeMux()
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok\n"))
		})
		mux.Handle("/", mcpHandler)

		log.Printf("Listening on HTTP %s (streamable transport, /healthz live)", *httpAddr)
		if err := http.ListenAndServe(*httpAddr, mux); err != nil {
			log.Fatalf("http server: %v", err)
		}
		return
	}

	// Stdio fallback — works with Claude Code on the workstation
	// connecting via the MCP stdio bridge, and any client preferring stdio.
	log.Printf("Listening on stdio")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}
