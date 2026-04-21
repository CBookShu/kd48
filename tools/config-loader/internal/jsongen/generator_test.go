package jsongen

import (
	"encoding/json"
	"testing"

	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestGenerator_Generate(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Name: "note", Type: "string"},
			{Name: "amount", Type: "int32"},
		},
		Rows: []csvparser.Row{
			{Values: []csvparser.Value{
				{Raw: "首登奖", Parsed: "首登奖", Type: "string"},
				{Raw: "10", Parsed: int32(10), Type: "int32"},
			}},
		},
		ConfigName: "TestConfig",
	}

	g := NewGenerator()
	payload, err := g.Generate(sheet, 1)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if payload.ConfigName != "TestConfig" {
		t.Errorf("ConfigName = %q, want %q", payload.ConfigName, "TestConfig")
	}
	if payload.Revision != 1 {
		t.Errorf("Revision = %d, want 1", payload.Revision)
	}
	if len(payload.Data) != 1 {
		t.Fatalf("len(Data) = %d, want 1", len(payload.Data))
	}

	data := payload.Data[0]
	if data["note"] != "首登奖" {
		t.Errorf("data[note] = %v, want '首登奖'", data["note"])
	}
	if data["amount"] != int32(10) {
		t.Errorf("data[amount] = %v, want 10", data["amount"])
	}
}

func TestGenerator_Generate_JSON(t *testing.T) {
	sheet := &csvparser.Sheet{
		Headers: []csvparser.ColumnHeader{
			{Name: "tags", Type: "string[]"},
		},
		Rows: []csvparser.Row{
			{Values: []csvparser.Value{
				{Raw: "'vip'|'hot'", Parsed: []string{"vip", "hot"}, Type: "string[]"},
			}},
		},
		ConfigName: "Test",
	}

	g := NewGenerator()
	payload, err := g.Generate(sheet, 1)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var result map[string]interface{}
	json.Unmarshal(bytes, &result)

	data := result["data"].([]interface{})[0].(map[string]interface{})
	tags := data["tags"].([]interface{})
	if len(tags) != 2 {
		t.Errorf("len(tags) = %d, want 2", len(tags))
	}
}
