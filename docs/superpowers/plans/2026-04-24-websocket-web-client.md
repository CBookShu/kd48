# WebSocket Web 客户端实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将 Web 客户端从 HTTP REST API 改为使用 WebSocket 进行所有后端通信。

**架构：** 单一 WebSocket 连接配合 Promise 风格的 API 封装。Web 客户端建立一个 WebSocket 连接到网关，用于所有 API 调用（登录、注册、签到、物品）。通过匹配请求和响应的 `method` 字段进行消息关联。

**技术栈：** JavaScript (ES6+)、WebSocket API、Promise/async-await、Go (后端修改)

---

## 文件结构

**后端修改：**
- `gateway/internal/ws/wrapper.go` - 修改 `WrapIngress` 函数，注入 `user_id` 到 context

**前端修改：**
- `web/js/api.js` - 完全重写为 WsClient 类
- `web/login.html` - 更新为使用 WsClient
- `web/register.html` - 更新为使用 WsClient
- `web/checkin.html` - 更新为使用 WsClient
- `web/js/checkin.js` - 更新为使用 WsClient
- `web/items.html` - 更新为使用 WsClient
- `web/index.html` - 更新认证检查逻辑

---

## Task 1: 修复 WrapIngress 注入 user_id

**Files:**
- Modify: `gateway/internal/ws/wrapper.go:35-45`
- Test: `gateway/internal/ws/wrapper_test.go`

- [ ] **Step 1: 写一个失败的测试**

在 `gateway/internal/ws/wrapper_test.go` 末尾添加测试：

```go
func TestWrapIngress_InjectsUserIDIntoContext(t *testing.T) {
	stub := &stubIngressClient{
		reply: &gatewayv1.IngressReply{JsonPayload: []byte(`{}`)},
	}
	h := WrapIngress(stub, "/test")

	// 模拟已认证用户
	meta := &clientMeta{
		userID:          12345,
		isAuthenticated: true,
	}

	var gotUserID int64
	// 使用自定义 context 检查器
	ctxChecker := contextChecker(func(ctx context.Context) {
		if v := ctx.Value("user_id"); v != nil {
			gotUserID = v.(int64)
		}
	})

	// 修改 stub 以捕获 context
	stubWithChecker := &contextCheckerIngressClient{
		IngressClient: stub,
		checkCtx:      ctxChecker,
	}
	h = WrapIngress(stubWithChecker, "/test")

	_, err := h(context.Background(), []byte(`{}`), meta)
	if err != nil {
		t.Fatal(err)
	}

	if gotUserID != 12345 {
		t.Fatalf("want user_id=12345 in context, got %d", gotUserID)
	}
}

type contextChecker func(ctx context.Context)

type contextCheckerIngressClient struct {
	*stubIngressClient
	checkCtx contextChecker
}

func (c *contextCheckerIngressClient) Call(ctx context.Context, in *gatewayv1.IngressRequest, opts ...grpc.CallOption) (*gatewayv1.IngressReply, error) {
	if c.checkCtx != nil {
		c.checkCtx(ctx)
	}
	return c.stubIngressClient.Call(ctx, in, opts...)
}
```

- [ ] **Step 2: 运行测试验证失败**

Run: `cd gateway && go test ./internal/ws/... -run TestWrapIngress_InjectsUserIDIntoContext -v`
Expected: FAIL - user_id 未注入到 context

- [ ] **Step 3: 实现最小代码**

修改 `gateway/internal/ws/wrapper.go` 中的 `WrapIngress` 函数：

```go
// WrapIngress 将 WS payload（UTF-8 JSON 字节）经 GatewayIngress/Call 转发至后端。
func WrapIngress(cli gatewayv1.GatewayIngressClient, route string) WsHandlerFunc {
	return func(ctx context.Context, payload []byte, meta *clientMeta) (*WsHandlerResult, error) {
		// 如果已认证，将 user_id 注入到 context
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

- [ ] **Step 4: 运行测试验证通过**

Run: `cd gateway && go test ./internal/ws/... -run TestWrapIngress -v`
Expected: PASS - 所有 WrapIngress 测试通过

- [ ] **Step 5: 提交**

```bash
git add gateway/internal/ws/wrapper.go gateway/internal/ws/wrapper_test.go
git commit -m "fix(gateway): inject user_id into context in WrapIngress

