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
	Log         LogConf         `mapstructure:"log"`
	Redis       RedisConf       `mapstructure:"redis"`
	Etcd        EtcdConf        `mapstructure:"etcd"`
	MySQL       MysqlConf       `mapstructure:"mysql"`
	Session     SessionConf     `mapstructure:"session"`
}

type ServerConf struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type GatewayConf struct {
	Port                     int    `mapstructure:"port"`
	MetaServiceTypesPrefix   string `mapstructure:"meta_service_types_prefix"`
	MetaGatewayRoutesPrefix  string `mapstructure:"meta_gateway_routes_prefix"`
}

type UserServiceConf struct {
	Port int `mapstructure:"port"`
}

// 🚨 修改点：增加 FilePath 字段
type LogConf struct {
	Level    string `mapstructure:"level"`
	FilePath string `mapstructure:"file_path"` // 例如: ./logs/app.log
}

type RedisConf struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type EtcdConf struct {
	Endpoints []string `mapstructure:"endpoints"`
}

type MysqlConf struct {
	DSN string `mapstructure:"dsn"`
}

type SessionConf struct {
	ExpireHours int64 `mapstructure:"expire_hours"`
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

	if c.Gateway.MetaServiceTypesPrefix == "" {
		c.Gateway.MetaServiceTypesPrefix = "kd48/meta/service-types/"
	}
	if c.Gateway.MetaGatewayRoutesPrefix == "" {
		c.Gateway.MetaGatewayRoutesPrefix = "kd48/meta/gateway-routes/"
	}

	return &c, nil
}
