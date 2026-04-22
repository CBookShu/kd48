package config

import (
	"sort"
	"sync"

	baseconfig "github.com/CBookShu/kd48/pkg/config"
)

var (
	globalStore *ConfigStore
	once        sync.Once
)

// ConfigStore 管理所有配置
type ConfigStore struct {
	mu     sync.RWMutex
	stores map[string]any // name → *TypedStore[T]
}

// GetStore 获取全局 Store（单例）
func GetStore() *ConfigStore {
	once.Do(func() {
		globalStore = &ConfigStore{
			stores: make(map[string]any),
		}
	})
	return globalStore
}

// ResetStore 重置全局 Store（仅测试使用）
func ResetStore() {
	globalStore = &ConfigStore{
		stores: make(map[string]any),
	}
	once = sync.Once{}
}

// GetTypedStore 获取类型安全的 Store（内部使用）
func (cs *ConfigStore) GetTypedStore(name string) any {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.stores[name]
}

// GetRegisteredNames 获取所有已注册的配置名
func (cs *ConfigStore) GetRegisteredNames() []string {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	names := make([]string, 0, len(cs.stores))
	for name := range cs.stores {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Register 注册配置（init 中调用）
func Register[T any](pkg baseconfig.Config) *TypedStore[T] {
	cs := GetStore()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// 如果已存在，返回已有的 store
	if existing, ok := cs.stores[pkg.ConfigName()]; ok {
		return existing.(*TypedStore[T])
	}

	ts := NewTypedStore[T]()
	cs.stores[pkg.ConfigName()] = ts
	return ts
}