Backend services expect ctx.Value(\"user_id\") for authenticated requests.
This fix ensures the user_id from clientMeta is propagated to the context.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 2: 重写 api.js 为 WsClient 类

**Files:**
- Create: `web/js/api.js` (完全重写)

- [ ] **Step 1: 实现 WsClient 类**

完全替换 `web/js/api.js` 的内容：

```javascript
// web/js/api.js - WebSocket API 客户端

class WsClient {
  constructor(url = 'ws://localhost:8080/ws') {
    this.url = url;
    this.ws = null;
    this.pendingCalls = new Map(); // method -> {resolve, reject, timeout}
    this.connectPromise = null;
  }

  // 建立 WebSocket 连接
  connect() {
    if (this.connectPromise) {
      return this.connectPromise;
    }

    this.connectPromise = new Promise((resolve, reject) => {
      this.ws = new WebSocket(this.url);

      const timeout = setTimeout(() => {
        reject(new Error('连接超时'));
        this.ws.close();
      }, 5000);

      this.ws.onopen = () => {
        clearTimeout(timeout);
        resolve();
      };

      this.ws.onerror = (err) => {
        clearTimeout(timeout);
        reject(new Error('连接失败'));
      };

      this.ws.onmessage = (event) => {
        this.handleMessage(event.data);
      };

      this.ws.onclose = () => {
        this.handleClose();
      };
    });

    return this.connectPromise;
  }

  // 处理接收到的消息
  handleMessage(data) {
    try {
      const msg = JSON.parse(data);
      const pending = this.pendingCalls.get(msg.method);
      if (pending) {
        clearTimeout(pending.timeout);
        this.pendingCalls.delete(msg.method);

        if (msg.code === 0) {
          pending.resolve(msg);
        } else {
          pending.reject(new Error(msg.msg || '请求失败'));
        }
      }
    } catch (e) {
      console.error('解析消息失败:', e);
    }
  }

  // 处理连接关闭
  handleClose() {
    // 拒绝所有待处理的请求
    for (const [method, pending] of this.pendingCalls) {
      clearTimeout(pending.timeout);
      pending.reject(new Error('连接已断开'));
    }
    this.pendingCalls.clear();
    this.connectPromise = null;
  }

  // 检查连接状态
  isConnected() {
    return this.ws && this.ws.readyState === WebSocket.OPEN;
  }

  // 关闭连接
  disconnect() {
    if (this.ws) {
      this.ws.close();
      this.ws = null;
    }
    this.connectPromise = null;
  }

  // 通用 API 调用
  call(method, payload = {}) {
    return new Promise((resolve, reject) => {
      if (!this.isConnected()) {
        reject(new Error('未连接'));
        return;
      }

      // 设置超时
      const timeout = setTimeout(() => {
        this.pendingCalls.delete(method);
        reject(new Error('请求超时'));
      }, 5000);

      // 保存待处理的调用
      this.pendingCalls.set(method, { resolve, reject, timeout });

      // 发送请求
      const msg = {
        method: method,
        payload: JSON.stringify(payload)
      };
      this.ws.send(JSON.stringify(msg));
    });
  }

  // 设置请求头（包含 token）
  callWithToken(method, payload = {}) {
    const token = this.getToken();
    if (token) {
      payload._token = token;
    }
    return this.call(method, payload);
  }

  // === 便捷方法 ===

  // 用户登录
  login(username, password) {
    return this.call('/user.v1.UserService/Login', {
      username: username,
      password: password
    });
  }

  // 用户注册
  register(username, password) {
    return this.call('/user.v1.UserService/Register', {
      username: username,
      password: password
    });
  }

  // 获取签到状态
  getCheckinStatus() {
    return this.callWithToken('/lobby.v1.CheckinService/GetStatus');
  }

  // 签到
  checkin() {
    return this.callWithToken('/lobby.v1.CheckinService/Checkin');
  }

  // 获取我的物品
  getMyItems() {
    return this.callWithToken('/lobby.v1.ItemService/GetMyItems');
  }

  // === Token 管理 ===

  saveToken(token) {
    localStorage.setItem('token', token);
  }

  getToken() {
    return localStorage.getItem('token');
  }

  clearToken() {
    localStorage.removeItem('token');
  }
}

// 全局单例实例
let wsClient = null;

// 获取或创建 WsClient 实例
function getWsClient() {
  if (!wsClient) {
    wsClient = new WsClient();
  }
  return wsClient;
}

// 检查登录状态
function checkAuth() {
  const client = getWsClient();
  const token = client.getToken();
  if (!token) {
    window.location.href = '/login.html';
    return false;
  }
  return true;
}

// 登出
function logout() {
  const client = getWsClient();
  client.clearToken();
  client.disconnect();
  window.location.href = '/login.html';
}

// 兼容旧 API 的便捷方法
function saveToken(token) {
  getWsClient().saveToken(token);
}

function getToken() {
  return getWsClient().getToken();
}

function clearToken() {
  getWsClient().clearToken();
}
```

- [ ] **Step 2: 提交**

```bash
git add web/js/api.js
git commit -m "feat(web): rewrite api.js with WsClient class

Replace HTTP REST API with WebSocket-based communication:
- WsClient class with Promise-based call() method
- Message correlation via method field matching
- Token management in localStorage
- Connection state management

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 3: 更新 login.html 使用 WsClient

**Files:**
- Modify: `web/login.html`

- [ ] **Step 1: 更新登录页面脚本**

修改 `web/login.html`，替换 `<script>` 部分：

```html
  <script src="/js/api.js"></script>
  <script>
    const client = getWsClient();

    document.getElementById('loginForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      const errorEl = document.getElementById('error');

      try {
        // 建立连接
        await client.connect();
      } catch (err) {
        errorEl.textContent = '连接失败: ' + err.message;
        return;
      }

      try {
        const resp = await client.login(username, password);
        if (resp.code === 0) {
          client.saveToken(resp.data.token);
          window.location.href = '/';
        } else {
          errorEl.textContent = resp.msg || '登录失败';
        }
      } catch (err) {
        errorEl.textContent = err.message;
      }
    });

    // 已登录则跳转首页
    if (client.getToken()) {
      window.location.href = '/';
    }
  </script>
