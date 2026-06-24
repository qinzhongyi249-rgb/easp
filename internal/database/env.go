package database

import (
	"fmt"
	"os"
	"strconv"
)

// ConfigFromEnv 从环境变量读取数据库配置，避免在源码中硬编码敏感信息。
func ConfigFromEnv() (Config, error) {
	cfg := Config{
		Host:     os.Getenv("EASP_DB_HOST"),
		Port:     3306,
		User:     os.Getenv("EASP_DB_USER"),
		Password: os.Getenv("EASP_DB_PASSWORD"),
		Database: os.Getenv("EASP_DB_NAME"),
	}
	if port := os.Getenv("EASP_DB_PORT"); port != "" {
		parsed, err := strconv.Atoi(port)
		if err != nil {
			return cfg, fmt.Errorf("invalid EASP_DB_PORT: %w", err)
		}
		cfg.Port = parsed
	}
	missing := make([]string, 0, 4)
	if cfg.Host == "" {
		missing = append(missing, "EASP_DB_HOST")
	}
	if cfg.User == "" {
		missing = append(missing, "EASP_DB_USER")
	}
	if cfg.Password == "" {
		missing = append(missing, "EASP_DB_PASSWORD")
	}
	if cfg.Database == "" {
		missing = append(missing, "EASP_DB_NAME")
	}
	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing required environment variables: %v", missing)
	}
	return cfg, nil
}

// DSNFromConfig 生成 MySQL DSN。不要打印返回值，里面包含密码。
func DSNFromConfig(cfg Config) string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
}

// DSNFromEnv 从环境变量生成 MySQL DSN。不要打印返回值，里面包含密码。
func DSNFromEnv() (string, error) {
	cfg, err := ConfigFromEnv()
	if err != nil {
		return "", err
	}
	return DSNFromConfig(cfg), nil
}
