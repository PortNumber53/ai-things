package utils

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func DecodeMeta(metaJSON []byte) (map[string]any, error) {
	if len(metaJSON) == 0 {
		return map[string]any{}, nil
	}
	var meta map[string]any
	if err := json.Unmarshal(metaJSON, &meta); err != nil {
		return nil, err
	}
	return meta, nil
}

func EnsureStatusMap(meta map[string]any) map[string]any {
	status, ok := meta["status"].(map[string]any)
	if !ok {
		status = map[string]any{}
		meta["status"] = status
	}
	return status
}

func SetStatus(meta map[string]any, key string, value bool) {
	status := EnsureStatusMap(meta)
	status[key] = value
}

func GetStatus(meta map[string]any, key string) (bool, bool) {
	status, ok := meta["status"].(map[string]any)
	if !ok {
		return false, false
	}
	value, ok := status[key]
	if !ok {
		return false, false
	}
	switch v := value.(type) {
	case bool:
		return v, true
	case string:
		return strings.EqualFold(v, "true"), true
	default:
		return false, true
	}
}

func GetString(meta map[string]any, path ...string) (string, bool) {
	value, ok := GetValue(meta, path...)
	if !ok {
		return "", false
	}
	str, ok := value.(string)
	if !ok {
		return "", false
	}
	return str, true
}

func GetMap(meta map[string]any, path ...string) (map[string]any, bool) {
	value, ok := GetValue(meta, path...)
	if !ok {
		return nil, false
	}
	result, ok := value.(map[string]any)
	return result, ok
}

func GetValue(meta map[string]any, path ...string) (any, bool) {
	current := any(meta)
	for _, key := range path {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[key]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(key)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}
	return current, true
}

func ExtractTextFromMeta(meta map[string]any) (string, error) {
	// Prefer the "clean" canonical text if present. This avoids TTS reading out markdown
	// formatting that may exist in model responses (e.g. "**TITLE:**" => "asterisk, asterisk...").
	if text, ok := GetString(meta, "original_text"); ok && text != "" {
		return ProcessText(text), nil
	}
	if text, ok := GetString(meta, "ollama_response", "response"); ok && text != "" {
		return ProcessText(text), nil
	}
	if text, ok := GetString(meta, "gemini_response", "candidates", "0", "content", "parts", "0", "text"); ok && text != "" {
		return ProcessText(text), nil
	}
	return "", errors.New("text not found in meta")
}

func ProcessText(raw string) string {
	// Remove markdown emphasis markers so TTS doesn't speak them.
	// Example: "**TITLE:** Foo" would otherwise become "asterisk asterisk title colon asterisk asterisk foo".
	raw = strings.ReplaceAll(raw, "*", "")

	lines := strings.Split(raw, "\n")
	var builder strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TITLE:") {
			continue
		}
		if strings.HasPrefix(line, "CONTENT:") {
			line = strings.TrimPrefix(line, "CONTENT:")
			builder.WriteString(line)
			builder.WriteString("\n")
			continue
		}
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func MD5String(value string) string {
	sum := md5.Sum([]byte(value))
	return hex.EncodeToString(sum[:])
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func EnsureDir(path string) error {
	if path == "" {
		return fmt.Errorf("empty path")
	}
	Logf("ensure dir: %s", path)
	return os.MkdirAll(path, 0o777)
}
