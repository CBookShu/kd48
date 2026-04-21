package csvparser

import (
	"strings"
	"testing"
)

func TestParser_Parse_ValidCSV(t *testing.T) {
	csv := `奖励说明,数量,标签
note,amount,tags
string,int32,string[]
首登奖,10,'vip'|'hot'`

	p := NewParser()
	sheet, err := p.Parse(strings.NewReader(csv), "TestConfig--test.csv")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if sheet.ConfigName != "TestConfig" {
		t.Errorf("ConfigName = %q, want %q", sheet.ConfigName, "TestConfig")
	}
	if len(sheet.Headers) != 3 {
		t.Errorf("len(Headers) = %d, want 3", len(sheet.Headers))
	}
	if len(sheet.Rows) != 1 {
		t.Errorf("len(Rows) = %d, want 1", len(sheet.Rows))
	}
}

func TestParser_Parse_ThreeRowHeader(t *testing.T) {
	csv := `说明,数量
desc,qty
string,int32
测试,100`

	p := NewParser()
	sheet, err := p.Parse(strings.NewReader(csv), "test.csv")
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if sheet.Headers[0].Description != "说明" {
		t.Errorf("Headers[0].Description = %q, want %q", sheet.Headers[0].Description, "说明")
	}
	if sheet.Headers[0].Name != "desc" {
		t.Errorf("Headers[0].Name = %q, want %q", sheet.Headers[0].Name, "desc")
	}
	if sheet.Headers[0].Type != "string" {
		t.Errorf("Headers[0].Type = %q, want %q", sheet.Headers[0].Type, "string")
	}
}

func TestParser_DeriveConfigName(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"CheckinDaily--2026-04-21.csv", "CheckinDaily"},
		{"RewardDemo--test.csv", "RewardDemo"},
		{"SimpleConfig.csv", "SimpleConfig"},
	}

	for _, tt := range tests {
		got := deriveConfigName(tt.filename)
		if got != tt.want {
			t.Errorf("deriveConfigName(%q) = %q, want %q", tt.filename, got, tt.want)
		}
	}
}
