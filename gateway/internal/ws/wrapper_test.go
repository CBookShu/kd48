package ws

import (
	"context"
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type stubIngressClient struct {
	lastRoute   string
	lastPayload []byte
	reply       *gatewayv1.IngressReply
	err         error
}

func (s *stubIngressClient) Call(ctx context.Context, in *gatewayv1.IngressRequest, opts ...grpc.CallOption) (*gatewayv1.IngressReply, error) {
	s.lastRoute = in.GetRoute()
	s.lastPayload = in.GetJsonPayload()
	return s.reply, s.err
}

func TestWrapIngress_ForwardsRouteAndPayload(t *testing.T) {
	stub := &stubIngressClient{
		reply: &gatewayv1.IngressReply{JsonPayload: []byte(`{"ok":true}`)},
	}
	h := WrapIngress(stub, "/user.v1.UserService/Login")
	payload := []byte(`{"username":"u","password":"p"}`)
	res, err := h(context.Background(), payload, &clientMeta{})
	if err != nil {
		t.Fatal(err)
	}
	if stub.lastRoute != "/user.v1.UserService/Login" {
		t.Fatalf("route: %q", stub.lastRoute)
	}
	if string(stub.lastPayload) != string(payload) {
		t.Fatalf("payload: %q", stub.lastPayload)
	}
	if res == nil || res.Message != nil {
		t.Fatalf("want JSON branch, got %+v", res)
	}
	if string(res.JSON) != `{"ok":true}` {
		t.Fatalf("json: %s", res.JSON)
	}
}

func TestWrapIngress_PropagatesError(t *testing.T) {
	stub := &stubIngressClient{err: status.Error(codes.PermissionDenied, "permission denied")}
	_, err := WrapIngress(stub, "/x")(context.Background(), []byte(`{}`), &clientMeta{})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestWrapIngress_InjectsUserIDIntoContext(t *testing.T) {
	stub := &stubIngressClient{
		reply: &gatewayv1.IngressReply{JsonPayload: []byte(`{}`)},
	}

	// 模拟已认证用户
	meta := &clientMeta{
		userID:          12345,
		isAuthenticated: true,
	}

	var gotUserID int64
	var ctxChecked bool

	// 创建一个包装器来检查 context
	h := WrapIngress(&contextCheckerClient{
		stubIngressClient: stub,
		checkCtx: func(ctx context.Context) {
			ctxChecked = true
			if v := ctx.Value("user_id"); v != nil {
				gotUserID = v.(int64)
			}
		},
	}, "/test")

	_, err := h(context.Background(), []byte(`{}`), meta)
	if err != nil {
		t.Fatal(err)
	}

	if !ctxChecked {
		t.Fatal("context was not checked")
	}

	if gotUserID != 12345 {
		t.Fatalf("want user_id=12345 in context, got %d", gotUserID)
	}
}

type contextCheckerClient struct {
	*stubIngressClient
	checkCtx func(ctx context.Context)
}

func (c *contextCheckerClient) Call(ctx context.Context, in *gatewayv1.IngressRequest, opts ...grpc.CallOption) (*gatewayv1.IngressReply, error) {
	if c.checkCtx != nil {
		c.checkCtx(ctx)
	}
	return c.stubIngressClient.Call(ctx, in, opts...)
}
