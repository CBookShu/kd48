# Config-Loader 打表工具设计

> **状态**：待批准
> **日期**：2026-04-21
> **关联**：[Lobby 策划 CSV 与打表工具规格](./2026-04-16-lobby-config-csv-and-tooling-spec.md)（单一信源）

本文档为打表工具的**实现设计**，聚焦于目录结构、数据结构、管道接口、CLI、Go 代码生成等实现细节。CSV 格式、`json_payload` 形状、处理流水线等规格以关联文档为准。

---

## 1. 概述

### 1.1 目标

实现完整的打表工具，支持：

1. CSV 解析（三行头格式）
2. 数据校验
3. `json_payload` 生成
4. MySQL 写入（`lobby_config_revision` 表）
5. Redis 通知发布
6. Go struct 代码生成

### 1.2 决策记录

| 决策项 | 选择 |
|--------|------|
| 功能范围 | 全部功能 |
| Redis 失败处理 | 不阻塞，记录日志，退出码 0 |
| 文件位置 | `tools/config-loader/` |
| Go 生成位置 | `tools/config-loader/generated/` |
| CLI 框架 | `flag`（与 seed-gateway-meta 一致） |
| 架构模式 | 管道式架构 |

### 1.3 后续规划

- **Web UI**：文件列表、版本管理、生效切换、ACL 权限控制（独立迭代）

---

## 2. 目录结构

```
tools/config-loader/
├── cmd/
│   └── config-loader/
│       └── main.go              # CLI 入口，组装管道
├── internal/
│   ├── csvparser/
│   │   ├── parser.go            # CSV 解析（三行头 → Sheet）
│   │   ├── parser_test.go
│   │   ├── types.go             # 内置类型定义与解析
│   │   └── types_test.go
│   ├── validator/
│   │   ├── validator.go         # 数据校验（空值、格式、约束）
│   │   └── validator_test.go
│   ├── jsongen/
│   │   ├── generator.go         # Sheet → json_payload
│   │   └── generator_test.go
│   ├── mysqlwriter/
│   │   ├── writer.go            # 事务写入 lobby_config_revision
│   │   └── writer_test.go
│   ├── redisnotify/
│   │   ├── publisher.go         # PUBLISH 到 Redis
│   │   └── publisher_test.go
│   ├── gogen/
│   │   ├── generator.go         # Sheet → Go struct
│   │   └── generator_test.go
│   └── pipeline/
│       ├── pipeline.go          # 管道执行器
│       └── pipeline_test.go
├── generated/                   # 生成的 Go 文件输出目录
│   └── .gitkeep
├── go.mod
└── README.md
```

### 2.1 go.work 集成

```go
// go.work 添加
use ./tools/config-loader
```

### 2.2 go.mod

```go
module github.com/CBookShu/kd48/tools/config-loader

go 1.26.1

require (
    github.com/go-sql-driver/mysql v1.9.2
    github.com/redis/go-redis/v9 v9.18.0
)
```

---

## 3. 数据流

```
CSV 文件
    ↓
csvparser.Parse() → Sheet
    ↓
validator.Validate(Sheet) → error
    ↓
jsongen.Generate(Sheet, opts) → Payload
    ↓
[可选] mysqlwriter.Write(Payload) → MySQL
    ↓
[可选] redisnotify.Publish(config_name, revision) → Redis
    ↓
[可选] gogen.Generate(Sheet, opts) → Go struct file
```

---

## 4. 核心数据结构

### 4.1 Sheet（解析后的 CSV）

```go
// internal/csvparser/sheet.go

// Sheet 表示解析后的 CSV 表
type Sheet struct {
    Headers    []ColumnHeader // 三行头
    Rows       []Row          // 数据行
    ConfigName string         // 从文件名推导
    SourceFile string         // 原始文件路径
}

// ColumnHeader 三行头信息
type ColumnHeader struct {
    Description string // 第1行：中文说明
    Name        string // 第2行：变量名
    Type        string // 第3行：类型
}

// Row 单行数据
type Row struct {
    Values []Value // 每列的值
}

// Value 类型化的单元格值
type Value struct {
    Raw     string      // 原始字符串
    Parsed  interface{} // 解析后的值
    Type    string      // 类型
    IsEmpty bool        // 是否空单元格
}
```

### 4.2 Payload（json_payload）

```go
// internal/jsongen/payload.go

// Payload 表示 json_payload 根对象（规格 §3）
type Payload struct {
    ConfigName string           `json:"config_name"`
    Revision   int64            `json:"revision"`
    Data       []map[string]any `json:"data"`
}
```

---

## 5. 管道阶段接口

### 5.1 接口定义

