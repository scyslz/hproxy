package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Load 从 JSON 文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	cfg := &Config{
		// 默认值
		Interval: "30s",
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	cfg.ConfigFile = path

	// 处理相对路径：证书路径相对于 config.json 所在目录
	if cfg.ConfigFile != "" {
		configDir := filepath.Dir(cfg.ConfigFile)
		if !filepath.IsAbs(cfg.Cert) && cfg.Cert != "" {
			cfg.Cert = filepath.Join(configDir, cfg.Cert)
		}
		if !filepath.IsAbs(cfg.Key) && cfg.Key != "" {
			cfg.Key = filepath.Join(configDir, cfg.Key)
		}
	}

	return cfg, nil
}

// ReloadIfChanged 检查配置文件是否有改动，有则重新加载
// 返回 true 表示已重新加载
func (c *Config) ReloadIfChanged() bool {
	if c.ConfigFile == "" {
		return false
	}
	_, err := os.Stat(c.ConfigFile)
	if err != nil {
		return false
	}
	// TODO: 实现配置热重载（需要记录上次修改时间）
	return false
}
