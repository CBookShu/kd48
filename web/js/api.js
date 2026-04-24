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
