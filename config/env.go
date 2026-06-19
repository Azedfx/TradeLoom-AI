package config

import (
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	BitgetAPIKey    string `envconfig:"BITGET_API_KEY"`
	BitgetSecretKey string `envconfig:"BITGET_SECRET_KEY"`
	AlibabaCloudKey string `envconfig:"ALIBABA_CLOUD_KEY"`
	LLMBaseURL      string `envconfig:"LLM_BASE_URL" default:"https://hackathon.bitgetops.com/v1/chat/completions"`
	LLMModel        string `envconfig:"LLM_MODEL" default:"qwen3.6-plus"`
	TradeMode       string `envconfig:"TRADE_MODE" default:"demo"`
	DefaultSymbol   string `envconfig:"DEFAULT_SYMBOL" default:"BTCUSDT"`
	Port            string `envconfig:"PORT" default:":8040"`
}

func (c *Config) LLMApiKey() string {
	if c.AlibabaCloudKey != "" {
		return c.AlibabaCloudKey
	}
	return ""
}

func Load() (*Config, error) {
	var cfg Config
	godotenv.Load()
	err := envconfig.Process("", &cfg)
	return &cfg, err
}
