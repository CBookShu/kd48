# Web 单页面应用实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将多个独立页面整合为单页面应用（SPA），在 index.html 中管理登录/注册/已登录状态。

**架构：** 单页面应用，通过 JavaScript 切换视图，不进行页面跳转。Token 存储在 localStorage，页面加载时自动验证 token。

**技术栈：** JavaScript (ES6+)、WebSocket API、Go (后端修改)

---

## 文件结构

**后端修改：**
- `api/proto/user/v1/user.proto` - 添加 VerifyToken 消息和 rpc
- `services/user/cmd/user/server.go` - 实现 VerifyToken 方法
- `services/user/cmd/user/ingress.go` - 注册路由

**前端修改：**
- `web/js/api.js` - 添加 verifyToken 方法
- `web/index.html` - 重写为单页面应用
- `web/checkin.html` - 添加登录检查
- `web/items.html` - 添加登录检查

---

## Task 1: 后端添加 VerifyToken API

**Files:**
- Modify: `api/proto/user/v1/user.proto` - 添加 VerifyToken 消息和 rpc
- Modify: `services/user/cmd/user/server.go` - 实现 VerifyToken 方法
- Modify: `services/user/cmd/user/ingress.go` - 注册路由

- [ ] **Step 1: 添加 Proto 定义**

修改 `api/proto/user/v1/user.proto`，在 service UserService 中添加：

```protobuf
rpc VerifyToken (VerifyTokenRequest) returns (VerifyTokenReply) {}

message VerifyTokenRequest {}

message VerifyTokenReply {
  int32 code = 1;
  string msg = 2;
  VerifyTokenData data = 3;
}

message VerifyTokenData {
  int64 user_id = 1;
  string username = 2;
}
```

- [ ] **Step 2: 重新生成 Proto 代码**

运行: `cd /Users/cbookshu/dev/temp/kd48 && ./gen_proto.sh`

- [ ] **Step 3: 实现 VerifyToken 方法**

在 `services/user/cmd/user/server.go` 文件末尾添加：

```go
func (s *userService) VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenReply, error) {
	slog.InfoContext(ctx, "Received VerifyToken request")

	// 从 context 获取 user_id（网关已注入）
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, status.Error(codes.Unauthenticated, "未认证")
	}

	uid, ok := userID.(int64)
	if !ok {
		return nil, status.Error(codes.Internal, "internal server error")
	}

	// 查询用户信息
	routingKey := fmt.Sprintf("sys:user:id:%d", uid)
	queries, err := s.getQueries(ctx, routingKey)
	if err != nil {
		return nil, err
	}

	user, err := queries.GetUserByID(ctx, uint64(uid))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.Unauthenticated, "用户不存在")
		}
		slog.ErrorContext(ctx, "GetUserByID failed", "error", err)
		return nil, status.Error(codes.Internal, "internal server error")
	}

	return &userv1.VerifyTokenReply{
		Code:    0,
		UserId:  user.ID,
		Username: user.Username,
	}, nil
}
```

- [ ] **Step 3: 注册路由**

修改 `services/user/cmd/user/ingress.go`：

1. 在 `userLoginRegister` 接口添加 VerifyToken 方法：
```go
type userLoginRegister interface {
	Login(ctx context.Context, req *userv1.LoginRequest) (*userv1.LoginReply, error)
	Register(ctx context.Context, req *userv1.RegisterRequest) (*userv1.RegisterReply, error)
	VerifyToken(ctx context.Context, req *userv1.VerifyTokenRequest) (*userv1.VerifyTokenReply, error)
}
```

2. 在 Call 方法的 switch 中添加 case（在 Register 之后）：
```go
case "/user.v1.UserService/VerifyToken":
	if s.inner == nil {
		return nil, status.Error(codes.Internal, "ingress inner not configured")
	}
	// VerifyToken 不需要解析 payload，直接调用
	out, err := s.inner.VerifyToken(ctx, &userv1.VerifyTokenRequest{})
	if err != nil {
		return nil, err
	}
	b, err := protojson.Marshal(out)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "marshal reply: %v", err)
	}
	return &gatewayv1.IngressReply{JsonPayload: b}, nil
```

