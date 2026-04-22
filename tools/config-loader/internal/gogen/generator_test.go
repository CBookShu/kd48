package gogen

import (
	"strings"
	"testing"

	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestGenerator_Generate(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Name: "note", Type: "string"},
			{Name: "amount", Type: "int32"},
			{Name: "tags", Type: "string[]"},
			{Name: "start_time", Type: "time"},
		},
		ConfigName: "Checkin",
	}

	g := NewGenerator()
	code, err := g.Generate(sheet, "lobbyconfig")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.Contains(code, "type CheckinRow struct") {
		t.Error("generated code should contain CheckinRow struct")
	}
	if !strings.Contains(code, "Note string") {
		t.Error("generated code should contain Note field")
	}
	if !strings.Contains(code, "Amount int32") {
		t.Error("generated code should contain Amount field")
	}
	if !strings.Contains(code, "Tags []string") {
		t.Error("generated code should contain Tags field")
	}
	if !strings.Contains(code, "StartTime ConfigTime") {
		t.Error("generated code should contain StartTime field with ConfigTime type")
	}
}

func TestGenerator_Generate_ConfigTime(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Name: "start_time", Type: "time"},
		},
		ConfigName: "Test",
	}

	g := NewGenerator()
	code, err := g.Generate(sheet, "lobbyconfig")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if !strings.Contains(code, "type ConfigTime struct") {
		t.Error("generated code should contain ConfigTime struct")
	}
	if !strings.Contains(code, "UnmarshalJSON") {
		t.Error("generated code should contain UnmarshalJSON method")
	}
}

func TestGenerate_PackageStruct(t *testing.T) {
	g := NewGenerator()

	sheet := &csvparser.Sheet{
		ConfigName: "test_config",
		Headers: []csvparser.ColumnHeader{
			{Name: "id", Type: "int32"},
			{Name: "name", Type: "string"},
		},
	}

	code, err := g.Generate(sheet, "testconfig")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// 检查 Package 结构体
	if !strings.Contains(code, "type Package struct") {
		t.Error("generated code missing Package struct")
	}

	// 检查 ConfigName 方法
	if !strings.Contains(code, "func (p *Package) ConfigName() string") {
		t.Error("generated code missing ConfigName method")
	}

	// 检查 ConfigData 方法
	if !strings.Contains(code, "func (p *Package) ConfigData() any") {
		t.Error("generated code missing ConfigData method")
	}

	// 检查 Store 变量
	if !strings.Contains(code, "var Store *") {
		t.Error("generated code missing Store variable")
	}

	// 检查 init 函数
	if !strings.Contains(code, "func init()") {
		t.Error("generated code missing init function")
	}

	// 检查 baseconfig 导入
	if !strings.Contains(code, `baseconfig "github.com/CBookShu/kd48/pkg/config"`) {
		t.Error("generated code missing baseconfig import")
	}

	// 检查 lobbyconfig 导入
	if !strings.Contains(code, `lobbyconfig "github.com/CBookShu/kd48/services/lobby/internal/config"`) {
		t.Error("generated code missing lobbyconfig import")
	}
}
