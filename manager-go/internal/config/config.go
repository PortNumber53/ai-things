package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Hostname           string
	AppEnv             string
	BaseOutputFolder   string
	BaseAppFolder      string
	SubtitleFolder     string
	SubtitleScript     string
	YoutubeUpload      string
	TikTokUploadScript string
	Portnumber53APIKey string

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
}

func Load() (Config, error) {
	root := os.Getenv("MANAGER_ROOT")
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Config{}, err
		}
		root = cwd
	}

	// Load .env if present in root.
	_ = godotenv.Overload(filepath.Join(root, ".env"))

	// Load _extra_env in root or parent (matches ExtraEnv behavior).
	_ = loadExtraEnv(root)

	cfg := Config{}
	cfg.Hostname = getenvDefault("HOSTNAME", "")
	if cfg.Hostname == "" {
		if host, err := os.Hostname(); err == nil {
			cfg.Hostname = host
		}
	}
	cfg.AppEnv = getenvDefault("APP_ENV", "production")

	cfg.BaseOutputFolder = os.Getenv("BASE_OUTPUT_FOLDER")
	cfg.BaseAppFolder = os.Getenv("BASE_APP_FOLDER")
	cfg.SubtitleFolder = filepath.Join(cfg.BaseOutputFolder, "subtitles")

	cfg.TTSOnnxModel = os.Getenv("TTS_ONNX_MODEL")
	cfg.TTSConfig = os.Getenv("TT_CONFIG_FILE")
	cfg.TTSVoice = os.Getenv("TTS_VOICE")

	cfg.SubtitleScript = os.Getenv("SUBTITLE_SCRIPT")
	if cfg.SubtitleScript == "" {
		if cfg.AppEnv == "production" {
			cfg.SubtitleScript = "python /deploy/ai-things/current/podcast/whisper.py"
		} else {
			cfg.SubtitleScript = "python /home/grimlock/ai/ai-things/podcast/whisper.py"
		}
	}

	cfg.YoutubeUpload = os.Getenv("YOUTUBE_UPLOAD")
	if cfg.YoutubeUpload == "" {
		if cfg.AppEnv == "production" {
			cfg.YoutubeUpload = "python /deploy/ai-things/current/auto-subtitles-generator/upload_video.py"
		} else {
			cfg.YoutubeUpload = "python upload_video.py"
		}
	}

	cfg.TikTokUploadScript = os.Getenv("TIKTOK_UPLOAD_SCRIPT")
	if cfg.TikTokUploadScript == "" {
		if cfg.AppEnv == "production" {
			cfg.TikTokUploadScript = "python /deploy/ai-things/current/utility/upload-video-to-tiktok.py"
		} else {
			cfg.TikTokUploadScript = "python upload-video-to-tiktok.py"
		}
	}

	cfg.Portnumber53APIKey = os.Getenv("PORTNUMBER53_API_KEY")

	cfg.DBURL = firstNonEmpty(os.Getenv("DB_URL"), os.Getenv("DATABASE_URL"))
	cfg.DBHost = getenvDefault("DB_HOST", "127.0.0.1")
	cfg.DBPort = getenvInt("DB_PORT", 5432)
	cfg.DBName = getenvDefault("DB_DATABASE", "laravel")
	cfg.DBUser = getenvDefault("DB_USERNAME", "root")
	cfg.DBPassword = os.Getenv("DB_PASSWORD")
	cfg.DBSSLMode = getenvDefault("DB_SSLMODE", "prefer")

	cfg.RabbitMQHost = getenvDefault("RABBITMQ_HOST", "127.0.0.1")
	cfg.RabbitMQPort = getenvInt("RABBITMQ_PORT", 5672)
	cfg.RabbitMQUser = getenvDefault("RABBITMQ_USER", "guest")
	cfg.RabbitMQPassword = getenvDefault("RABBITMQ_PASSWORD", "guest")
	cfg.RabbitMQVHost = getenvDefault("RABBITMQ_VHOST", "/")

	if cfg.BaseOutputFolder == "" || cfg.BaseAppFolder == "" {
		return cfg, errors.New("BASE_OUTPUT_FOLDER and BASE_APP_FOLDER must be set")
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

func loadExtraEnv(root string) error {
	candidates := []string{root, filepath.Dir(root)}
	for _, base := range candidates {
		path := filepath.Join(base, "_extra_env")
		if _, err := os.Stat(path); err == nil {
			return godotenv.Overload(path)
		}
	}
	return nil
}

func getenvDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func getenvInt(key string, fallback int) int {
	value := os.Getenv(key)
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

func urlEscape(value string) string {
	// Keep it simple for now; RabbitMQ credentials here are typically safe.
	return value
}
