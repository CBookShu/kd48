package dsroute

import "time"

type DataSourcesConfig struct {
	MySQLPools  map[string]MySQLPoolSpec `yaml:"mysql_pools" json:"mysql_pools"`
	RedisPools  map[string]RedisPoolSpec `yaml:"redis_pools" json:"redis_pools"`
	MySQLRoutes []RouteRule              `yaml:"mysql_routes" json:"mysql_routes"`
	RedisRoutes []RouteRule              `yaml:"redis_routes" json:"redis_routes"`
}

type MySQLPoolSpec struct {
	DSN             string        `yaml:"dsn" json:"dsn"`
	MaxOpen         int           `yaml:"max_open" json:"max_open"`
	MaxIdle         int           `yaml:"max_idle" json:"max_idle"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" json:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `yaml:"conn_max_idle_time" json:"conn_max_idle_time"`
}

type RedisPoolSpec struct {
	Addr         string        `yaml:"addr" json:"addr"`
	DB           int           `yaml:"db" json:"db"`
	Password     string        `yaml:"password" json:"password"`
	PoolSize     int           `yaml:"pool_size" json:"pool_size"`
	MinIdleConns int           `yaml:"min_idle_conns" json:"min_idle_conns"`
	DialTimeout  time.Duration `yaml:"dial_timeout" json:"dial_timeout"`
	ReadTimeout  time.Duration `yaml:"read_timeout" json:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" json:"write_timeout"`
}
