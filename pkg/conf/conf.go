package conf

import (
	"fmt"

	"github.com/spf13/viper"
)

// Config 全局配置结构体，后续所有的配置项都在这里扩展
type Config struct {
	Server      ServerConf      `mapstructure:"server"`
	Gateway     GatewayConf     `mapstructure:"gateway"`
	UserService UserServiceConf `mapstructure:"user_service"`
}

type ServerConf struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type GatewayConf struct {
	Port int `mapstructure:"port"`
}

type UserServiceConf struct {
	Port int `mapstructure:"port"`
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config error: %w", err)
	}

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, fmt.Errorf("unmarshal config error: %w", err)
	}

	return &c, nil
}
