package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Prompt(message string) (string, error) {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", message)
	text, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(text), nil
}
