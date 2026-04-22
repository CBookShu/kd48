package config

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
)

// ConfigLoader 从 MySQL 加载配置
type ConfigLoader struct {
	db    *sql.DB
	store *ConfigStore
}

// NewConfigLoader 创建加载器
func NewConfigLoader(db *sql.DB, store *ConfigStore) *ConfigLoader {
	return &ConfigLoader{db: db, store: store}
}

// LoadOne 加载单个配置
func (l *ConfigLoader) LoadOne(ctx context.Context, name string) error {
	// 1. 获取对应的 TypedStore
	ts := l.store.GetTypedStore(name)
	if ts == nil {
		return fmt.Errorf("config %s not registered", name)
	}

	// 2. 从 MySQL 读取最新版本
	query := `
		SELECT data, revision
		FROM lobby_config_revision
		WHERE config_name = ?
		ORDER BY revision DESC
		LIMIT 1
	`

	var data []byte
	var revision int64
	err := l.db.QueryRowContext(ctx, query, name).Scan(&data, &revision)
	if err != nil {
		return fmt.Errorf("query config %s: %w", name, err)
	}

	// 3. 解析 JSON 并更新 Store
	return l.parseAndUpdate(name, data, revision, ts)
}

// parseAndUpdate 解析 JSON 并更新 TypedStore
func (l *ConfigLoader) parseAndUpdate(name string, data []byte, revision int64, ts any) error {
	// 使用反射获取 TypedStore 的类型参数并解析
	storeValue := reflect.ValueOf(ts)
	if storeValue.Kind() != reflect.Ptr {
		return fmt.Errorf("typed store must be a pointer")
	}

	// 获取 TypedStore[T] 的类型参数 T
	storeType := storeValue.Elem().Type()
	// TypedStore 结构体中 snapshot 字段是 *Snapshot[T]
	snapshotField, ok := storeType.FieldByName("snapshot")
	if !ok {
		return fmt.Errorf("typed store missing snapshot field")
	}

	// snapshot 是 *Snapshot[T]，所以先去掉指针，再找 Data 字段
	snapshotType := snapshotField.Type
	if snapshotType.Kind() == reflect.Ptr {
		snapshotType = snapshotType.Elem()
	}

	dataField, ok := snapshotType.FieldByName("Data")
	if !ok {
		return fmt.Errorf("snapshot missing Data field")
	}

	// Data 是 []T，获取元素类型 T
	sliceType := dataField.Type
	if sliceType.Kind() != reflect.Slice {
		return fmt.Errorf("Data field must be a slice")
	}

	// 创建 []T 类型的变量来解析 JSON
	slicePtr := reflect.New(sliceType)
	if err := json.Unmarshal(data, slicePtr.Interface()); err != nil {
		return fmt.Errorf("parse config %s as %v: %w", name, sliceType, err)
	}

	// 调用 Update 方法
	updateMethod := storeValue.MethodByName("Update")
	if !updateMethod.IsValid() {
		return fmt.Errorf("typed store missing Update method")
	}

	// 准备参数：revision (int64), data ([]T)
	args := []reflect.Value{
		reflect.ValueOf(revision),
		slicePtr.Elem(),
	}
	updateMethod.Call(args)

	slog.Debug("config loaded", "name", name, "revision", revision)
	return nil
}

// LoadAll 加载所有已注册的配置
func (l *ConfigLoader) LoadAll(ctx context.Context) error {
	names := l.store.GetRegisteredNames()
	for _, name := range names {
		if err := l.LoadOne(ctx, name); err != nil {
			slog.Warn("failed to load config", "name", name, "error", err)
			// 继续加载其他配置
		}
	}
	return nil
}
