package csvparser

import (
	"testing"
)

func TestParseValue_Int32(t *testing.T) {
	v, err := ParseValue("42", "int32")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	if v.Parsed.(int32) != 42 {
		t.Errorf("Parsed = %v, want 42", v.Parsed)
	}
}

func TestParseValue_Int32Empty(t *testing.T) {
	v, err := ParseValue("", "int32")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	if !v.IsEmpty {
		t.Error("IsEmpty should be true")
	}
	if v.Parsed.(int32) != 0 {
		t.Errorf("Parsed = %v, want 0", v.Parsed)
	}
}

func TestParseValue_StringArray(t *testing.T) {
	v, err := ParseValue("'vip'|'hot'", "string[]")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	arr := v.Parsed.([]string)
	if len(arr) != 2 || arr[0] != "vip" || arr[1] != "hot" {
		t.Errorf("Parsed = %v, want [vip, hot]", arr)
	}
}

func TestParseValue_Int32Array(t *testing.T) {
	v, err := ParseValue("1|2|3", "int32[]")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	arr := v.Parsed.([]int32)
	if len(arr) != 3 || arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
		t.Errorf("Parsed = %v, want [1, 2, 3]", arr)
	}
}

func TestParseValue_Int32ArrayWithEmpty(t *testing.T) {
	v, err := ParseValue("1||3", "int32[]")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	arr := v.Parsed.([]int32)
	if len(arr) != 3 || arr[0] != 1 || arr[1] != 0 || arr[2] != 3 {
		t.Errorf("Parsed = %v, want [1, 0, 3]", arr)
	}
}

func TestParseValue_Map(t *testing.T) {
	v, err := ParseValue("32='15'|45='hello'", "int32=string")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	m := v.Parsed.(map[int32]string)
	if m[32] != "15" || m[45] != "hello" {
		t.Errorf("Parsed = %v, want {32: '15', 45: 'hello'}", m)
	}
}

func TestParseValue_Time(t *testing.T) {
	v, err := ParseValue("2026-04-15 10:00:00", "time")
	if err != nil {
		t.Fatalf("ParseValue() error = %v", err)
	}
	if v.Parsed.(string) != "2026-04-15 10:00:00" {
		t.Errorf("Parsed = %v, want '2026-04-15 10:00:00'", v.Parsed)
	}
}

func TestParseValue_TimeEmpty(t *testing.T) {
	_, err := ParseValue("", "time")
	if err == nil {
		t.Error("ParseValue() should error for empty time")
	}
}
