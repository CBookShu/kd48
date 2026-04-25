package main

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	gatewayv1 "github.com/CBookShu/kd48/api/proto/gateway/v1"
	userv1 "github.com/CBookShu/kd48/api/proto/user/v1"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/encoding/protojson"
)

// === Helper Functions ===

func setupIngressTest(t *testing.T, inner userLoginRegister) (gatewayv1.GatewayIngressClient, func()) {
	t.Helper()
	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(inner, nil))
	go func() { _ = s.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	cli := gatewayv1.NewGatewayIngressClient(conn)
	return cli, func() {
		_ = conn.Close()
		s.Stop()
	}
}

// setupIngressTestWithRedis creates an ingress server with Redis for token verification tests
func setupIngressTestWithRedis(t *testing.T, inner userLoginRegister) (gatewayv1.GatewayIngressClient, *miniredis.Miniredis, func()) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	const bufSize = 1024 * 1024
	lis := bufconn.Listen(bufSize)
	s := grpc.NewServer()
	gatewayv1.RegisterGatewayIngressServer(s, newIngressServer(inner, rdb))
	go func() { _ = s.Serve(lis) }()

	ctx := context.Background()
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) { return lis.Dial() }),
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	cli := gatewayv1.NewGatewayIngressClient(conn)
	return cli, mr, func() {
		_ = conn.Close()
		s.Stop()
		rdb.Close()
	}
}

// === Mock Implementations ===

type mockUserLR struct {
	loginErr     error
	registerErr  error
	verifyErr    error
	loginResp    *userv1.LoginData
	registerResp *userv1.RegisterData
	verifyResp   *userv1.VerifyTokenData
}

func (m mockUserLR) Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginData, error) {
	if m.loginErr != nil {
		return nil, m.loginErr
	}
	if m.loginResp != nil {
		return m.loginResp, nil
	}
	return &userv1.LoginData{Token: "test-token", UserId: 123}, nil
}

func (m mockUserLR) Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterData, error) {
	if m.registerErr != nil {
		return nil, m.registerErr
	}
	if m.registerResp != nil {
		return m.registerResp, nil
	}
	return &userv1.RegisterData{Token: "test-token", UserId: 123}, nil
}

func (m mockUserLR) VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenData, error) {
	if m.verifyErr != nil {
		return nil, m.verifyErr
	}
	if m.verifyResp != nil {
		return m.verifyResp, nil
	}
	return &userv1.VerifyTokenData{
		UserId:   123,
		Username: "testuser",
	}, nil
}

// === Unknown Route Tests ===

func TestIngressServer_Call_UnknownRoute(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/unknown/Method",
		JsonPayload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "unknown ingress route")
}

// === Invalid JSON Tests ===

func TestIngressServer_Call_Login_InvalidJSON(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`invalid json`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "invalid json")
}

func TestIngressServer_Call_Register_InvalidJSON(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{invalid}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "invalid json")
}

// === Service Not Configured Tests ===

func TestIngressServer_Call_Login_ServiceNotConfigured(t *testing.T) {
	cli, cleanup := setupIngressTest(t, nil)
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "not configured")
}

func TestIngressServer_Call_Register_ServiceNotConfigured(t *testing.T) {
	cli, cleanup := setupIngressTest(t, nil)
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "not configured")
}

// === Login Tests ===

func TestIngressServer_Call_Login_Success(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	reply, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`{"username":"testuser","password":"testpass"}`),
	})

	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(reply.GetJsonPayload(), &got))
	assert.Equal(t, "test-token", got["token"])
	assert.Equal(t, "123", got["userId"]) // protojson serializes uint64 as string
}

func TestIngressServer_Call_Login_Unauthenticated(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{
		loginErr: status.Error(codes.Unauthenticated, "invalid username or password"),
	})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`{"username":"wronguser","password":"wrongpass"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "invalid username or password")
}

func TestIngressServer_Call_Login_InternalError(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{
		loginErr: status.Error(codes.Internal, "internal server error"),
	})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Login",
		JsonPayload: []byte(`{"username":"testuser","password":"testpass"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// === Register Tests ===

func TestIngressServer_Call_Register_Success(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	reply, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{"username":"newuser","password":"newpass"}`),
	})

	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(reply.GetJsonPayload(), &got))
	assert.Equal(t, "test-token", got["token"])
	assert.Equal(t, "123", got["userId"]) // protojson serializes uint64 as string
}

func TestIngressServer_Call_Register_AlreadyExists(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{
		registerErr: status.Error(codes.AlreadyExists, "username already exists"),
	})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{"username":"existinguser","password":"pass"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.AlreadyExists, status.Code(err))
	assert.Contains(t, err.Error(), "username already exists")
}

func TestIngressServer_Call_Register_InvalidArgument(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{
		registerErr: status.Error(codes.InvalidArgument, "username and password required"),
	})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{"username":"","password":""}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "username and password required")
}

func TestIngressServer_Call_Register_InternalError(t *testing.T) {
	cli, cleanup := setupIngressTest(t, mockUserLR{
		registerErr: status.Error(codes.Internal, "internal server error"),
	})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/Register",
		JsonPayload: []byte(`{"username":"testuser","password":"testpass"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
}

// === VerifyToken Tests ===

// Note: TestIngressServer_Call_VerifyToken_Success_WithUserIDInContext is tested
// at the gateway level in gateway/internal/ws/wrapper_test.go where context values
// can be properly injected through the clientMeta.

func TestIngressServer_Call_VerifyToken_Success_FromRedis(t *testing.T) {
	cli, mr, cleanup := setupIngressTestWithRedis(t, mockUserLR{})
	defer cleanup()

	// Set up session in Redis
	token := "valid-token-12345"
	sessionKey := "user:session:" + token
	mr.Set(sessionKey, "456:testuser")

	reply, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`{"token":"` + token + `"}`),
	})

	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(reply.GetJsonPayload(), &got))
	assert.Equal(t, "testuser", got["username"])
	assert.Equal(t, "123", got["userId"]) // protojson serializes uint64 as string
}

