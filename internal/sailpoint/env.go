package sailpoint

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotEnv loads .env from the repo root, if present, and returns the path it
// loaded ("" when none was found). MCP clients launch the server from their own
// working directory, so the file is located relative to the binary
// (bin/sailpoint-mcp-server -> ..), falling back to ./.env for `go run .`.
// Variables already set in the environment (a client env block, an exported
// shell var) always win.
func LoadDotEnv() (string, error) {
	for _, path := range dotEnvCandidates() {
		err := godotenv.Load(path)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("unable to read %s: %w", path, err)
		}
	}
	return "", nil
}

func dotEnvCandidates() []string {
	var candidates []string
	if executable, err := os.Executable(); err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			executable = resolved
		}
		candidates = append(candidates, filepath.Join(filepath.Dir(executable), "..", ".env"))
	}
	return append(candidates, ".env")
}
