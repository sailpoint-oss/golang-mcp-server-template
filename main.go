// Command golang-mcp-server-template serves the SailPoint MCP tools over stdio.
package main

import (
	"context"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/sailpoint-oss/golang-mcp-server-template/internal/logger"
	"github.com/sailpoint-oss/golang-mcp-server-template/internal/sailpoint"
	"github.com/sailpoint-oss/golang-mcp-server-template/internal/tools"
)

const version = "0.1.0"

func main() {
	// Before anything that might print: the SailPoint SDK writes diagnostics to
	// stdout, which would corrupt the JSON-RPC stream. Redirect returns the real
	// stdout for the transport to use.
	stdout := logger.Redirect()

	if err := run(context.Background(), stdout); err != nil {
		logger.Log("fatal", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout *os.File) error {
	// Fail fast with a clear message instead of on the first tool call.
	if _, err := sailpoint.Client(); err != nil {
		return err
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "sailpoint-mcp-server",
		Version: version,
	}, nil)

	if err := tools.RegisterSearchIdentities(server); err != nil {
		return err
	}

	logger.Log("server ready on stdio")

	return server.Run(ctx, &mcp.IOTransport{Reader: os.Stdin, Writer: stdout})
}
