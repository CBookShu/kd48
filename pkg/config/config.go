package config

// Config 基础接口，所有生成的配置必须实现
type Config interface {
	// ConfigName 配置名称
	ConfigName() string

	// ConfigData 配置数据（返回切片指针，用于类型推导）
	ConfigData() any
}