```go
// internal/pipeline/pipeline.go

// Stage 管道阶段接口
type Stage interface {
    Name() string
    Execute(ctx context.Context, input any) (output any, err error)
}

// Pipeline 管道执行器
type Pipeline struct {
    stages []Stage
}

func NewPipeline(stages ...Stage) *Pipeline
func (p *Pipeline) Execute(ctx context.Context, input any) (output any, err error)
```

### 5.2 阶段列表

| 阶段 | 输入 | 输出 | 职责 |
|------|------|------|------|
| `ParseStage` | 文件路径 | `*Sheet` | 读取 CSV，解析三行头 |
| `ValidateStage` | `*Sheet` | `*Sheet` | 校验变量名、类型、数据行格式 |
| `JSONGenStage` | `*Sheet` + opts | `*Payload` | 生成 json_payload |
| `MySQLStage` | `*Payload` + opts | `*Payload` | 写入 MySQL（可选） |
| `RedisStage` | `config_name`, `revision` | - | PUBLISH 通知（可选） |
| `GoGenStage` | `*Sheet` + opts | 文件路径 | 生成 Go struct（可选） |

---

## 6. CLI 接口

### 6.1 命令示例

```bash
# 校验 + 产出模式（输出到文件）
config-loader -input ./CheckinDaily--daily.csv -output ./out.json

# 写库 + 通知模式
config-loader \
  -input ./CheckinDaily--daily.csv \
  -mysql-dsn "root:root@tcp(localhost:3306)/kd48?parseTime=true" \
  -redis-addr "localhost:6379" \
  -scope checkin \
  -title "每日签到配置"

# Go 代码生成
config-loader \
  -input ./CheckinDaily--daily.csv \
  -gen-go \
  -go-out ./generated/checkin.go \
  -go-package lobbyconfig

# 完整模式
config-loader \
  -input ./CheckinDaily--daily.csv \
  -mysql-dsn "..." \
  -redis-addr "..." \
  -scope checkin \
  -title "每日签到配置" \
  -gen-go \
  -go-out ./generated/checkin.go
```

### 6.2 参数列表

| 参数 | 必填 | 说明 |
|------|------|------|
| `-input` | ✅ | CSV 文件路径 |
| `-output` | ❌ | JSON 输出文件路径（默认 stdout） |
| `-mysql-dsn` | ❌ | MySQL 连接串，提供则写入数据库 |
| `-redis-addr` | ❌ | Redis 地址，提供则发送通知 |
| `-scope` | ❌ | 业务域（checkin/reward/rank/task） |
| `-title` | ❌ | 配置标题 |
| `-revision` | ❌ | 显式指定版本号（默认 unix_millis） |
| `-gen-go` | ❌ | 启用 Go 代码生成 |
| `-go-out` | ❌ | Go 文件输出路径 |
| `-go-package` | ❌ | Go 包名（默认 lobbyconfig） |
| `-dry-run` | ❌ | 校验但不写入 |
| `-verbose` | ❌ | 详细日志 |

### 6.3 退出码

| 码 | 含义 |
|----|------|
| 0 | 成功 |
| 1 | 参数错误 |
| 2 | CSV 解析错误 |
| 3 | 校验失败 |
| 4 | MySQL 写入失败 |
| 5 | Go 生成失败 |

---

## 7. Go 代码生成

### 7.1 类型映射

| CSV 类型 | Go 类型 | JSON 处理 |
|----------|---------|-----------|
| `int32` | `int32` | 标准解析 |
| `int64` | `int64` | 标准解析 |
| `string` | `string` | 标准解析 |
| `time` | `ConfigTime` | 自定义解析（见 7.2） |
| `int32[]` | `[]int32` | 标准解析 |
| `int64[]` | `[]int64` | 标准解析 |
| `string[]` | `[]string` | 标准解析 |
| `int32=string` | `map[int32]string` | 标准解析 |
| `string=int64` | `map[string]int64` | 标准解析 |
| `string=string` | `map[string]string` | 标准解析 |

### 7.2 ConfigTime 类型

```go
// ConfigTime 包装 time.Time，支持自定义 JSON 解析
type ConfigTime struct {
    Raw  string    `json:"-"` // 原始字符串值
    Time time.Time `json:"-"` // 解析后的时间
}

func (t *ConfigTime) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t.Raw = s
    parsed, err := time.ParseInLocation(TimeFormat, s, time.Local)
    if err != nil {
        return err
    }
    t.Time = parsed
    return nil
}

func (t ConfigTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(t.Raw)
}

func (t ConfigTime) String() string {
    return t.Raw
}
```

### 7.3 生成示例

输入 CSV：
```csv
奖励说明,数量,标签,开始时间
note,amount,tags,start_time
string,int32,string[],time
首登奖,10,'vip'|'hot',2026-04-15 10:00:00
```

