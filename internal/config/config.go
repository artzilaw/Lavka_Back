package config

import (
	"encoding/json"
	"flag"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env             string
	StoragePath     string
	DatabaseURL     string
	TokenTTL        time.Duration
	HTTP            HTTPConfig
	CleanupInterval time.Duration
	TokenSecret     string
}

type HTTPConfig struct {
	Port int `json:"port"`
}

func MustLoadConfig() *Config {
	_ = godotenv.Load()

	path := fetchConfigPath()
	if path == "" {
		panic("config path is empty")
	}

	return MustLoadByPath(path)
}

func MustLoadByPath(path string) *Config {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		panic("config file does not exist: " + path)
	}

	file, err := os.Open(path)
	if err != nil {
		panic("failed to open config file: " + err.Error())
	}
	defer file.Close()

	var rawConfig struct {
		Env             string     `json:"env"`
		StoragePath     string     `json:"storage_path"`
		TokenTTL        string     `json:"token_ttl"`
		HTTP            HTTPConfig `json:"http"`
		CleanupInterval string     `json:"cleanup_interval"`
	}

	if err := json.NewDecoder(file).Decode(&rawConfig); err != nil {
		panic("failed to decode config: " + err.Error())
	}

	secret := os.Getenv("TOKEN_SECRET")
	if secret == "" {
		panic("TOKEN_SECRET is not set in environment or .env file")
	}

	return &Config{
		Env:             rawConfig.Env,
		StoragePath:     rawConfig.StoragePath,
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		TokenTTL:        parseDuration(rawConfig.TokenTTL, "token_ttl"),
		HTTP:            rawConfig.HTTP,
		CleanupInterval: parseDuration(rawConfig.CleanupInterval, "cleanup_interval"),
		TokenSecret:     secret,
	}
}

func parseDuration(val string, fieldName string) time.Duration {
	duration, err := time.ParseDuration(val)
	if err != nil {
		panic("invalid " + fieldName + " format: " + err.Error())
	}
	return duration
}

func fetchConfigPath() string {
	var res string

	if flag.Lookup("config") == nil {
		flag.StringVar(&res, "config", "", "Path to config file")
	}
	if !flag.Parsed() {
		flag.Parse()
	}
	if f := flag.Lookup("config"); f != nil && res == "" {
		res = f.Value.String()
	}

	if res == "" {
		res = os.Getenv("CONFIG_PATH")
	}

	return res
}
