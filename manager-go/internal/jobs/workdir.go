package jobs

import (
	"os"
	"path/filepath"
	"strings"
)

func dirExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	st, err := os.Stat(path)
	return err == nil && st.IsDir()
}

// guessScriptDirFromCommand tries to find an existing *.py path in a command string and returns its directory.
func guessScriptDirFromCommand(cmd string) string {
	parts := strings.Fields(cmd)
	// Walk from the end; scripts tend to be late in the command.
	for i := len(parts) - 1; i >= 0; i-- {
		p := strings.Trim(parts[i], "\"'")
		if !strings.HasSuffix(p, ".py") {
			continue
		}
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		return filepath.Dir(p)
	}
	return ""
}

func resolveWorkDir(candidates []string, command string) string {
	for _, c := range candidates {
		if dirExists(c) {
			return c
		}
	}
	if fromCmd := guessScriptDirFromCommand(command); dirExists(fromCmd) {
		return fromCmd
	}
	return "."
}