生成代码：
```go
// Code generated by config-loader. DO NOT EDIT.

package lobbyconfig

import (
    "encoding/json"
    "time"
)

// TimeFormat 配置时间格式
const TimeFormat = "2006-01-02 15:04:05"

// ConfigTime 包装 time.Time，支持自定义 JSON 解析
type ConfigTime struct {
    Raw  string    `json:"-"`
    Time time.Time `json:"-"`
}

func (t *ConfigTime) UnmarshalJSON(data []byte) error {
    var s string
    if err := json.Unmarshal(data, &s); err != nil {
        return err
    }
    t.Raw = s
    parsed, err := time.ParseInLocation(TimeFormat, s, time.Local)
    if err != nil {
        return err
    }
    t.Time = parsed
    return nil
}

func (t ConfigTime) MarshalJSON() ([]byte, error) {
    return json.Marshal(t.Raw)
}

func (t ConfigTime) String() string {
    return t.Raw
}

// CheckinRow 表示配置数据行
type CheckinRow struct {
    Note      string   `json:"note"`
    Amount    int32    `json:"amount"`
    Tags      []string `json:"tags"`
    StartTime ConfigTime `json:"start_time"`
}

// CheckinConfig 表示完整配置
type CheckinConfig struct {
    ConfigName string       `json:"config_name"`
    Revision   int64        `json:"revision"`
    Data       []CheckinRow `json:"data"`
}

// ParseCheckinConfig 从 JSON 解析配置
func ParseCheckinConfig(data []byte) (*CheckinConfig, error) {
    var cfg CheckinConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
```

---

## 8. 错误处理

### 8.1 错误类型

```go
// internal/errors/errors.go

type Error struct {
    Code    ErrorCode
    Message string
    Line    int    // CSV 行号（0-based，-1 表示不适用）
    Column  int    // CSV 列号（0-based，-1 表示不适用）
    Raw     string // 原始值
}

type ErrorCode int

const (
    ErrInvalidCSV      ErrorCode = iota // CSV 格式错误
    ErrInvalidHeader                    // 表头错误
    ErrInvalidTypeName                  // 类型名无效
    ErrInvalidValue                     // 值解析失败
    ErrTimeEmpty                        // time 字段为空
    ErrDuplicateColumn                  // 列名重复
    ErrMySQLWrite                       // MySQL 写入失败
    ErrRedisPublish                     // Redis 发布失败
    ErrGoGenerate                       // Go 生成失败
)

func (e *Error) Error() string {
    if e.Line >= 0 && e.Column >= 0 {
        return fmt.Sprintf("[%d:%d] %s: %s", e.Line, e.Column, e.Message, e.Raw)
    }
    return fmt.Sprintf("%s: %s", e.Message, e.Raw)
}
```

### 8.2 错误输出示例

```
[4:2] invalid int32 value: "abc"
[5:3] time field cannot be empty
```

### 8.3 Redis 失败处理

MySQL 写入成功但 Redis PUBLISH 失败时：
- 记录错误日志
- 退出码 0（数据已落库，通知可补发）
- 不回滚 MySQL

---

## 9. 测试策略

按 AGENTS.md 强制 TDD，测试覆盖目标 > 80%。

```
internal/
├── csvparser/
│   ├── parser.go
│   ├── parser_test.go      # 解析三行头、数据行
│   ├── types.go
│   └── types_test.go       # 类型解析
├── validator/
│   ├── validator.go
│   └── validator_test.go   # 变量名正则、空值约束、time 必填
├── jsongen/
│   ├── generator.go
│   └── generator_test.go   # 对照规格 §4 示例验证输出
├── mysqlwriter/
│   ├── writer.go
│   └── writer_test.go      # 使用 sqlmock 测试
├── redisnotify/
│   ├── publisher.go
│   └── publisher_test.go   # 使用 miniredis 测试
├── gogen/
│   ├── generator.go
│   └── generator_test.go   # 验证生成代码可编译
└── pipeline/
    ├── pipeline.go
    └── pipeline_test.go    # 端到端管道测试
```

---

## 10. 与现有规格文档的关系

**单一信源**：`docs/superpowers/specs/2026-04-16-lobby-config-csv-and-tooling-spec.md`

| 规格章节 | 本设计实现 |
|----------|------------|
| §2 CSV 格式 | `csvparser` |
| §3 json_payload 形状 | `jsongen` |
| §5 打表工具流水线 | `pipeline` |
| 文件名规范 | `config_name` 推导逻辑 |

**本设计新增**：
- Go 代码生成规则
- CLI 参数设计
- 错误类型定义
- 测试策略

---

## 批准记录（人类门闩）

- **状态**：待批准
- **批准范围**：（手填）
- **批准人 / 日期**：（手填）
- **TDD**：强制
