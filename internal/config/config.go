package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// Alipay
	AlipayAppID      string `yaml:"alipay_app_id"`
	AlipayPrivateKey string `yaml:"alipay_private_key"`
	AlipayPublicKey  string `yaml:"alipay_public_key"`
	AlipaySandbox    bool   `yaml:"alipay_sandbox"`
	AlipayPreCreate  bool   `yaml:"alipay_precreate"`

	// VMQ
	VMQBaseURL   string `yaml:"vmq_base_url"`
	VMQKey       string `yaml:"vmq_key"`
	VMQDeviceKey string `yaml:"vmq_device_key"`

	// Epay
	EpayMerchantID  string `yaml:"epay_merchant_id"`
	EpayMerchantKey string `yaml:"epay_merchant_key"`

	// Database
	DatabaseDriver string `yaml:"database_driver"` // "postgres" or "sqlite"
	DatabaseURL    string `yaml:"database_url"`

	// Service
	PublicBaseURL    string `yaml:"public_base_url"`
	ListenAddr      string `yaml:"listen_addr"`
	VMQOrderTimeout int    `yaml:"vmq_order_timeout"` // minutes

	// Poller
	PollInterval int `yaml:"poll_interval"` // seconds
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:      ":8081",
		VMQOrderTimeout: 5,
		PollInterval:    30,
		DatabaseDriver:  "sqlite",
		DatabaseURL:     "alipay_vmq.db",
	}

	if path := getEnvOrDefault("CONFIG_FILE", "config.yaml"); path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if err := yaml.Unmarshal(data, cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
		}
	}

	applyEnv(cfg)

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("ALIPAY_APP_ID"); v != "" {
		cfg.AlipayAppID = v
	}
	if v := os.Getenv("ALIPAY_PRIVATE_KEY"); v != "" {
		cfg.AlipayPrivateKey = v
	}
	if v := os.Getenv("ALIPAY_PRIVATE_KEY_PATH"); v != "" {
		if data, err := os.ReadFile(v); err == nil {
			cfg.AlipayPrivateKey = strings.TrimSpace(string(data))
		}
	}
	if v := os.Getenv("ALIPAY_PUBLIC_KEY"); v != "" {
		cfg.AlipayPublicKey = v
	}
	if v := os.Getenv("VMQ_BASE_URL"); v != "" {
		cfg.VMQBaseURL = v
	}
	if v := os.Getenv("VMQ_KEY"); v != "" {
		cfg.VMQKey = v
	}
	if v := os.Getenv("VMQ_DEVICE_KEY"); v != "" {
		cfg.VMQDeviceKey = v
	}
	if v := os.Getenv("EPAY_MERCHANT_ID"); v != "" {
		cfg.EpayMerchantID = v
	}
	if v := os.Getenv("EPAY_MERCHANT_KEY"); v != "" {
		cfg.EpayMerchantKey = v
	}
	if v := os.Getenv("DATABASE_DRIVER"); v != "" {
		cfg.DatabaseDriver = v
	}
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.DatabaseURL = v
	}
	if v := os.Getenv("PUBLIC_BASE_URL"); v != "" {
		cfg.PublicBaseURL = v
	}
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("VMQ_ORDER_TIMEOUT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.VMQOrderTimeout = n
		}
	}
	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.PollInterval = n
		}
	}
	if v := os.Getenv("ALIPAY_SANDBOX"); v != "" {
		cfg.AlipaySandbox = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("ALIPAY_PRECREATE"); v != "" {
		cfg.AlipayPreCreate = strings.EqualFold(v, "true") || v == "1"
	}
}

func validate(cfg *Config) error {
	if cfg.AlipayAppID == "" {
		return fmt.Errorf("ALIPAY_APP_ID is required")
	}
	if cfg.AlipayPrivateKey == "" {
		return fmt.Errorf("ALIPAY_PRIVATE_KEY or ALIPAY_PRIVATE_KEY_PATH is required")
	}
	if cfg.AlipayPublicKey == "" {
		return fmt.Errorf("ALIPAY_PUBLIC_KEY is required")
	}
	if cfg.VMQBaseURL == "" {
		return fmt.Errorf("VMQ_BASE_URL is required")
	}
	if cfg.VMQKey == "" {
		return fmt.Errorf("VMQ_KEY is required")
	}
	if cfg.VMQDeviceKey == "" {
		return fmt.Errorf("VMQ_DEVICE_KEY is required")
	}
	if cfg.EpayMerchantID == "" {
		return fmt.Errorf("EPAY_MERCHANT_ID is required")
	}
	if cfg.EpayMerchantKey == "" {
		return fmt.Errorf("EPAY_MERCHANT_KEY is required")
	}
	if cfg.PublicBaseURL == "" {
		return fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	return nil
}

func getEnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