3. 在 main.go 中确保将 userService 传给 ingressServer（检查现有代码是否需要修改）。

- [ ] **Step 4: 提交**

```bash
git add services/user/cmd/user/server.go
git commit -m "feat(user): add VerifyToken API for token validation

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: 前端添加 verifyToken 方法

**Files:**
- Modify: `web/js/api.js`

- [ ] **Step 1: 添加 verifyToken 方法**

在 `web/js/api.js` 的 WsClient 类中添加方法（在 `getMyItems` 方法之后）：

```javascript
// 验证 token 是否有效
verifyToken() {
  return this.callWithToken('/user.v1.UserService/VerifyToken');
}
```

- [ ] **Step 2: 提交**

```bash
git add web/js/api.js
git commit -m "feat(web): add verifyToken method to WsClient

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: 重写 index.html 为单页面应用

**Files:**
- Modify: `web/index.html`

- [ ] **Step 1: 重写 index.html**

完全替换 `web/index.html` 的内容：

```html
<!-- web/index.html -->
<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <title>kd48 - 游戏平台</title>
  <link rel="stylesheet" href="/css/style.css">
  <style>
    .hidden { display: none !important; }
    .view { animation: fadeIn 0.3s ease; }
    @keyframes fadeIn {
      from { opacity: 0; transform: translateY(10px); }
      to { opacity: 1; transform: translateY(0); }
    }
  </style>
</head>
<body>
  <div id="app">
    <!-- Loading 视图 -->
    <div id="view-loading" class="view">
      <div class="container">
        <div class="card">
          <p>加载中...</p>
        </div>
      </div>
    </div>

    <!-- Guest 视图 (未登录) -->
    <div id="view-guest" class="view hidden">
      <div class="container">
        <!-- 登录表单 -->
        <div id="login-panel" class="card">
          <h2>登录</h2>
          <form id="loginForm">
            <input type="text" id="username" placeholder="用户名" required>
            <input type="password" id="password" placeholder="密码" required>
            <button type="submit" class="btn">登录</button>
          </form>
          <p style="margin-top: 10px;">
            没有账号？<a href="#" id="show-register">注册</a>
          </p>
          <p id="login-error" style="color: red;"></p>
        </div>

        <!-- 注册表单 -->
        <div id="register-panel" class="card hidden">
          <h2>注册</h2>
          <form id="registerForm">
            <input type="text" id="reg-username" placeholder="用户名" required>
            <input type="password" id="reg-password" placeholder="密码" required>
            <input type="password" id="reg-password2" placeholder="确认密码" required>
            <button type="submit" class="btn">注册</button>
          </form>
          <p style="margin-top: 10px;">
            已有账号？<a href="#" id="show-login">登录</a>
          </p>
          <p id="register-error" style="color: red;"></p>
        </div>
      </div>
    </div>

    <!-- LoggedIn 视图 (已登录) -->
    <div id="view-loggedin" class="view hidden">
      <nav class="nav">
        <span id="welcome-user"></span>
        <a href="/">首页</a>
        <a href="/checkin.html">签到</a>
        <a href="/items.html">背包</a>
        <a href="#" id="logout-btn">退出</a>
      </nav>

      <div class="container">
        <div class="card">
          <h1>欢迎来到 kd48</h1>
          <p>一个简单的游戏平台演示</p>
          <div style="margin-top: 20px;">
            <a href="/checkin.html" class="btn">签到</a>
            <a href="/items.html" class="btn">背包</a>
          </div>
        </div>
      </div>
    </div>
  </div>

  <script src="/js/api.js"></script>
  <script>
    // 视图常量
    const VIEW = {
      LOADING: 'loading',
      GUEST: 'guest',
      LOGGEDIN: 'loggedin'
    };

    let client = getWsClient();
    let currentUser = null;

    // 切换视图
    function showView(view) {
      document.getElementById('view-loading').classList.add('hidden');
      document.getElementById('view-guest').classList.add('hidden');
      document.getElementById('view-loggedin').classList.add('hidden');
      document.getElementById('view-' + view).classList.remove('hidden');
    }

    // 显示登录表单
    function showLogin() {
      document.getElementById('login-panel').classList.remove('hidden');
      document.getElementById('register-panel').classList.add('hidden');
    }

    // 显示注册表单
    function showRegister() {
      document.getElementById('login-panel').classList.add('hidden');
      document.getElementById('register-panel').classList.remove('hidden');
    }

    // 登录处理
    document.getElementById('loginForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      const errorEl = document.getElementById('login-error');
      errorEl.textContent = '';

      try {
        await client.connect();
      } catch (err) {
        errorEl.textContent = '连接失败: ' + err.message;
        return;
      }

      try {
        const resp = await client.login(username, password);
        if (resp.code === 0 && resp.data?.token) {
          client.saveToken(resp.data.token);
          currentUser = { username: username, userId: resp.data.user_id };
          updateWelcome();
          showView(VIEW.LOGGEDIN);
        } else {
          errorEl.textContent = resp.msg || '登录失败';
        }
      } catch (err) {
        errorEl.textContent = err.message;
      }
    });

    // 注册处理
    document.getElementById('registerForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('reg-username').value;
      const password = document.getElementById('reg-password').value;
      const password2 = document.getElementById('reg-password2').value;
      const errorEl = document.getElementById('register-error');
      errorEl.textContent = '';

      if (password !== password2) {
        errorEl.textContent = '密码不一致';
        return;
      }

      try {
        await client.connect();
      } catch (err) {
        errorEl.textContent = '连接失败: ' + err.message;
        return;
      }

      try {
        const resp = await client.register(username, password);
        if (resp.code === 0) {
          alert('注册成功，请登录');
          showLogin();
          // 自动填充用户名
          document.getElementById('username').value = username;
        } else {
          errorEl.textContent = resp.msg || '注册失败';
        }
      } catch (err) {
        errorEl.textContent = err.message;
      }
    });

    // 切换登录/注册
    document.getElementById('show-register').addEventListener('click', (e) => {
      e.preventDefault();
      showRegister();
    });

    document.getElementById('show-login').addEventListener('click', (e) => {
      e.preventDefault();
      showLogin();
    });

    // 退出登录
    document.getElementById('logout-btn').addEventListener('click', (e) => {
      e.preventDefault();
      client.clearToken();
      client.disconnect();
      currentUser = null;
      showView(VIEW.GUEST);
    });

    // 更新欢迎语
    function updateWelcome() {
      const el = document.getElementById('welcome-user');
      if (currentUser) {
        el.textContent = '欢迎, ' + currentUser.username;
      }
    }

    // 初始化
    async function init() {
      const token = client.getToken();

      if (!token) {
        showView(VIEW.GUEST);
        return;
      }

      try {
        await client.connect();
        const resp = await client.verifyToken();
        if (resp.code === 0 && resp.data) {
          currentUser = { username: resp.data.username, userId: resp.data.user_id };
          updateWelcome();
          showView(VIEW.LOGGEDIN);
        } else {
          // token 无效，清除并显示登录
          client.clearToken();
          showView(VIEW.GUEST);
        }
      } catch (err) {
        console.error('验证 token 失败:', err);
        client.clearToken();
        showView(VIEW.GUEST);
      }
    }

    init();
  </script>
</body>
</html>
```

