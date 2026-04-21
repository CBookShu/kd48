# Config-Loader

CSV 配置打表工具，用于 Lobby 服务配置管理。

## 功能

- CSV 解析（三行头格式）
- 数据校验
- JSON payload 生成
- MySQL 写入
- Redis 通知
- Go struct 代码生成

## 使用

```bash
# 校验 + 输出
config-loader -input ./example.csv -output ./out.json

# 写库 + 通知
config-loader -input ./example.csv \
  -mysql-dsn "root:root@tcp(localhost:3306)/kd48?parseTime=true" \
  -redis-addr "localhost:6379" \
  -scope checkin

# Go 代码生成
config-loader -input ./example.csv -gen-go -go-out ./generated/example.go
```

## CSV 格式

三行表头：
- 第1行：中文说明
- 第2行：变量名（snake_case）
- 第3行：类型

支持类型：int32, int64, string, time, int32[], int64[], string[], map

## 参数

| 参数 | 必填 | 说明 |
|------|------|------|
| `-input` | ✅ | CSV 文件路径 |
| `-output` | ❌ | JSON 输出路径（默认 stdout） |
| `-mysql-dsn` | ❌ | MySQL 连接串 |
| `-redis-addr` | ❌ | Redis 地址 |
| `-scope` | ❌ | 业务域 |
| `-title` | ❌ | 配置标题 |
| `-revision` | ❌ | 版本号（默认 unix_millis） |
| `-gen-go` | ❌ | 启用 Go 代码生成 |
| `-go-out` | ❌ | Go 文件输出路径 |
| `-go-package` | ❌ | Go 包名（默认 lobbyconfig） |
| `-dry-run` | ❌ | 仅校验不写入 |
| `-verbose` | ❌ | 详细日志 |
