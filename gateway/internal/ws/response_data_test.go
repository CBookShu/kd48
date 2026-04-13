package ws

import (
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestDataFromWsHandlerResult_Nil(t *testing.T) {
	data, err := DataFromWsHandlerResult(nil)
	if err != nil || data != nil {
		t.Fatalf("want nil,nil got %v, %v", data, err)
	}
}

func TestDataFromWsHandlerResult_JSONBranch(t *testing.T) {
	data, err := DataFromWsHandlerResult(&WsHandlerResult{
		JSON: []byte(`{"success":true,"token":"abc"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	m, ok := data.(map[string]interface{})
	if !ok {
		t.Fatalf("want map, got %T", data)
	}
	if m["success"] != true {
		t.Fatalf("success: %v", m["success"])
	}
	if m["token"] != "abc" {
		t.Fatalf("token: %v", m["token"])
	}
}

func TestDataFromWsHandlerResult_JSONBranch_Invalid(t *testing.T) {
	_, err := DataFromWsHandlerResult(&WsHandlerResult{JSON: []byte(`not-json`)})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDataFromWsHandlerResult_ProtoBranch(t *testing.T) {
	data, err := DataFromWsHandlerResult(&WsHandlerResult{
		Message: &wrapperspb.StringValue{Value: "hello"},
	})
	if err != nil {
		t.Fatal(err)
	}
	// protojson 对 google.protobuf.StringValue 的 JSON 映射为裸字符串，非 {"value":"..."}
	s, ok := data.(string)
	if !ok || s != "hello" {
		t.Fatalf("want string hello, got %T %v", data, data)
	}
}

func TestDataFromWsHandlerResult_Empty(t *testing.T) {
	data, err := DataFromWsHandlerResult(&WsHandlerResult{})
	if err != nil || data != nil {
		t.Fatalf("want nil,nil got %v, %v", data, err)
	}
}
