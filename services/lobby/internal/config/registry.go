package config

import (
	"fmt"
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
// 如果同名配置已存在且类型匹配，返回已有 store
// 如果类型不匹配，panic（这是编程错误，应在开发阶段发现）
func Register[T any](pkg baseconfig.Config) *TypedStore[T] {
	cs := GetStore()

	cs.mu.Lock()
	defer cs.mu.Unlock()

	name := pkg.ConfigName()

	// 如果已存在，检查类型并返回
	if existing, ok := cs.stores[name]; ok {
		// 安全类型断言，避免 panic
		if typedStore, ok := existing.(*TypedStore[T]); ok {
			return typedStore
		}
		// 类型不匹配，这是编程错误（init 阶段应尽早发现）
		panic(fmt.Sprintf("config %q already registered with different type", name))
	}

	ts := NewTypedStore[T]()
	cs.stores[name] = ts
	return ts
}