```

- [ ] **Step 2: 提交**

```bash
git add web/login.html
git commit -m "feat(web): update login.html to use WsClient

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 4: 更新 register.html 使用 WsClient

**Files:**
- Modify: `web/register.html`

- [ ] **Step 1: 更新注册页面脚本**

修改 `web/register.html`，替换 `<script>` 部分：

```html
  <script src="/js/api.js"></script>
  <script>
    const client = getWsClient();

    document.getElementById('registerForm').addEventListener('submit', async (e) => {
      e.preventDefault();
      const username = document.getElementById('username').value;
      const password = document.getElementById('password').value;
      const password2 = document.getElementById('password2').value;
      const errorEl = document.getElementById('error');

      if (password !== password2) {
        errorEl.textContent = '密码不一致';
        return;
      }

      try {
        // 建立连接
        await client.connect();
      } catch (err) {
        errorEl.textContent = '连接失败: ' + err.message;
        return;
      }

      try {
        const resp = await client.register(username, password);
        if (resp.code === 0) {
          alert('注册成功，请登录');
          window.location.href = '/login.html';
        } else {
          errorEl.textContent = resp.msg || '注册失败';
        }
      } catch (err) {
        errorEl.textContent = err.message;
      }
    });
  </script>
```

- [ ] **Step 2: 提交**

```bash
git add web/register.html
git commit -m "feat(web): update register.html to use WsClient

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 5: 更新 checkin.html 和 checkin.js 使用 WsClient

**Files:**
- Modify: `web/checkin.html`
- Modify: `web/js/checkin.js`

- [ ] **Step 1: 更新 checkin.js**

修改 `web/js/checkin.js`，更新 API 调用：

```javascript
// web/js/checkin.js

let client = null;

// 初始化 WebSocket 客户端
async function initClient() {
  client = getWsClient();
  await client.connect();
}

// 渲染签到状态
function renderStatus(data) {
  const statusDiv = document.getElementById('status');

  statusDiv.innerHTML = `
    <p>连续签到: <strong>${data.continuous_days}</strong> 天</p>
    <p>累计签到: <strong>${data.total_days}</strong> 天</p>
    <p>本期: ${data.period_name}</p>
  `;

  // 渲染日期格子
  const daysDiv = document.getElementById('days');
  daysDiv.innerHTML = '';

  for (let i = 1; i <= 7; i++) {
    const dayBox = document.createElement('div');
    dayBox.className = 'day-box';
    dayBox.textContent = `第${i}天`;

    if (i <= data.total_days) {
      dayBox.classList.add('checked');
    }
    if (i === data.total_days + 1 && !data.today_checked) {
      dayBox.classList.add('current');
    }

    daysDiv.appendChild(dayBox);
  }

  // 更新按钮状态
  const btn = document.getElementById('checkinBtn');
  if (data.today_checked) {
    btn.disabled = true;
    btn.textContent = '今日已签到';
  } else {
    btn.disabled = false;
    btn.textContent = '立即签到';
  }
}

