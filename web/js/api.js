// web/js/api.js
const API_BASE = 'http://localhost:8080';

// 调用 gRPC 服务（通过 Gateway Ingress）
async function callApi(service, method, data, token) {
  const response = await fetch(`${API_BASE}/api/${service}/${method}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { 'Authorization': `Bearer ${token}` } : {})
    },
    body: JSON.stringify(data)
  });

  return response.json();
}

// 用户注册
async function register(username, password) {
  return callApi('user.v1.UserService', 'Register', {
    username: username,
    password: password
  });
}

// 用户登录
async function login(username, password) {
  return callApi('user.v1.UserService', 'Login', {
    username: username,
    password: password
  });
}

// 获取签到状态
async function getCheckinStatus(token) {
  return callApi('lobby.v1.CheckinService', 'GetStatus', {}, token);
}

// 签到
async function checkin(token) {
  return callApi('lobby.v1.CheckinService', 'Checkin', {}, token);
}

// 获取我的物品
async function getMyItems(token) {
  return callApi('lobby.v1.ItemService', 'GetMyItems', {}, token);
}

// 保存 token
function saveToken(token) {
  localStorage.setItem('token', token);
}

// 获取 token
function getToken() {
  return localStorage.getItem('token');
}

// 清除 token
function clearToken() {
  localStorage.removeItem('token');
}

// 检查登录状态
function checkAuth() {
  const token = getToken();
  if (!token) {
    window.location.href = '/login.html';
    return false;
  }
  return true;
}

// 登出
function logout() {
  clearToken();
  window.location.href = '/login.html';
}
