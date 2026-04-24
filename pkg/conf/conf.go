package conf

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

// Config 全局配置结构体，后续所有的配置项都在这里扩展
type Config struct {
	Server       ServerConf       `mapstructure:"server"`
	Gateway      GatewayConf      `mapstructure:"gateway"`
	UserService  UserServiceConf  `mapstructure:"user_service"`
	LobbyService LobbyServiceConf `mapstructure:"lobby_service"`
	Log          LogConf          `mapstructure:"log"`
	Redis        RedisConf        `mapstructure:"redis"`
	Etcd         EtcdConf         `mapstructure:"etcd"`
	MySQL        MysqlConf        `mapstructure:"mysql"`
	Session      SessionConf      `mapstructure:"session"`
	DataSources  *DataSourcesConf `mapstructure:"data_sources"`
}

type ServerConf struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type GatewayConf struct {
	Port                    int    `mapstructure:"port"`
	MetaServiceTypesPrefix  string `mapstructure:"meta_service_types_prefix"`
	MetaGatewayRoutesPrefix string `mapstructure:"meta_gateway_routes_prefix"`
	StaticDir               string `mapstructure:"static_dir"` // 静态文件目录，如 "./web"
}

type UserServiceConf struct {
	Port int `mapstructure:"port"`
}

type LobbyServiceConf struct {
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

type DataSourcesConf struct {
	MySQLPools  map[string]MySQLPoolConf `mapstructure:"mysql_pools"`
	RedisPools  map[string]RedisPoolConf `mapstructure:"redis_pools"`
	MySQLRoutes []RouteRuleConf          `mapstructure:"mysql_routes"`
	RedisRoutes []RouteRuleConf          `mapstructure:"redis_routes"`
}

type MySQLPoolConf struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpen         int           `mapstructure:"max_open"`
	MaxIdle         int           `mapstructure:"max_idle"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisPoolConf struct {
	Addr         string        `mapstructure:"addr"`
	DB           int           `mapstructure:"db"`
	Password     string        `mapstructure:"password"`
	PoolSize     int           `mapstructure:"pool_size"`
	MinIdleConns int           `mapstructure:"min_idle_conns"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type RouteRuleConf struct {
	Prefix string `mapstructure:"prefix"`
	Pool   string `mapstructure:"pool"`
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

func (c *Config) GetDataSourcesOrSynthesize() *DataSourcesConf {
	if c.DataSources != nil {
		return c.DataSources
	}
	return &DataSourcesConf{
		MySQLPools: map[string]MySQLPoolConf{
			"default": {DSN: c.MySQL.DSN, MaxOpen: 20, MaxIdle: 5},
		},
		RedisPools: map[string]RedisPoolConf{
			"default": {Addr: c.Redis.Addr, DB: c.Redis.DB, Password: c.Redis.Password},
		},
		MySQLRoutes: []RouteRuleConf{{Prefix: "", Pool: "default"}},
		RedisRoutes: []RouteRuleConf{{Prefix: "", Pool: "default"}},
	}
}

func (d *DataSourcesConf) ToDSRouteConfig() *DSRouteConfig {
	if d == nil {
		return nil
	}
	mysqlPools := make(map[string]MySQLPoolSpec)
	for k, v := range d.MySQLPools {
		mysqlPools[k] = MySQLPoolSpec{
			DSN:             v.DSN,
			MaxOpen:         v.MaxOpen,
			MaxIdle:         v.MaxIdle,
			ConnMaxLifetime: v.ConnMaxLifetime,
			ConnMaxIdleTime: v.ConnMaxIdleTime,
		}
	}
	redisPools := make(map[string]RedisPoolSpec)
	for k, v := range d.RedisPools {
		redisPools[k] = RedisPoolSpec{
			Addr:         v.Addr,
			DB:           v.DB,
			Password:     v.Password,
			PoolSize:     v.PoolSize,
			MinIdleConns: v.MinIdleConns,
			DialTimeout:  v.DialTimeout,
			ReadTimeout:  v.ReadTimeout,
			WriteTimeout: v.WriteTimeout,
		}
	}
	mysqlRoutes := make([]RouteRuleSpec, len(d.MySQLRoutes))
	for i, r := range d.MySQLRoutes {
		mysqlRoutes[i] = RouteRuleSpec{Prefix: r.Prefix, Pool: r.Pool}
	}
	redisRoutes := make([]RouteRuleSpec, len(d.RedisRoutes))
	for i, r := range d.RedisRoutes {
		redisRoutes[i] = RouteRuleSpec{Prefix: r.Prefix, Pool: r.Pool}
	}
	return &DSRouteConfig{
		MySQLPools:  mysqlPools,
		RedisPools:  redisPools,
		MySQLRoutes: mysqlRoutes,
		RedisRoutes: redisRoutes,
	}
}

type DSRouteConfig struct {
	MySQLPools  map[string]MySQLPoolSpec
	RedisPools  map[string]RedisPoolSpec
	MySQLRoutes []RouteRuleSpec
	RedisRoutes []RouteRuleSpec
}

type MySQLPoolSpec struct {
	DSN             string
	MaxOpen         int
	MaxIdle         int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

type RedisPoolSpec struct {
	Addr         string
	DB           int
	Password     string
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type RouteRuleSpec struct {
	Prefix string
	Pool   string
}
