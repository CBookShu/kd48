package validator

import (
	"testing"

	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestValidator_Validate_ValidSheet(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Description: "说明", Name: "note", Type: "string"},
			{Description: "数量", Name: "amount", Type: "int32"},
		},
		Rows: []csvparser.Row{
			{Values: []csvparser.Value{
				{Raw: "测试", Type: "string", IsEmpty: false},
				{Raw: "100", Type: "int32", IsEmpty: false},
			}},
		},
	}

	v := NewValidator()
	err := v.Validate(sheet)
	if err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestValidator_Validate_DuplicateColumnName(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Description: "说明", Name: "note", Type: "string"},
			{Description: "说明2", Name: "note", Type: "int32"},
		},
		Rows: []csvparser.Row{},
	}

	v := NewValidator()
	err := v.Validate(sheet)
	if err == nil {
		t.Fatal("Validate() should error for duplicate column names")
	}
}

func TestValidator_Validate_InvalidColumnName(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Description: "说明", Name: "InvalidName", Type: "string"},
		},
		Rows: []csvparser.Row{},
	}

	v := NewValidator()
	err := v.Validate(sheet)
	if err == nil {
		t.Fatal("Validate() should error for invalid column name (not snake_case)")
	}
}
