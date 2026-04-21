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
