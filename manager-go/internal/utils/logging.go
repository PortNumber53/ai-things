package utils

import (
	"fmt"
	"os"
	"time"
)

// Verbose enables diagnostic logging across the manager.
var Verbose bool

func Logf(format string, args ...any) {
	if !Verbose {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(os.Stderr, "[%s] "+format+"\n", append([]any{timestamp}, args...)...)
}
