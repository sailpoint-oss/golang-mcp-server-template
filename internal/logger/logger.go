// Package logger keeps stdout clear for the MCP stdio transport.
package logger

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// stdout captures the real process stdout at package initialisation, before
// Redirect rewires os.Stdout. This runs before main, so it always holds the
// genuine descriptor.
var stdout = os.Stdout

// Redirect points os.Stdout and the standard log package at stderr, and returns
// the real stdout for the MCP transport to write JSON-RPC frames to.
//
// The MCP stdio transport uses stdout exclusively for JSON-RPC frames. Anything
// else written there corrupts the stream, and the SailPoint SDK writes
// diagnostics with fmt.Print (e.g. the deprecation notice in
// NewDefaultConfiguration, and token-request failures in getAccessToken), so
// stdout is pinned to stderr before the SDK is touched.
func Redirect() *os.File {
	os.Stdout = os.Stderr
	log.SetOutput(os.Stderr)
	log.SetFlags(0)
	return stdout
}

// Log writes a diagnostic line to stderr.
func Log(message string, detail ...any) {
	suffix := ""
	if len(detail) > 0 {
		suffix = " " + safeString(detail[0])
	}
	fmt.Fprintf(os.Stderr, "[sailpoint-mcp] %s%s\n", message, suffix)
}

func safeString(value any) string {
	if s, ok := value.(string); ok {
		return s
	}
	if err, ok := value.(error); ok {
		return err.Error()
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
}
