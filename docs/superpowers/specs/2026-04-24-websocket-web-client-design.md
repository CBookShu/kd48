# WebSocket Web Client Design

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Convert the web client from HTTP REST API to WebSocket for all backend communication.

**Architecture:** Single WebSocket connection with promise-based API wrapper. The web client establishes one WebSocket connection to the gateway and uses it for all API calls (login, register, checkin, items). Message correlation is handled by matching request `method` to response `method`.

**Tech Stack:** JavaScript (ES6+), WebSocket API, Promise-based async/await

---

## Problem Statement

The current web client (`web/js/api.js`) uses HTTP REST API:
```javascript
fetch('http://localhost:8080/api/user.v1.UserService/Login', {...})
```

But the gateway only provides:
- `/health` - health check
- `/ws` - WebSocket endpoint for gRPC calls
- `/web/*` - static file serving

There is no `/api/*` HTTP REST endpoint. The web client needs to use WebSocket for all communication.

**Additional Issue:** The gateway's `WrapIngress` function does not inject `user_id` into the context, but the lobby service expects `ctx.Value("user_id")` for authenticated requests. This must be fixed.

---

## Solution: WebSocket API Client

### Architecture Overview

```
┌─────────────────────────────────────────────────────────┐
│  Web Browser                                            │
│  ┌─────────────────────────────────────────────────┐   │
│  │  api.js (WsClient)                              │   │
│  │  - connect() → WebSocket connection             │   │
│  │  - call(method, payload) → Promise              │   │
│  │  - Message correlation by method field          │   │
│  └─────────────────────────────────────────────────┘   │
│           │                                              │
│  ┌────────┴───────┬───────────┬───────────┐            │
│  │  login.html    │ checkin.html │ items.html │        │
│  │  register.html │              │           │         │
│  └────────────────┴───────────┴───────────┘            │
└─────────────────────────────────────────────────────────┘
                         │ WebSocket
                         ▼
┌─────────────────────────────────────────────────────────┐
│  Gateway (:8080)                                        │
│  /ws → WebSocket Handler → Route → Backend gRPC        │
└─────────────────────────────────────────────────────────┘
```

### Message Protocol

**Request Format:**
```json
{
  "method": "/user.v1.UserService/Login",
  "payload": "{\"username\":\"alice\",\"password\":\"secret\"}"
}
```

**Response Format:**
```json
{
  "method": "/user.v1.UserService/Login",
  "code": 0,
  "msg": "success",
  "data": {"success": true, "token": "abc...", "user_id": 123}
}
```

**Error Response:**
```json
{
  "method": "/user.v1.UserService/Login",
  "code": 16,
  "msg": "invalid username or password",
  "data": null
}
```

### WsClient Class

```javascript
class WsClient {
  constructor(url)
  connect()                    // Establish WebSocket connection
  disconnect()                 // Close connection
  isConnected()                // Check connection state
  call(method, payload)        // Generic API call, returns Promise

  // Convenience methods
  login(username, password)
  register(username, password)
  getCheckinStatus()
  checkin()
  getMyItems()

  // Token management
  saveToken(token)
  getToken()
  clearToken()
}
```

### Connection Flow

**Unauthenticated Pages (login.html, register.html):**
1. Page loads
2. Create WsClient instance
3. Call `connect()` to establish WebSocket
4. Call `login()` or `register()`
5. On success, save token and redirect

**Authenticated Pages (checkin.html, items.html, index.html):**
1. Page loads
2. Check for existing token
3. If no token, redirect to login
4. Create WsClient instance
5. Call `connect()` to establish WebSocket
6. Make authenticated API calls

### Error Handling

| Code | Meaning | Action |
|------|---------|--------|
| 0 | Success | Process data |
| 16 | Unauthenticated | Redirect to login |
| 3 | InvalidArgument | Show error message |
| 6 | AlreadyExists | Show "username exists" |
| Other | Other error | Show error message |

| Scenario | Behavior |
|----------|----------|
| Connection failed | Show error message with retry button |
| Request timeout (5s) | Show "请求超时" error |
| WebSocket closed | Show "连接断开" error |

---

## Backend Changes

### 1. `gateway/internal/ws/wrapper.go` (Modify)

**Problem:** `WrapIngress` does not inject `user_id` into context, but backend services expect `ctx.Value("user_id")`.

**Fix:** Update `WrapIngress` to inject `meta.userID` into context when user is authenticated.

```go
func WrapIngress(cli gatewayv1.GatewayIngressClient, route string) WsHandlerFunc {
    return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
        // Inject user_id into context if authenticated
        if meta.userID > 0 {
            ctx = context.WithValue(ctx, "user_id", meta.userID)
        }

        reply, err := cli.Call(ctx, &gatewayv1.IngressRequest{
            Route:       route,
            JsonPayload: payload,
        })
        if err != nil {
            return nil, err
        }
        return &WsHandlerResult{JSON: reply.GetJsonPayload()}, nil
    }
}
```

---

## Frontend Changes

### 1. `web/js/api.js` (Rewrite)

**New Implementation:**
- WsClient class with WebSocket connection management
- Promise-based `call()` method for request/response correlation
- Convenience methods: `login()`, `register()`, `checkin()`, `getCheckinStatus()`, `getMyItems()`
- Token storage in localStorage
- Connection state management

### 2. `web/login.html` (Modify)

**Changes:**
- Create WsClient instance on page load
- Call `wsClient.login()` instead of `login()` (HTTP fetch)
- Handle connection errors
- On success: save token, redirect to `/`

### 3. `web/register.html` (Modify)

**Changes:**
- Create WsClient instance on page load
- Call `wsClient.register()` instead of `register()` (HTTP fetch)
- Handle connection errors
- On success: redirect to login page

### 4. `web/checkin.html` (Modify)

**Changes:**
- Create WsClient instance on page load
- Call `wsClient.getCheckinStatus()` instead of `getCheckinStatus()`
- Call `wsClient.checkin()` instead of `checkin()`
- Handle connection errors and auth errors

### 5. `web/items.html` (Modify)

**Changes:**
- Create WsClient instance on page load
- Call `wsClient.getMyItems()` instead of `getMyItems()`
- Handle connection errors and auth errors

### 6. `web/index.html` (Modify)

**Changes:**
- Create WsClient instance on page load
- Update auth check to use new token management

---

## Testing Strategy

### Manual Testing

1. **Registration Flow:**
   - Open `/register.html`
   - Enter username and password
   - Submit form
   - Verify registration succeeds
   - Verify redirect to login page

2. **Login Flow:**
   - Open `/login.html`
   - Enter registered credentials
   - Submit form
   - Verify login succeeds
   - Verify redirect to index page
   - Verify token stored in localStorage

3. **Checkin Flow:**
   - Navigate to `/checkin.html`
   - Verify status loads correctly
   - Click checkin button
   - Verify rewards displayed

4. **Items Flow:**
   - Navigate to `/items.html`
   - Verify items load correctly

5. **Error Cases:**
   - Try registering with existing username
   - Try logging in with wrong password
   - Disconnect network, verify error handling

---

## Constraints

- Single WebSocket connection per page
- No message ID correlation (use method field for matching)
- Token stored in localStorage
- Connection timeout: 5 seconds
- Request timeout: 5 seconds

---

## Out of Scope

- WebSocket reconnection (future enhancement)
- Heartbeat/ping-pong from client (server handles this)
- Multiple concurrent requests correlation (current protocol uses method matching)
