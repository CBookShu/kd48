# Web 单页面应用设计文档

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 将多个独立页面整合为单页面应用（SPA），统一在 index.html 中管理登录/注册/已登录状态。

**架构：** 单页面应用，通过 JavaScript 切换视图，不进行页面跳转。

---

## 问题描述

当前 Web 客户端有多个独立页面：
- `index.html` - 首页，未登录自动跳转 login.html
- `login.html` - 登录页
- `register.html` - 注册页
- `checkin.html` - 签到页
- `items.html` - 物品页

缺点：
1. 页面跳转体验不好
2. 未登录时自动跳转，用户体验生硬
3. 每个页面都要检查登录状态，代码重复

---

## 解决方案：单页面应用

### 视图结构

```
index.html (单一入口)
├── 视图 1: loading → 显示加载状态
│
├── 视图 2: guest (未登录) → 显示登录/注册表单
│   ├── 登录表单 (用户名 + 密码 + 登录按钮)
│   ├── 注册表单 (用户名 + 密码 + 确认密码 + 注册按钮)
│   └── 切换链接 ("没有账号？注册" / "已有账号？登录")
│
└── 视图 3: loggedin (已登录) → 显示功能菜单
    ├── 用户信息 (欢迎语)
    ├── 签到入口 (链接到 checkin.html)
    ├── 背包入口 (链接到 items.html)
    └── 退出按钮
```

### 状态管理

```javascript
// 视图状态
const VIEW = {
  LOADING: 'loading',     // 页面加载中
  GUEST: 'guest',         // 未登录
  LOGGEDIN: 'loggedin'    // 已登录
};

let currentView = VIEW.LOADING;
```

### 数据流

1. **页面加载** → 显示 loading 视图
2. **检查 localStorage token**
   - 有 token → 调用 verifyToken 验证
     - 成功 → 切换到 loggedin 视图
     - 失败 → 清除 token，切换到 guest 视图
   - 无 token → 切换到 guest 视图

### 用户操作

| 操作 | 处理 |
|------|------|
| 登录提交 | 调用 login API，成功保存 token，切换到 loggedin |
| 注册提交 | 调用 register API，成功提示登录，切换到登录表单 |
| 点击退出 | 清除 token，断开连接，切换到 guest |
| 切换登录/注册 | 切换表单显示 |

---

## 后端改动

### verifyToken API

需要新增验证 token 的接口：

**请求：**
```json
{
  "method": "/user.v1.UserService/VerifyToken",
  "payload": "{}"
}
```

**响应（成功）：**
```json
{
  "method": "/user.v1.UserService/VerifyToken",
  "code": 0,
  "data": {
    "user_id": 123,
    "username": "alice"
  }
}
```

**响应（失败/过期）：**
```json
{
  "method": "/user.v1.UserService/VerifyToken",
  "code": 16,
  "msg": "token 无效或已过期"
}
```

实现方式：复用现有的 token 验证逻辑（user service 已有验证能力）。

---

## 前端改动

### 1. api.js (修改)

新增方法：

```javascript
// 验证 token 是否有效
verifyToken() {
  return this.callWithToken('/user.v1.UserService/VerifyToken');
}
```

### 2. index.html (重写)

**结构：**
```html
<div id="app">
  <!-- loading 视图 -->
  <div id="view-loading" class="view">加载中...</div>

  <!-- guest 视图 -->
  <div id="view-guest" class="view hidden">
    <!-- 登录表单 -->
    <div id="login-form">...</div>
    <!-- 注册表单 -->
    <div id="register-form" class="hidden">...</div>
  </div>

  <!-- loggedin 视图 -->
  <div id="view-loggedin" class="view hidden">
    <nav>...</nav>
    <div class="container">...</div>
  </div>
</div>
```

**视图切换：**
```javascript
function showView(view) {
  // 隐藏所有视图
  document.querySelectorAll('.view').forEach(el => el.classList.add('hidden'));
  // 显示目标视图
  document.getElementById(`view-${view}`).classList.remove('hidden');
}
```

### 3. 页面路由

保留 checkin.html 和 items.html（方便书签），但增加登录检查：
- 有 token → 正常显示
- 无 token → 提示并提供"去登录"按钮

---

## 不在范围内

- 复杂的状态管理（Redux 等）
- 路由库（手动切换视图）
- 多账号缓存

---

## 验收标准

1. 打开 index.html，无 token 时显示登录/注册表单
2. 登录成功后显示功能菜单，不跳转
3. 刷新页面保持登录状态（自动验证 token）
4. 点击退出返回登录界面
5. 注册成功后提示登录，并显示登录表单
