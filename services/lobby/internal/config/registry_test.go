package config

import (
	"testing"
)

// testConfig 实现 baseconfig.Config 接口
type testConfig struct {
	name string
}

func (t *testConfig) ConfigName() string {
	return t.name
}

func (t *testConfig) ConfigData() any {
	return &[]int{}
}

func TestRegister_CreatesTypedStore(t *testing.T) {
	// 重置全局状态
	ResetStore()

	pkg := &testConfig{name: "test_config"}
	store := Register[int](pkg)

	if store == nil {
		t.Fatal("Register() returned nil")
	}

	// 验证可以通过 GetTypedStore 获取
	cs := GetStore()
	ts := cs.GetTypedStore("test_config")
	if ts == nil {
		t.Error("GetTypedStore() returned nil for registered config")
	}
}

func TestGetStore_Singleton(t *testing.T) {
	ResetStore()

	s1 := GetStore()
	s2 := GetStore()

	if s1 != s2 {
		t.Error("GetStore() returned different instances")
	}
}

func TestRegister_SameNameIdempotent(t *testing.T) {
	ResetStore()

	pkg := &testConfig{name: "same_name"}

	s1 := Register[int](pkg)
	s2 := Register[int](pkg)

	// 两次注册应该返回同一个 store
	if s1 != s2 {
		t.Error("Register() with same name returned different stores")
	}
}

func TestGetTypedStore_NotFound(t *testing.T) {
	ResetStore()

	cs := GetStore()
	ts := cs.GetTypedStore("non_existent")

	if ts != nil {
		t.Error("GetTypedStore() for non-existent config should return nil")
	}
}
