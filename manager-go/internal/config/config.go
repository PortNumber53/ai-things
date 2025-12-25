package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	defaultConfigPath = "/etc/ai-things/config.ini"
	configPathEnv     = "AI_THINGS_CONFIG"
)

type Config struct {
	Hostname                   string
	AppEnv                     string
	BaseOutputFolder           string
	BaseAppFolder              string
	SubtitleFolder             string
	SubtitleScript             string
	YoutubeUpload              string
	TikTokUploadScript         string
	Portnumber53APIKey         string
	Portnumber53TimeoutSeconds int

	TTSOnnxModel string
	TTSConfig    string
	TTSVoice     string

	DBURL      string
	DBHost     string
	DBPort     int
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string

	RabbitMQHost     string
	RabbitMQPort     int
	RabbitMQUser     string
	RabbitMQPassword string
	RabbitMQVHost    string

	OllamaHostname    string
	OllamaPort        int
	OllamaModel       string
	GeminiAPIKey      string
	TikTokAccessToken string
	TikTokVideoPath   string

	SlackAppID             string
	SlackClientID          string
	SlackClientSecret      string
	SlackSigningSecret     string
	SlackVerificationToken string
	SlackScopes            string
	SlackRedirectURL       string
	SlackPort              int
	// Slack image workflow (used by Slack-driven image generation).
	SlackTeamID       string
	SlackImageChannel string
}

func Load() (Config, error) {
	configPath := os.Getenv(configPathEnv)
	if configPath == "" {
		configPath = defaultConfigPath
	}

	ini, err := readINI(configPath)
	if err != nil {
		return Config{}, fmt.Errorf("load config %s: %w", configPath, err)
	}

	cfg := Config{}
	cfg.Hostname = ini.get("app", "hostname")
	if cfg.Hostname == "" {
		if host, err := os.Hostname(); err == nil {
			cfg.Hostname = host
		}
	}
	cfg.AppEnv = ini.getDefault("app", "env", "production")

	cfg.BaseOutputFolder = ini.get("app", "base_output_folder")
	cfg.BaseAppFolder = ini.get("app", "base_app_folder")
	cfg.SubtitleFolder = filepath.Join(cfg.BaseOutputFolder, "subtitles")

	cfg.TTSOnnxModel = ini.get("tts", "onnx_model")
	cfg.TTSConfig = ini.get("tts", "config_file")
	cfg.TTSVoice = ini.get("tts", "voice")

	cfg.SubtitleScript = ini.get("paths", "subtitle_script")
	if cfg.SubtitleScript == "" && cfg.BaseAppFolder != "" {
		cfg.SubtitleScript = fmt.Sprintf("python %s", filepath.Join(cfg.BaseAppFolder, "podcast", "whisper.py"))
	}

	cfg.YoutubeUpload = ini.get("paths", "youtube_upload_script")
	if cfg.YoutubeUpload == "" && cfg.BaseAppFolder != "" {
		py := pythonForProjectVenv(cfg.BaseAppFolder, "auto-subtitles-generator")
		script := filepath.Join(cfg.BaseAppFolder, "auto-subtitles-generator", "upload_video.py")
		cfg.YoutubeUpload = fmt.Sprintf("%s %s", py, script)
	}

	cfg.TikTokUploadScript = ini.get("paths", "tiktok_upload_script")
	if cfg.TikTokUploadScript == "" && cfg.BaseAppFolder != "" {
		cfg.TikTokUploadScript = fmt.Sprintf("python %s", filepath.Join(cfg.BaseAppFolder, "utility", "upload-video-to-tiktok.py"))
	}

	cfg.Portnumber53APIKey = ini.get("portnumber53", "api_key")
	cfg.Portnumber53TimeoutSeconds = ini.getIntDefault("portnumber53", "timeout_seconds", 1000)
	// Accept new config keys; fall back to legacy ollama.brain_host for compatibility.
	cfg.OllamaHostname = firstNonEmpty(
		ini.get("ollama", "hostname"),
		ini.get("ollama", "brain_host"),
	)
	cfg.OllamaPort = ini.getIntDefault("ollama", "port", 11434)
	cfg.OllamaModel = ini.getDefault("ollama", "model", "llama3.2")
	cfg.GeminiAPIKey = ini.get("gemini", "api_key")
	cfg.TikTokAccessToken = ini.get("tiktok", "access_token")
	cfg.TikTokVideoPath = ini.get("tiktok", "video_path")

	// Slack (prefer config.ini, fall back to env vars for compatibility).
	cfg.SlackAppID = firstNonEmpty(ini.get("slack", "app_id"), os.Getenv("SLACK_APP_ID"))
	cfg.SlackClientID = firstNonEmpty(ini.get("slack", "client_id"), os.Getenv("SLACK_CLIENT_ID"))
	cfg.SlackClientSecret = firstNonEmpty(ini.get("slack", "client_secret"), os.Getenv("SLACK_CLIENT_SECRET"))
	cfg.SlackSigningSecret = firstNonEmpty(ini.get("slack", "signing_secret"), os.Getenv("SLACK_SIGNING_SECRET"))
	cfg.SlackVerificationToken = firstNonEmpty(ini.get("slack", "verification_token"), os.Getenv("SLACK_VERIFICATION_TOKEN"))
	cfg.SlackScopes = firstNonEmpty(ini.get("slack", "scopes"), os.Getenv("SLACK_SCOPES"))
	cfg.SlackRedirectURL = firstNonEmpty(ini.get("slack", "redirect_url"), os.Getenv("SLACK_REDIRECT_URL"))
	cfg.SlackPort = firstNonEmptyIntDefault(8085, ini.get("slack", "port"), os.Getenv("SLACK_PORT"))
	cfg.SlackTeamID = firstNonEmpty(ini.get("slack", "team_id"), os.Getenv("SLACK_TEAM_ID"))
	cfg.SlackImageChannel = firstNonEmpty(ini.get("slack", "image_channel"), os.Getenv("SLACK_IMAGE_CHANNEL"))

	cfg.DBURL = firstNonEmpty(ini.get("db", "url"), ini.get("db", "database_url"))
	cfg.DBHost = ini.getDefault("db", "host", "127.0.0.1")
	cfg.DBPort = ini.getIntDefault("db", "port", 5432)
	cfg.DBName = ini.getDefault("db", "name", "laravel")
	cfg.DBUser = ini.getDefault("db", "user", "root")
	cfg.DBPassword = ini.get("db", "password")
	cfg.DBSSLMode = ini.getDefault("db", "sslmode", "prefer")

	cfg.RabbitMQHost = ini.getDefault("rabbitmq", "host", "127.0.0.1")
	cfg.RabbitMQPort = ini.getIntDefault("rabbitmq", "port", 5672)
	cfg.RabbitMQUser = ini.getDefault("rabbitmq", "user", "guest")
	cfg.RabbitMQPassword = ini.getDefault("rabbitmq", "password", "guest")
	cfg.RabbitMQVHost = ini.getDefault("rabbitmq", "vhost", "/")

	if cfg.BaseOutputFolder == "" || cfg.BaseAppFolder == "" {
		return cfg, errors.New("app.base_output_folder and app.base_app_folder must be set in config.ini")
	}

	return cfg, nil
}

