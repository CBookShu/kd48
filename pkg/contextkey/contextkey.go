// Package contextkey 定义全局 context.Value 键，避免跨服务键冲突。
package contextkey

import "context"

// contextKey 是 context.Value 键的类型。
type contextKey string

const (
	// UserIDKey 是用户 ID 在 context 中的键。
	// 所有服务必须使用此键存取 user_id，确保类型安全。
	UserIDKey contextKey = "user_id"
)

// GetUserID 从 context 中获取用户 ID。
// 返回 user_id 和是否存在的标志。
func GetUserID(ctx context.Context) (uint32, bool) {
	v := ctx.Value(UserIDKey)
	if v == nil {
		return 0, false
	}
	userID, ok := v.(uint32)
	return userID, ok
}

// WithUserID 返回一个包含用户 ID 的新 context。
func WithUserID(ctx context.Context, userID uint32) context.Context {
	return context.WithValue(ctx, UserIDKey, userID)
}
