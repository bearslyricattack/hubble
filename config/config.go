package config

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	HTTP struct {
		Addr string `yaml:"addr"`
	} `yaml:"http"`
	Hubble struct {
		Addr string `yaml:"addr"`
	} `yaml:"hubble"`
	Redis struct {
		Addr     string `yaml:"addr"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
}

// LoadConfig 从YAML配置文件加载配置
func LoadConfig(configPath string) (Config, error) {
	// 初始化默认配置
	config := Config{}
	config.HTTP.Addr = ":8080"
	config.Hubble.Addr = "localhost:4245"
	config.Redis.Addr = "localhost:6379"
	config.Redis.Password = ""
	config.Redis.DB = 0

	// 如果未指定配置文件路径，则使用默认值
	if configPath == "" {
		log.Println("未指定配置文件，使用默认配置")
		return config, nil
	}

	// 读取配置文件
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("读取配置文件失败: %w", err)
	}

	// 解析YAML
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("解析配置文件失败: %w", err)
	}

	log.Printf("成功从 %s 加载配置", configPath)
	return config, nil
}
