// Command smoke performs an MCP handshake against the built server binary,
// lists its tools, and issues one live search_identities call.
//
// Usage: go run ./cmd/smoke [path-to-server-binary]
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatalf("FAIL %v", err)
	}
}

func run(ctx context.Context) error {
	binary := "./bin/sailpoint-mcp-server"
	if len(os.Args) > 1 {
		binary = os.Args[1]
	}

	command := exec.Command(binary)
	command.Stderr = os.Stderr

	client := mcp.NewClient(&mcp.Implementation{Name: "smoke", Version: "0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: command}, nil)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer session.Close()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		return fmt.Errorf("tools/list: %w", err)
	}
	dump("TOOLS", tools)

	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "search_identities",
		Arguments: map[string]any{
			"query":      "*",
			"limit":      1,
			"attributes": []string{"id", "name"},
		},
	})
	if err != nil {
		return fmt.Errorf("tools/call: %w", err)
	}
	dump("CALL", result)

	if result.IsError {
		return fmt.Errorf("search_identities returned a tool error")
	}
	return nil
}

func dump(label string, value any) {
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", label, value)
		return
	}
	fmt.Fprintf(os.Stderr, "%s: %s\n", label, truncate(string(encoded), 2000))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "... (truncated)"
}
