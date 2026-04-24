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
