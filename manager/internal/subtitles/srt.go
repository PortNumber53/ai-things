package subtitles

import (
	"regexp"
	"strconv"
	"strings"
)

type Caption struct {
	StartTime string
	EndTime   string
	Text      string
}

var timeRegex = regexp.MustCompile(`(\d\d:\d\d:\d\d,\d\d\d)\s-->\s(\d\d:\d\d:\d\d,\d\d\d)`)

func ParseSRT(input string) []Caption {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil
	}
	blocks := splitBlocks(trimmed)
	captions := make([]Caption, 0, len(blocks))
	for _, block := range blocks {
		lines := splitLines(block)
		if len(lines) < 2 {
			continue
		}
		// First line is index; second line is time range.
		matches := timeRegex.FindStringSubmatch(lines[1])
		if len(matches) < 3 {
			continue
		}
		text := ""
		if len(lines) > 2 {
			text = strings.Join(lines[2:], "\n")
		}
		captions = append(captions, Caption{
			StartTime: matches[1],
			EndTime:   matches[2],
			Text:      strings.TrimRight(text, "\n"),
		})
	}
	return captions
}

func SerializeSRT(captions []Caption) string {
	var builder strings.Builder
	for idx, caption := range captions {
		builder.WriteString(intToString(idx + 1))
		builder.WriteString("\n")
		builder.WriteString(caption.StartTime)
		builder.WriteString(" --> ")
		builder.WriteString(caption.EndTime)
		builder.WriteString("\n")
		builder.WriteString(caption.Text)
		builder.WriteString("\n\n")
	}
	return builder.String()
}

func NormalizeText(input string) string {
	text := strings.ReplaceAll(input, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	return strings.TrimRight(text, "\n")
}

func splitBlocks(input string) []string {
	re := regexp.MustCompile(`\r?\n\r?\n+`)
	return re.Split(input, -1)
}

func splitLines(input string) []string {
	text := NormalizeText(input)
	return strings.Split(text, "\n")
}

func intToString(value int) string {
	return strconv.Itoa(value)
}
