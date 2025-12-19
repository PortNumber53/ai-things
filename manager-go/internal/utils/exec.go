package utils

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func RunCommand(command string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	Logf("run: %s", command)

	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if Verbose && output.Len() > 0 {
			out := output.String()
			Logf("output (error):\n%s", strings.TrimRight(out, "\n"))
		}
		return output.String(), fmt.Errorf("command failed: %w", err)
	}
	if Verbose && output.Len() > 0 {
		out := output.String()
		Logf("output:\n%s", strings.TrimRight(out, "\n"))
	}
	return output.String(), nil
}
