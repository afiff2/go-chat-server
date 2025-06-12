package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server ServerConfig `toml:"serverConfig"`
	Log    LogConfig    `toml:"log"`
	Mysql  MysqlConfig  `toml:"mysqlConfig"`
	Redis  RedisConfig  `toml:"redisConfig"`
}

type ServerConfig struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type LogConfig struct {
	Env   string `toml:"env"` // dev / prod
	Path  string `toml:"path"`
	Level string `toml:"level"` // debug / info / warn / error
}

type MysqlConfig struct {
	User         string `toml:"user"`
	Socket       string `toml:"socket"`
	DatabaseName string `toml:"databaseName"`
}

type RedisConfig struct {
	Host     string `toml:"host"`
	Port     int    `toml:"port"`
	Password string `toml:"password"`
	Db       int    `toml:"db"`
}

var config *Config

// LoadConfig 从指定路径加载配置文件
func LoadConfig(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Printf("配置文件不存在: %s\n", path)
		return err
	}

	config = &Config{}
	_, err := toml.DecodeFile(path, config)
	if err != nil {
		fmt.Printf("解析配置文件失败: %v\n", err)
		return err
	}
	return nil
}

// GetConfig 获取全局配置单例
func GetConfig() *Config {
	if config == nil {
		defaultPath := "/root/Project/go-chat-server/configs/config.toml"
		if err := LoadConfig(defaultPath); err != nil {
			fmt.Printf("加载默认配置失败: %v\n", err)
			os.Exit(1)
		}
	}
	return config
}