- [ ] **Step 2: 提交**

```bash
git add web/index.html
git commit -m "feat(web): rewrite index.html as SPA

- Single page with loading/guest/loggedin views
- Auto-login on page load via token verification
- Toggle between login and register forms
- Token stored in localStorage, cleared on logout

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: 更新 checkin.html 和 items.html 登录检查

**Files:**
- Modify: `web/checkin.html`
- Modify: `web/items.html`

- [ ] **Step 1: 更新 checkin.html**

修改 `web/checkin.html` 的脚本部分，更新错误处理：

```html
<script src="/js/api.js"></script>
<script src="/js/checkin.js"></script>
<script>
  async function init() {
    const client = getWsClient();
    const token = client.getToken();

    if (!token) {
      document.getElementById('status').innerHTML = '<p>请先 <a href="/">登录</a></p>';
      return;
    }

    try {
      await initClient();
      await loadStatus();
    } catch (err) {
      console.error('初始化失败:', err);
      if (err.message.includes('未认证') || err.message.includes('token')) {
        client.clearToken();
        document.getElementById('status').innerHTML = '<p>登录已过期，请 <a href="/">重新登录</a></p>';
      } else {
        document.getElementById('status').textContent = '连接失败: ' + err.message;
      }
    }
  }

  init();
</script>
```

- [ ] **Step 2: 更新 items.html**

修改 `web/items.html` 的脚本部分：

```html
<script src="/js/api.js"></script>
<script>
  const client = getWsClient();

  async function loadItems() {
    const token = client.getToken();

    if (!token) {
      document.getElementById('items').innerHTML = '<p>请先 <a href="/">登录</a></p>';
      return;
    }

    try {
      await client.connect();
    } catch (err) {
      document.getElementById('items').textContent = '连接失败: ' + err.message;
      return;
    }

    try {
      const resp = await client.getMyItems();
      if (resp.code === 0) {
        renderItems(resp.data);
      } else if (resp.code === 16) {
        // 未认证
        client.clearToken();
        document.getElementById('items').innerHTML = '<p>登录已过期，请 <a href="/">重新登录</a></p>';
      } else {
        document.getElementById('items').textContent = resp.msg || '加载失败';
      }
    } catch (err) {
      document.getElementById('items').textContent = '网络错误: ' + err.message;
    }
  }

  function renderItems(data) {
    const itemsDiv = document.getElementById('items');

    if (!data.items || Object.keys(data.items).length === 0) {
      itemsDiv.innerHTML = '<p>暂无道具</p>';
      return;
    }

    let html = '<table style="width: 100%; border-collapse: collapse;">';
    html += '<tr><th style="text-align: left; padding: 10px; border-bottom: 1px solid #ddd;">物品ID</th><th style="text-align: right; padding: 10px; border-bottom: 1px solid #ddd;">数量</th></tr>';

    for (const [itemId, count] of Object.entries(data.items)) {
      html += `<tr><td style="padding: 10px; border-bottom: 1px solid #eee;">物品 ${itemId}</td><td style="text-align: right; padding: 10px; border-bottom: 1px solid #eee;">${count}</td></tr>`;
    }

    html += '</table>';
    itemsDiv.innerHTML = html;
  }

  loadItems();
