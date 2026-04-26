package ws

import (
	"testing"

	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestDataFromWsHandlerResult_SnakeCaseUint32(t *testing.T) {
	// 验证 protojson 输出 snake_case 和数字类型
	loginData := &userv1.LoginData{
		Token:  "test-token",
		UserId: 123,
	}

	result := &WsHandlerResult{
		Message: loginData,
	}

	data, err := DataFromWsHandlerResult(result)
	require.NoError(t, err)

	dataMap, ok := data.(map[string]interface{})
	require.True(t, ok, "result should be a map")

	// 验证 snake_case: user_id (不是 userId)
	assert.Contains(t, dataMap, "user_id", "field should be snake_case: user_id")
	assert.NotContains(t, dataMap, "userId", "field should NOT be camelCase: userId")

	// 验证数字类型: float64 (JSON unmarshal 将数字解析为 float64)
	userID, ok := dataMap["user_id"]
	require.True(t, ok)
	assert.IsType(t, float64(0), userID, "user_id should be number, not string")
	assert.Equal(t, float64(123), userID)
}