func (c Config) DBConnString() string {
	if c.DBURL != "" {
		return c.DBURL
	}
	return fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		c.DBHost,
		c.DBPort,
		c.DBName,
		c.DBUser,
		c.DBPassword,
		c.DBSSLMode,
	)
}

func (c Config) RabbitMQURL() string {
	vhost := strings.TrimPrefix(c.RabbitMQVHost, "/")
	return fmt.Sprintf(
		"amqp://%s:%s@%s:%d/%s",
		urlEscape(c.RabbitMQUser),
		urlEscape(c.RabbitMQPassword),
		c.RabbitMQHost,
		c.RabbitMQPort,
		vhost,
	)
}

type iniData struct {
	sections map[string]map[string]string
}

func readINI(path string) (iniData, error) {
	file, err := os.Open(path)
	if err != nil {
		return iniData{}, err
	}
	defer file.Close()

	data := iniData{sections: map[string]map[string]string{}}
	section := "default"
	data.sections[section] = map[string]string{}

	scanner := bufio.NewScanner(file)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(line[1 : len(line)-1])
			section = strings.ToLower(section)
			if section == "" {
				return iniData{}, fmt.Errorf("invalid section header at line %d", lineNo)
			}
			if _, ok := data.sections[section]; !ok {
				data.sections[section] = map[string]string{}
			}
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return iniData{}, fmt.Errorf("invalid line %d: %q", lineNo, line)
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return iniData{}, fmt.Errorf("empty key at line %d", lineNo)
		}
		value = strings.TrimSpace(value)
		value = trimQuotes(value)
		data.sections[section][key] = value
	}
	if err := scanner.Err(); err != nil {
		return iniData{}, err
	}
	return data, nil
}

func trimQuotes(value string) string {
	if len(value) < 2 {
		return value
	}
	if value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	if value[0] == '\'' && value[len(value)-1] == '\'' {
		return value[1 : len(value)-1]
	}
	return value
}

func (ini iniData) get(section, key string) string {
	if len(ini.sections) == 0 {
		return ""
	}
	section = strings.ToLower(section)
	key = strings.ToLower(key)
	if section == "" {
		section = "default"
	}
	if values, ok := ini.sections[section]; ok {
		return values[key]
	}
	return ""
}

func (ini iniData) getDefault(section, key, fallback string) string {
	value := ini.get(section, key)
	if value == "" {
		return fallback
	}
	return value
}

func (ini iniData) getIntDefault(section, key string, fallback int) int {
	value := ini.get(section, key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func firstNonEmptyInt(values ...string) (int, bool) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err != nil {
			continue
		}
		return parsed, true
	}
	return 0, false
}

func firstNonEmptyIntDefault(fallback int, values ...string) int {
	if parsed, ok := firstNonEmptyInt(values...); ok {
		return parsed
	}
	return fallback
}

func urlEscape(value string) string {
	// Keep it simple for now; RabbitMQ credentials here are typically safe.
	return value
}

func pythonForProjectVenv(baseAppFolder, project string) string {
	// Expected layout:
	//   /deploy/ai-things/current  (baseAppFolder)
	//   /deploy/ai-things/venvs/<project>/bin/python
	if baseAppFolder == "" || project == "" {
		return "python"
	}
	baseDeploy := filepath.Dir(baseAppFolder)
	candidate := filepath.Join(baseDeploy, "venvs", project, "bin", "python")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	return "python"
}
