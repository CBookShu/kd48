package config

import "testing"

// MockConfig 用于测试接口合规性
type MockConfig struct {
	name string
	data any
}

func (m *MockConfig) ConfigName() string {
	return m.name
}

func (m *MockConfig) ConfigData() any {
	return m.data
}

func TestConfigInterface_Compliance(t *testing.T) {
	// 编译时检查 MockConfig 实现 Config 接口
	var _ Config = &MockConfig{}

	m := &MockConfig{name: "test_config", data: []int{1, 2, 3}}

	if got := m.ConfigName(); got != "test_config" {
		t.Errorf("ConfigName() = %v, want test_config", got)
	}

	data := m.ConfigData()
	if data == nil {
		t.Error("ConfigData() returned nil")
	}
}
