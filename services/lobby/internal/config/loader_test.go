package config

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestLoadOne_Success(t *testing.T) {
	ResetStore()

	// 注册配置
	pkg := &testConfig{name: "test_config"}
	Register[int](pkg)

	// 创建 mock 数据库
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟查询返回
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`[1, 2, 3]`, 42)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("test_config").
		WillReturnRows(rows)

	// 执行加载
	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadOne(context.Background(), "test_config")
	if err != nil {
		t.Fatalf("LoadOne() error = %v", err)
	}

	// 验证数据已加载
	ts := GetStore().GetTypedStore("test_config").(*TypedStore[int])
	snap := ts.Get()
	if snap == nil {
		t.Fatal("snapshot is nil after LoadOne")
	}
	if snap.Revision != 42 {
		t.Errorf("Revision = %d, want 42", snap.Revision)
	}
	if len(snap.Data) != 3 {
		t.Errorf("Data length = %d, want 3", len(snap.Data))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestLoadOne_NotFound(t *testing.T) {
	ResetStore()

	pkg := &testConfig{name: "missing_config"}
	Register[int](pkg)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟查询无结果
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("missing_config").
		WillReturnError(sql.ErrNoRows)

	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadOne(context.Background(), "missing_config")
	if err == nil {
		t.Error("LoadOne() should return error for missing config")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestLoadOne_NotRegistered(t *testing.T) {
	ResetStore()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadOne(context.Background(), "unregistered_config")
	if err == nil {
		t.Error("LoadOne() should return error for unregistered config")
	}
}

func TestLoadAll_PartialFailure(t *testing.T) {
	ResetStore()

	// 注册两个配置（按字母顺序，bad_config 在前）
	Register[int](&testConfig{name: "bad_config"})
	Register[int](&testConfig{name: "good_config"})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// bad_config 失败（按字母顺序先执行）
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("bad_config").
		WillReturnError(sql.ErrNoRows)

	// good_config 成功
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`[1, 2]`, 1)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("good_config").
		WillReturnRows(rows)

	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadAll(context.Background())
	// LoadAll 不应该返回错误，即使部分失败
	if err != nil {
		t.Errorf("LoadAll() error = %v, want nil", err)
	}

	// good_config 应该加载成功
	ts := GetStore().GetTypedStore("good_config").(*TypedStore[int])
	if ts.Get() == nil {
		t.Error("good_config should be loaded")
	}

	// bad_config 应该为 nil
	ts2 := GetStore().GetTypedStore("bad_config").(*TypedStore[int])
	if ts2.Get() != nil {
		t.Error("bad_config should not be loaded")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestLoadOne_InvalidJSON(t *testing.T) {
	ResetStore()

	pkg := &testConfig{name: "invalid_json_config"}
	Register[int](pkg)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	// 模拟返回无效 JSON
	rows := sqlmock.NewRows([]string{"data", "revision"}).
		AddRow(`not valid json`, 1)
	mock.ExpectQuery("SELECT data, revision FROM lobby_config_revision").
		WithArgs("invalid_json_config").
		WillReturnRows(rows)

	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadOne(context.Background(), "invalid_json_config")
	if err == nil {
		t.Error("LoadOne() should return error for invalid JSON")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestLoadAll_EmptyRegistry(t *testing.T) {
	ResetStore()

	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	defer db.Close()

	loader := NewConfigLoader(db, GetStore())
	err = loader.LoadAll(context.Background())
	if err != nil {
		t.Errorf("LoadAll() on empty registry error = %v, want nil", err)
	}
}