</script>
```

- [ ] **Step 3: 提交**

```bash
git add web/checkin.html web/items.html
git commit -m "feat(web): add login check to checkin and items pages

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: 验证功能

**Files:**
- 无文件修改，仅验证

- [ ] **Step 1: 启动服务**

Run: `cd /Users/cbookshu/dev/temp/kd48 && ./run.sh start`
Expected: 所有服务启动成功

- [ ] **Step 2: 测试 SPA 首页**

1. 打开浏览器访问 `http://localhost:8080/`
2. 验证显示登录表单（无 token）
3. 验证可以切换到注册表单

- [ ] **Step 3: 测试注册登录**

1. 点击注册，输入用户名密码
2. 提交注册
3. 验证注册成功提示
4. 登录
5. 验证显示功能菜单（签到、背包、退出）

- [ ] **Step 4: 测试自动登录**

1. 刷新页面
2. 验证自动登录，保持在已登录状态

- [ ] **Step 5: 测试退出**

1. 点击退出
2. 验证返回登录表单
3. 刷新页面，验证仍显示登录表单

- [ ] **Step 6: 测试 checkin 和 items**

1. 访问 checkin.html，验证正常显示
2. 访问 items.html，验证正常显示

- [ ] **Step 7: 提交状态**

```bash
git status
```
