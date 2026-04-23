-- 签到期配置表
CREATE TABLE IF NOT EXISTS checkin_period (
    period_id BIGINT PRIMARY KEY,
    period_name VARCHAR(255) NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    INDEX idx_status_time (status, start_time, end_time)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 每日签到奖励表
CREATE TABLE IF NOT EXISTS checkin_daily_reward (
    day INT PRIMARY KEY,
    rewards JSON NOT NULL,  -- {"1001": 100, "1002": 10}
    INDEX idx_day (day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 连续签到奖励表
CREATE TABLE IF NOT EXISTS checkin_continuous_reward (
    continuous_days INT PRIMARY KEY,
    rewards JSON NOT NULL,  -- {"2001": 1}
    INDEX idx_days (continuous_days)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 物品配置表
CREATE TABLE IF NOT EXISTS item_config (
    item_id INT PRIMARY KEY,
    item_name VARCHAR(255) NOT NULL,
    item_type VARCHAR(64) NOT NULL,
    description VARCHAR(512),
    icon VARCHAR(255),
    INDEX idx_type (item_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 玩家物品表
CREATE TABLE IF NOT EXISTS user_items (
    user_id BIGINT NOT NULL,
    item_id INT NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (user_id, item_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 玩家签到数据表
CREATE TABLE IF NOT EXISTS user_checkin (
    id BIGINT AUTO_INCREMENT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    period_id BIGINT NOT NULL,
    last_checkin_date DATE NOT NULL,
    continuous_days INT NOT NULL DEFAULT 0,
    claimed_days JSON,  -- [1,2,3,5,7]
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uk_user (user_id),
    INDEX idx_period (period_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