// 渲染奖励结果
function renderReward(data) {
  const resultDiv = document.getElementById('result');
  resultDiv.innerHTML = '<h3>签到成功！</h3>';

  const rewardsDiv = document.createElement('div');
  for (const [itemId, count] of Object.entries(data.rewards)) {
    const span = document.createElement('span');
    span.className = 'reward-item';
    span.textContent = `物品${itemId} x ${count}`;
    rewardsDiv.appendChild(span);
  }
  resultDiv.appendChild(rewardsDiv);
}

// 加载签到状态
async function loadStatus() {
  if (!client) {
    await initClient();
  }

  try {
    const resp = await client.getCheckinStatus();
    if (resp.code === 0) {
      renderStatus(resp.data);
    } else if (resp.code === 16) {
      // 未认证，跳转登录
      alert('请先登录');
      window.location.href = '/login.html';
    } else {
      alert(resp.msg || '加载状态失败');
    }
  } catch (err) {
    console.error('加载状态失败:', err);
    alert('网络错误: ' + err.message);
  }
}

// 执行签到
async function doCheckin() {
  const btn = document.getElementById('checkinBtn');
  btn.disabled = true;

  try {
    const resp = await client.checkin();
    if (resp.code === 0) {
      renderReward(resp.data);
      await loadStatus();
    } else if (resp.code === 200) {
      alert('今日已签到');
    } else if (resp.code === 16) {
      alert('请先登录');
      window.location.href = '/login.html';
    } else {
      alert(resp.msg || '签到失败');
    }
  } catch (err) {
    console.error('签到失败:', err);
    alert('网络错误: ' + err.message);
  } finally {
    btn.disabled = false;
  }
}
```

- [ ] **Step 2: 更新 checkin.html**

修改 `web/checkin.html`，更新脚本部分：

```html
  <script src="/js/api.js"></script>
  <script src="/js/checkin.js"></script>
  <script>
    async function init() {
      if (!checkAuth()) {
        return;
      }

      try {
        await initClient();
        await loadStatus();
      } catch (err) {
        console.error('初始化失败:', err);
        document.getElementById('status').textContent = '连接失败: ' + err.message;
      }
    }

    init();
  </script>
```

- [ ] **Step 3: 提交**

```bash
git add web/checkin.html web/js/checkin.js
git commit -m "feat(web): update checkin pages to use WsClient

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 6: 更新 items.html 使用 WsClient

**Files:**
- Modify: `web/items.html`

- [ ] **Step 1: 更新物品页面脚本**

修改 `web/items.html`，替换 `<script>` 部分：

```html
  <script src="/js/api.js"></script>
  <script>
    const client = getWsClient();

    async function loadItems() {
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
          // 未认证，跳转登录
          alert('请先登录');
          window.location.href = '/login.html';
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

    if (checkAuth()) {
      loadItems();
    }
  </script>
```

- [ ] **Step 2: 提交**

```bash
git add web/items.html
git commit -m "feat(web): update items.html to use WsClient

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 7: 更新 index.html 认证检查

**Files:**
- Modify: `web/index.html`

- [ ] **Step 1: 更新首页脚本**

修改 `web/index.html`，替换 `<script>` 部分：

```html
  <script src="/js/api.js"></script>
  <script>
    // 检查登录状态，未登录则跳转到登录页
    if (!checkAuth()) {
      // checkAuth 会自动跳转到登录页
    }
  </script>
```

- [ ] **Step 2: 提交**

```bash
git add web/index.html
git commit -m "feat(web): update index.html auth check

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>"
```

---

## Task 8: 验证整体功能

**Files:**
- 无文件修改，仅验证

- [ ] **Step 1: 启动服务**

Run: `cd /Users/cbookshu/dev/temp/kd48 && make run`
Expected: 所有服务启动成功

- [ ] **Step 2: 手动测试注册流程**

1. 打开浏览器访问 `http://localhost:8080/register.html`
2. 输入用户名和密码
3. 提交表单
4. 验证注册成功并跳转到登录页

- [ ] **Step 3: 手动测试登录流程**

1. 打开浏览器访问 `http://localhost:8080/login.html`
2. 输入已注册的凭据
3. 提交表单
4. 验证登录成功并跳转到首页
5. 验证 token 存储在 localStorage

- [ ] **Step 4: 手动测试签到流程**

1. 导航到 `http://localhost:8080/checkin.html`
2. 验证状态正确加载
3. 点击签到按钮
4. 验证奖励显示正确

- [ ] **Step 5: 手动测试物品流程**

1. 导航到 `http://localhost:8080/items.html`
2. 验证物品正确加载（应显示签到获得的物品）

- [ ] **Step 6: 提交最终状态**

```bash
git status
```

Expected: 工作区干净，所有更改已提交
