package ws

import (
	"encoding/json"

	"google.golang.org/protobuf/encoding/protojson"
)

// DataFromWsHandlerResult 将成功分支的 WsHandlerResult 转为发给前端的 JSON 友好结构（map/slice/标量）。
// resp 为 nil 时返回 (nil, nil)；Message 与 JSON 皆空时返回 (nil, nil)。
func DataFromWsHandlerResult(resp *WsHandlerResult) (interface{}, error) {
	if resp == nil {
		return nil, nil
	}
	if resp.Message != nil {
		marshaler := protojson.MarshalOptions{
			EmitUnpopulated: true,
			UseProtoNames:   true, // 使用 proto 字段名（snake_case）
		}
		jsonBytes, err := marshaler.Marshal(resp.Message)
		if err != nil {
			return nil, err
		}
		var data interface{}
		if err := json.Unmarshal(jsonBytes, &data); err != nil {
			return nil, err
		}
		return data, nil
	}
	if len(resp.JSON) > 0 {
		var data interface{}
		if err := json.Unmarshal(resp.JSON, &data); err != nil {
			return nil, err
		}
		return data, nil
	}
	return nil, nil
}