func TestIngressServer_Call_VerifyToken_MissingToken(t *testing.T) {
	cli, _, cleanup := setupIngressTestWithRedis(t, mockUserLR{})
	defer cleanup()

	// No token in payload and no user_id in context
	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "token required")
}

func TestIngressServer_Call_VerifyToken_InvalidToken(t *testing.T) {
	cli, _, cleanup := setupIngressTestWithRedis(t, mockUserLR{})
	defer cleanup()

	// Token doesn't exist in Redis
	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`{"token":"invalid-token"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
	assert.Contains(t, err.Error(), "invalid or expired token")
}

func TestIngressServer_Call_VerifyToken_InvalidJSON(t *testing.T) {
	cli, _, cleanup := setupIngressTestWithRedis(t, mockUserLR{})
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`invalid json`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
	assert.Contains(t, err.Error(), "invalid json")
}

func TestIngressServer_Call_VerifyToken_ServiceNotConfigured(t *testing.T) {
	cli, cleanup := setupIngressTest(t, nil)
	defer cleanup()

	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`{}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "not configured")
}

func TestIngressServer_Call_VerifyToken_RedisNotConfigured(t *testing.T) {
	// Test when inner is configured but Redis is nil
	cli, cleanup := setupIngressTest(t, mockUserLR{})
	defer cleanup()

	// No user_id in context and no Redis - should fail
	_, err := cli.Call(context.Background(), &gatewayv1.IngressRequest{
		Route:       "/user.v1.UserService/VerifyToken",
		JsonPayload: []byte(`{"token":"some-token"}`),
	})

	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "redis not configured")
}

// === Protojson Token Field Tests ===
// These tests verify that protojson correctly handles the token field in VerifyTokenRequest.
// They catch the bug where unknown fields are rejected by protojson.Unmarshal.

func TestProtojson_VerifyTokenRequest_TokenField(t *testing.T) {
	// This test verifies that the token field is properly defined in the proto
	// and can be marshaled/unmarshaled by protojson.
	// This would have caught the original bug where protojson rejected the token field.

	tests := []struct {
		name    string
		json    string
		wantErr bool
		wantVal string
	}{
		{
			name:    "valid token field",
			json:    `{"token":"my-test-token"}`,
			wantErr: false,
			wantVal: "my-test-token",
		},
		{
			name:    "empty token",
			json:    `{"token":""}`,
			wantErr: false,
			wantVal: "",
		},
		{
			name:    "no token field",
			json:    `{}`,
			wantErr: false,
			wantVal: "",
		},
		{
			name:    "token with special characters",
			json:    `{"token":"abc-123_xyz.ABC"}`,
			wantErr: false,
			wantVal: "abc-123_xyz.ABC",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req userv1.VerifyTokenRequest
			err := protojson.Unmarshal([]byte(tt.json), &req)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "protojson should accept the token field - if this fails, regenerate proto with ./gen_proto.sh")
				assert.Equal(t, tt.wantVal, req.GetToken())
			}
		})
	}
}

func TestProtojson_VerifyTokenRequest_RejectsUnknownFields(t *testing.T) {
	// This test verifies that protojson still rejects truly unknown fields.
	// This documents the behavior that caused the original bug.

	var req userv1.VerifyTokenRequest
	err := protojson.Unmarshal([]byte(`{"unknownField":"value"}`), &req)

	// protojson by default rejects unknown fields
	require.Error(t, err, "protojson should reject unknown fields")
	assert.Contains(t, err.Error(), "unknown field")
}

func TestProtojson_VerifyTokenRequest_RoundTrip(t *testing.T) {
	// Test that we can marshal and unmarshal VerifyTokenRequest correctly

	original := &userv1.VerifyTokenRequest{
		Token: "test-token-for-roundtrip",
	}

	// Marshal to JSON
	jsonBytes, err := protojson.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var parsed userv1.VerifyTokenRequest
	err = protojson.Unmarshal(jsonBytes, &parsed)
	require.NoError(t, err)

	assert.Equal(t, original.GetToken(), parsed.GetToken())
}
