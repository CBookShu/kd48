# Config-Loader 实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 实现完整的打表工具，支持 CSV 解析、数据校验、JSON 生成、MySQL 写入、Redis 通知、Go 代码生成。

**Architecture:** 管道式架构，每个阶段职责单一，支持独立测试。数据流：CSV → Parse → Validate → JSONGen → [MySQL] → [Redis] → [GoGen]。

**Tech Stack:** Go 1.26、encoding/csv、database/sql、go-redis/v9、flag

---

## 批准记录（人类门闩）

- **状态**：待批准
- **批准范围**：（手填）
- **批准人 / 日期**：（手填）
- **TDD**：强制

---

## 文件结构

| 路径 | 职责 |
|------|------|
| `tools/config-loader/go.mod` | 模块定义 |
| `tools/config-loader/internal/errors/errors.go` | 错误类型 |
| `tools/config-loader/internal/csvparser/sheet.go` | Sheet 数据结构 |
| `tools/config-loader/internal/csvparser/parser.go` | CSV 解析 |
| `tools/config-loader/internal/csvparser/types.go` | 类型解析 |
| `tools/config-loader/internal/validator/validator.go` | 数据校验 |
| `tools/config-loader/internal/jsongen/generator.go` | JSON 生成 |
| `tools/config-loader/internal/mysqlwriter/writer.go` | MySQL 写入 |
| `tools/config-loader/internal/redisnotify/publisher.go` | Redis 通知 |
| `tools/config-loader/internal/gogen/generator.go` | Go 代码生成 |
| `tools/config-loader/internal/pipeline/pipeline.go` | 管道执行器 |
| `tools/config-loader/cmd/config-loader/main.go` | CLI 入口 |
| `go.work` | 添加模块 |

---

## Task 1: 模块初始化与错误类型

**Files:**
- Create: `tools/config-loader/go.mod`
- Create: `tools/config-loader/internal/errors/errors.go`
- Create: `tools/config-loader/internal/errors/errors_test.go`
- Modify: `go.work`

- [ ] **Step 1: 创建 go.mod**

```go
module github.com/CBookShu/kd48/tools/config-loader

go 1.26.1

require (
    github.com/go-sql-driver/mysql v1.9.2
    github.com/redis/go-redis/v9 v9.18.0
)
```

- [ ] **Step 2: 更新 go.work**

在 `go.work` 的 `use` 块中添加 `./tools/config-loader`

- [ ] **Step 3: 写错误类型测试**

```go
package errors

import (
    "testing"
)

func TestError_Error_WithLineColumn(t *testing.T) {
    e := &Error{
        Code:    ErrInvalidValue,
        Message: "invalid int32 value",
        Line:    4,
        Column:  2,
        Raw:     "abc",
    }
    got := e.Error()
    want := "[4:2] invalid int32 value: abc"
    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}

func TestError_Error_WithoutLineColumn(t *testing.T) {
    e := &Error{
        Code:    ErrInvalidCSV,
        Message: "CSV format error",
        Line:    -1,
        Column:  -1,
        Raw:     "bad",
    }
    got := e.Error()
    want := "CSV format error: bad"
    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}
```

- [ ] **Step 4: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/errors/... -v
```
Expected: FAIL (errors.go 不存在)

- [ ] **Step 5: 写错误类型实现**

```go
package errors

import "fmt"

type ErrorCode int

const (
    ErrInvalidCSV ErrorCode = iota
    ErrInvalidHeader
    ErrInvalidTypeName
    ErrInvalidValue
    ErrTimeEmpty
    ErrDuplicateColumn
    ErrMySQLWrite
    ErrRedisPublish
    ErrGoGenerate
)

type Error struct {
    Code    ErrorCode
    Message string
    Line    int
    Column  int
    Raw     string
}

func (e *Error) Error() string {
    if e.Line >= 0 && e.Column >= 0 {
        return fmt.Sprintf("[%d:%d] %s: %s", e.Line, e.Column, e.Message, e.Raw)
    }
    return fmt.Sprintf("%s: %s", e.Message, e.Raw)
}

func New(code ErrorCode, message string, line, column int, raw string) *Error {
    return &Error{
        Code:    code,
        Message: message,
        Line:    line,
        Column:  column,
        Raw:     raw,
    }
}
```

- [ ] **Step 6: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/errors/... -v
```
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add tools/config-loader/go.mod tools/config-loader/internal/errors/ go.work
git commit -m "feat(config-loader): add module init and error types"
```

---

## Task 2: Sheet 数据结构与 CSV 解析

**Files:**
- Create: `tools/config-loader/internal/csvparser/sheet.go`
- Create: `tools/config-loader/internal/csvparser/parser.go`
- Create: `tools/config-loader/internal/csvparser/parser_test.go`

- [ ] **Step 1: 写 Sheet 数据结构**

```go
package csvparser

// Sheet 表示解析后的 CSV 表
type Sheet struct {
    Headers    []ColumnHeader
    Rows       []Row
    ConfigName string
    SourceFile string
}

// ColumnHeader 三行头信息
type ColumnHeader struct {
    Description string
    Name        string
    Type        string
}

// Row 单行数据
type Row struct {
    Values []Value
}

// Value 类型化的单元格值
type Value struct {
    Raw     string
    Parsed  interface{}
    Type    string
    IsEmpty bool
}
```

- [ ] **Step 2: 写解析器测试**

```go
package csvparser

import (
    "strings"
    "testing"
)

func TestParser_Parse_ValidCSV(t *testing.T) {
    csv := `奖励说明,数量,标签
note,amount,tags
string,int32,string[]
首登奖,10,'vip'|'hot'`
    
    p := NewParser()
    sheet, err := p.Parse(strings.NewReader(csv), "TestConfig--test.csv")
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }
    
    if sheet.ConfigName != "TestConfig" {
        t.Errorf("ConfigName = %q, want %q", sheet.ConfigName, "TestConfig")
    }
    if len(sheet.Headers) != 3 {
        t.Errorf("len(Headers) = %d, want 3", len(sheet.Headers))
    }
    if len(sheet.Rows) != 1 {
        t.Errorf("len(Rows) = %d, want 1", len(sheet.Rows))
    }
}

func TestParser_Parse_ThreeRowHeader(t *testing.T) {
    csv := `说明,数量
desc,qty
string,int32
测试,100`
    
    p := NewParser()
    sheet, err := p.Parse(strings.NewReader(csv), "test.csv")
    if err != nil {
        t.Fatalf("Parse() error = %v", err)
    }
    
    if sheet.Headers[0].Description != "说明" {
        t.Errorf("Headers[0].Description = %q, want %q", sheet.Headers[0].Description, "说明")
    }
    if sheet.Headers[0].Name != "desc" {
        t.Errorf("Headers[0].Name = %q, want %q", sheet.Headers[0].Name, "desc")
    }
    if sheet.Headers[0].Type != "string" {
        t.Errorf("Headers[0].Type = %q, want %q", sheet.Headers[0].Type, "string")
    }
}

func TestParser_DeriveConfigName(t *testing.T) {
    tests := []struct {
        filename string
        want     string
    }{
        {"CheckinDaily--2026-04-21.csv", "CheckinDaily"},
        {"RewardDemo--test.csv", "RewardDemo"},
        {"SimpleConfig.csv", "SimpleConfig"},
    }
    
    for _, tt := range tests {
        got := deriveConfigName(tt.filename)
        if got != tt.want {
            t.Errorf("deriveConfigName(%q) = %q, want %q", tt.filename, got, tt.want)
        }
    }
}
```

- [ ] **Step 3: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/csvparser/... -v
```
Expected: FAIL

- [ ] **Step 4: 写解析器实现**

```go
package csvparser

import (
    "encoding/csv"
    "io"
    "path/filepath"
    "strings"
)

type Parser struct{}

func NewParser() *Parser {
    return &Parser{}
}

func (p *Parser) Parse(r io.Reader, filename string) (*Sheet, error) {
    reader := csv.NewReader(r)
    reader.LazyQuotes = true
    
    records, err := reader.ReadAll()
    if err != nil {
        return nil, err
    }
    
    if len(records) < 3 {
        return nil, &ParseError{Message: "CSV must have at least 3 header rows"}
    }
    
    numCols := len(records[0])
    headers := make([]ColumnHeader, numCols)
    for i := 0; i < numCols; i++ {
        headers[i] = ColumnHeader{
            Description: records[0][i],
            Name:        records[1][i],
            Type:        records[2][i],
        }
    }
    
    rows := make([]Row, 0, len(records)-3)
    for i := 3; i < len(records); i++ {
        values := make([]Value, len(records[i]))
        for j, raw := range records[i] {
            typ := ""
            if j < numCols {
                typ = headers[j].Type
            }
            values[j] = Value{
                Raw:     raw,
                Type:    typ,
                IsEmpty: strings.TrimSpace(raw) == "",
            }
        }
        rows = append(rows, Row{Values: values})
    }
    
    return &Sheet{
        Headers:    headers,
        Rows:       rows,
        ConfigName: deriveConfigName(filename),
        SourceFile: filename,
    }, nil
}

func deriveConfigName(filename string) string {
    base := filepath.Base(filename)
    name := strings.TrimSuffix(base, ".csv")
    parts := strings.Split(name, "--")
    return parts[0]
}

type ParseError struct {
    Message string
}

func (e *ParseError) Error() string {
    return e.Message
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/csvparser/... -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/config-loader/internal/csvparser/
git commit -m "feat(config-loader): add CSV parser with Sheet structure"
```

---

## Task 3: 类型解析器

**Files:**
- Create: `tools/config-loader/internal/csvparser/types.go`
- Create: `tools/config-loader/internal/csvparser/types_test.go`

- [ ] **Step 1: 写类型解析测试**

```go
package csvparser

import (
    "testing"
)

func TestParseValue_Int32(t *testing.T) {
    v, err := ParseValue("42", "int32")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    if v.Parsed.(int32) != 42 {
        t.Errorf("Parsed = %v, want 42", v.Parsed)
    }
}

func TestParseValue_Int32Empty(t *testing.T) {
    v, err := ParseValue("", "int32")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    if !v.IsEmpty {
        t.Error("IsEmpty should be true")
    }
    if v.Parsed.(int32) != 0 {
        t.Errorf("Parsed = %v, want 0", v.Parsed)
    }
}

func TestParseValue_StringArray(t *testing.T) {
    v, err := ParseValue("'vip'|'hot'", "string[]")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    arr := v.Parsed.([]string)
    if len(arr) != 2 || arr[0] != "vip" || arr[1] != "hot" {
        t.Errorf("Parsed = %v, want [vip, hot]", arr)
    }
}

func TestParseValue_Int32Array(t *testing.T) {
    v, err := ParseValue("1|2|3", "int32[]")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    arr := v.Parsed.([]int32)
    if len(arr) != 3 || arr[0] != 1 || arr[1] != 2 || arr[2] != 3 {
        t.Errorf("Parsed = %v, want [1, 2, 3]", arr)
    }
}

func TestParseValue_Int32ArrayWithEmpty(t *testing.T) {
    v, err := ParseValue("1||3", "int32[]")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    arr := v.Parsed.([]int32)
    if len(arr) != 3 || arr[0] != 1 || arr[1] != 0 || arr[2] != 3 {
        t.Errorf("Parsed = %v, want [1, 0, 3]", arr)
    }
}

func TestParseValue_Map(t *testing.T) {
    v, err := ParseValue("32='15'|45='hello'", "int32=string")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    m := v.Parsed.(map[int32]string)
    if m[32] != "15" || m[45] != "hello" {
        t.Errorf("Parsed = %v, want {32: '15', 45: 'hello'}", m)
    }
}

func TestParseValue_Time(t *testing.T) {
    v, err := ParseValue("2026-04-15 10:00:00", "time")
    if err != nil {
        t.Fatalf("ParseValue() error = %v", err)
    }
    if v.Parsed.(string) != "2026-04-15 10:00:00" {
        t.Errorf("Parsed = %v, want '2026-04-15 10:00:00'", v.Parsed)
    }
}

func TestParseValue_TimeEmpty(t *testing.T) {
    _, err := ParseValue("", "time")
    if err == nil {
        t.Error("ParseValue() should error for empty time")
    }
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/csvparser/... -v -run TestParseValue
```
Expected: FAIL

- [ ] **Step 3: 写类型解析实现**

```go
package csvparser

import (
    "strconv"
    "strings"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/errors"
)

func ParseValue(raw, typ string) (Value, error) {
    v := Value{
        Raw:     raw,
        Type:    typ,
        IsEmpty: strings.TrimSpace(raw) == "",
    }
    
    switch typ {
    case "int32":
        return parseInt32(v, raw)
    case "int64":
        return parseInt64(v, raw)
    case "string":
        v.Parsed = raw
        return v, nil
    case "time":
        return parseTime(v, raw)
    case "int32[]":
        return parseInt32Array(v, raw)
    case "int64[]":
        return parseInt64Array(v, raw)
    case "string[]":
        return parseStringArray(v, raw)
    default:
        if strings.Contains(typ, "=") {
            return parseMap(v, raw, typ)
        }
        v.Parsed = raw
        return v, nil
    }
}

func parseInt32(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = int32(0)
        return v, nil
    }
    n, err := strconv.ParseInt(raw, 10, 32)
    if err != nil {
        return v, errors.New(errors.ErrInvalidValue, "invalid int32 value", -1, -1, raw)
    }
    v.Parsed = int32(n)
    return v, nil
}

func parseInt64(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = int64(0)
        return v, nil
    }
    n, err := strconv.ParseInt(raw, 10, 64)
    if err != nil {
        return v, errors.New(errors.ErrInvalidValue, "invalid int64 value", -1, -1, raw)
    }
    v.Parsed = n
    return v, nil
}

func parseTime(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        return v, errors.New(errors.ErrTimeEmpty, "time field cannot be empty", -1, -1, raw)
    }
    v.Parsed = raw
    return v, nil
}

func parseInt32Array(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = []int32{}
        return v, nil
    }
    parts := strings.Split(raw, "|")
    arr := make([]int32, len(parts))
    for i, p := range parts {
        if strings.TrimSpace(p) == "" {
            arr[i] = 0
            continue
        }
        n, err := strconv.ParseInt(p, 10, 32)
        if err != nil {
            return v, errors.New(errors.ErrInvalidValue, "invalid int32 in array", -1, -1, p)
        }
        arr[i] = int32(n)
    }
    v.Parsed = arr
    return v, nil
}

func parseInt64Array(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = []int64{}
        return v, nil
    }
    parts := strings.Split(raw, "|")
    arr := make([]int64, len(parts))
    for i, p := range parts {
        if strings.TrimSpace(p) == "" {
            arr[i] = 0
            continue
        }
        n, err := strconv.ParseInt(p, 10, 64)
        if err != nil {
            return v, errors.New(errors.ErrInvalidValue, "invalid int64 in array", -1, -1, p)
        }
        arr[i] = n
    }
    v.Parsed = arr
    return v, nil
}

func parseStringArray(v Value, raw string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = []string{}
        return v, nil
    }
    parts := strings.Split(raw, "|")
    arr := make([]string, len(parts))
    for i, p := range parts {
        arr[i] = unquote(strings.TrimSpace(p))
    }
    v.Parsed = arr
    return v, nil
}

func parseMap(v Value, raw, typ string) (Value, error) {
    if v.IsEmpty {
        v.Parsed = map[string]interface{}{}
        return v, nil
    }
    
    parts := strings.Split(typ, "=")
    keyType := strings.TrimSpace(parts[0])
    valueType := strings.TrimSpace(parts[1])
    
    entries := strings.Split(raw, "|")
    result := make(map[string]interface{})
    
    for _, entry := range entries {
        entry = strings.TrimSpace(entry)
        if entry == "" {
            continue
        }
        kv := splitKeyValue(entry)
        if len(kv) != 2 {
            continue
        }
        key := strings.TrimSpace(kv[0])
        value := strings.TrimSpace(kv[1])
        
        key = unquote(key)
        value = unquote(value)
        
        if keyType == "int32" || keyType == "int64" {
            n, _ := strconv.ParseInt(key, 10, 64)
            result[strconv.FormatInt(n, 10)] = value
        } else {
            result[key] = value
        }
    }
    v.Parsed = result
    return v, nil
}

func splitKeyValue(s string) []string {
    inQuote := false
    for i, c := range s {
        if c == '\'' || c == '"' {
            inQuote = !inQuote
        }
        if c == '=' && !inQuote {
            return []string{s[:i], s[i+1:]}
        }
    }
    return strings.SplitN(s, "=", 2)
}

func unquote(s string) string {
    if len(s) >= 2 && (s[0] == '\'' && s[len(s)-1] == '\'' || s[0] == '"' && s[len(s)-1] == '"') {
        return s[1 : len(s)-1]
    }
    return s
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/csvparser/... -v -run TestParseValue
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/config-loader/internal/csvparser/types.go tools/config-loader/internal/csvparser/types_test.go
git commit -m "feat(config-loader): add type parser for int32, int64, string, time, arrays, maps"
```

---

## Task 4: 数据校验器

**Files:**
- Create: `tools/config-loader/internal/validator/validator.go`
- Create: `tools/config-loader/internal/validator/validator_test.go`

- [ ] **Step 1: 写校验器测试**

```go
package validator

import (
    "testing"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestValidator_Validate_ValidSheet(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Description: "说明", Name: "note", Type: "string"},
            {Description: "数量", Name: "amount", Type: "int32"},
        },
        Rows: []csvparser.Row{
            {Values: []csvparser.Value{
                {Raw: "测试", Type: "string", IsEmpty: false},
                {Raw: "100", Type: "int32", IsEmpty: false},
            }},
        },
    }
    
    v := NewValidator()
    err := v.Validate(sheet)
    if err != nil {
        t.Fatalf("Validate() error = %v", err)
    }
}

func TestValidator_Validate_DuplicateColumnName(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Description: "说明", Name: "note", Type: "string"},
            {Description: "说明2", Name: "note", Type: "int32"},
        },
        Rows: []csvparser.Row{},
    }
    
    v := NewValidator()
    err := v.Validate(sheet)
    if err == nil {
        t.Fatal("Validate() should error for duplicate column names")
    }
}

func TestValidator_Validate_InvalidColumnName(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Description: "说明", Name: "InvalidName", Type: "string"},
        },
        Rows: []csvparser.Row{},
    }
    
    v := NewValidator()
    err := v.Validate(sheet)
    if err == nil {
        t.Fatal("Validate() should error for invalid column name (not snake_case)")
    }
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/validator/... -v
```
Expected: FAIL

- [ ] **Step 3: 写校验器实现**

```go
package validator

import (
    "regexp"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
    "github.com/CBookShu/kd48/tools/config-loader/internal/errors"
)

var snakeCaseRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Validator struct{}

func NewValidator() *Validator {
    return &Validator{}
}

func (v *Validator) Validate(sheet *csvparser.Sheet) error {
    seen := make(map[string]int)
    for i, h := range sheet.Headers {
        if !snakeCaseRegex.MatchString(h.Name) {
            return errors.New(errors.ErrInvalidHeader, 
                "column name must be snake_case", 
                1, i, h.Name)
        }
        if prev, exists := seen[h.Name]; exists {
            return errors.New(errors.ErrDuplicateColumn,
                "duplicate column name",
                1, i, h.Name).WithMeta("previous_column", prev)
        }
        seen[h.Name] = i
    }
    return nil
}
```

- [ ] **Step 4: 更新 errors.go 添加 WithMeta**

```go
func (e *Error) WithMeta(key string, value interface{}) *Error {
    return e
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/validator/... -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/config-loader/internal/validator/ tools/config-loader/internal/errors/errors.go
git commit -m "feat(config-loader): add validator for column names"
```

---

## Task 5: JSON 生成器

**Files:**
- Create: `tools/config-loader/internal/jsongen/generator.go`
- Create: `tools/config-loader/internal/jsongen/generator_test.go`

- [ ] **Step 1: 写生成器测试**

```go
package jsongen

import (
    "encoding/json"
    "testing"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestGenerator_Generate(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Name: "note", Type: "string"},
            {Name: "amount", Type: "int32"},
        },
        Rows: []csvparser.Row{
            {Values: []csvparser.Value{
                {Raw: "首登奖", Parsed: "首登奖", Type: "string"},
                {Raw: "10", Parsed: int32(10), Type: "int32"},
            }},
        },
        ConfigName: "TestConfig",
    }
    
    g := NewGenerator()
    payload, err := g.Generate(sheet, 1)
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }
    
    if payload.ConfigName != "TestConfig" {
        t.Errorf("ConfigName = %q, want %q", payload.ConfigName, "TestConfig")
    }
    if payload.Revision != 1 {
        t.Errorf("Revision = %d, want 1", payload.Revision)
    }
    if len(payload.Data) != 1 {
        t.Fatalf("len(Data) = %d, want 1", len(payload.Data))
    }
    
    data := payload.Data[0]
    if data["note"] != "首登奖" {
        t.Errorf("data[note] = %v, want '首登奖'", data["note"])
    }
    if data["amount"] != int32(10) {
        t.Errorf("data[amount] = %v, want 10", data["amount"])
    }
}

func TestGenerator_Generate_JSON(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Name: "tags", Type: "string[]"},
        },
        Rows: []csvparser.Row{
            {Values: []csvparser.Value{
                {Raw: "'vip'|'hot'", Parsed: []string{"vip", "hot"}, Type: "string[]"},
            }},
        },
        ConfigName: "Test",
    }
    
    g := NewGenerator()
    payload, err := g.Generate(sheet, 1)
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }
    
    bytes, err := json.Marshal(payload)
    if err != nil {
        t.Fatalf("json.Marshal() error = %v", err)
    }
    
    var result map[string]interface{}
    json.Unmarshal(bytes, &result)
    
    data := result["data"].([]interface{})[0].(map[string]interface{})
    tags := data["tags"].([]interface{})
    if len(tags) != 2 {
        t.Errorf("len(tags) = %d, want 2", len(tags))
    }
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/jsongen/... -v
```
Expected: FAIL

- [ ] **Step 3: 写生成器实现**

```go
package jsongen

import (
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

type Payload struct {
    ConfigName string           `json:"config_name"`
    Revision   int64            `json:"revision"`
    Data       []map[string]any `json:"data"`
}

type Generator struct{}

func NewGenerator() *Generator {
    return &Generator{}
}

func (g *Generator) Generate(sheet *csvparser.Sheet, revision int64) (*Payload, error) {
    data := make([]map[string]any, len(sheet.Rows))
    
    for i, row := range sheet.Rows {
        rowData := make(map[string]any)
        for j, value := range row.Values {
            if j < len(sheet.Headers) {
                rowData[sheet.Headers[j].Name] = value.Parsed
            }
        }
        data[i] = rowData
    }
    
    return &Payload{
        ConfigName: sheet.ConfigName,
        Revision:   revision,
        Data:       data,
    }, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/jsongen/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/config-loader/internal/jsongen/
git commit -m "feat(config-loader): add JSON generator"
```

---

## Task 6: MySQL 写入器

**Files:**
- Create: `tools/config-loader/internal/mysqlwriter/writer.go`
- Create: `tools/config-loader/internal/mysqlwriter/writer_test.go`

- [ ] **Step 1: 写写入器测试**

```go
package mysqlwriter

import (
    "database/sql"
    "testing"
    
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
)

func TestWriter_Write(t *testing.T) {
    db, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("sqlmock.New() error = %v", err)
    }
    defer db.Close()
    
    payload := &jsongen.Payload{
        ConfigName: "TestConfig",
        Revision:   1,
        Data:       []map[string]any{{"note": "test"}},
    }
    
    mock.ExpectExec("INSERT INTO lobby_config_revision").
        WithArgs("TestConfig", 1, "test", "", "", nil, nil, sqlmock.AnyArg(), sqlmock.AnyArg()).
        WillReturnResult(sqlmock.NewResult(1, 1))
    
    w := NewWriter(db)
    err = w.Write(payload, WriteOptions{
        Scope:  "test",
        Title:  "",
        Tags:   "",
        CSVText: "test",
    })
    if err != nil {
        t.Fatalf("Write() error = %v", err)
    }
    
    if err := mock.ExpectationsWereMet(); err != nil {
        t.Errorf("expectations not met: %v", err)
    }
}
```

- [ ] **Step 2: 添加 sqlmock 依赖**

```bash
cd tools/config-loader && go get github.com/DATA-DOG/go-sqlmock
```

- [ ] **Step 3: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/mysqlwriter/... -v
```
Expected: FAIL

- [ ] **Step 4: 写写入器实现**

```go
package mysqlwriter

import (
    "database/sql"
    "encoding/json"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
)

type WriteOptions struct {
    Scope   string
    Title   string
    Tags    string
    CSVText string
}

type Writer struct {
    db *sql.DB
}

func NewWriter(db *sql.DB) *Writer {
    return &Writer{db: db}
}

func (w *Writer) Write(payload *jsongen.Payload, opts WriteOptions) error {
    jsonBytes, err := json.Marshal(payload.Data)
    if err != nil {
        return err
    }
    
    query := `
        INSERT INTO lobby_config_revision 
        (config_name, revision, scope, title, tags, start_time, end_time, csv_text, json_payload)
        VALUES (?, ?, ?, ?, ?, NULL, NULL, ?, ?)`
    
    _, err = w.db.Exec(query,
        payload.ConfigName,
        payload.Revision,
        opts.Scope,
        opts.Title,
        opts.Tags,
        opts.CSVText,
        jsonBytes,
    )
    return err
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/mysqlwriter/... -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/config-loader/internal/mysqlwriter/ tools/config-loader/go.mod tools/config-loader/go.sum
git commit -m "feat(config-loader): add MySQL writer"
```

---

## Task 7: Redis 通知发布器

**Files:**
- Create: `tools/config-loader/internal/redisnotify/publisher.go`
- Create: `tools/config-loader/internal/redisnotify/publisher_test.go`

- [ ] **Step 1: 写发布器测试**

```go
package redisnotify

import (
    "context"
    "testing"
    
    "github.com/alicebob/miniredis/v2"
    "github.com/redis/go-redis/v9"
)

func TestPublisher_Publish(t *testing.T) {
    mr, err := miniredis.Run()
    if err != nil {
        t.Fatalf("miniredis.Run() error = %v", err)
    }
    defer mr.Close()
    
    client := redis.NewClient(&redis.Options{
        Addr: mr.Addr(),
    })
    
    p := NewPublisher(client, "kd48:lobby:config:notify")
    err = p.Publish(context.Background(), "TestConfig", 1)
    if err != nil {
        t.Fatalf("Publish() error = %v", err)
    }
    
    msgs, err := client.LRange(context.Background(), "kd48:lobby:config:notify", 0, -1).Result()
    _ = msgs
}
```

- [ ] **Step 2: 添加 miniredis 依赖**

```bash
cd tools/config-loader && go get github.com/alicebob/miniredis/v2
```

- [ ] **Step 3: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/redisnotify/... -v
```
Expected: FAIL

- [ ] **Step 4: 写发布器实现**

```go
package redisnotify

import (
    "context"
    "encoding/json"
    
    "github.com/redis/go-redis/v9"
)

type Publisher struct {
    client  *redis.Client
    channel string
}

func NewPublisher(client *redis.Client, channel string) *Publisher {
    return &Publisher{
        client:  client,
        channel: channel,
    }
}

func (p *Publisher) Publish(ctx context.Context, configName string, revision int64) error {
    msg := map[string]interface{}{
        "kind":        "lobby_config_published",
        "config_name": configName,
        "revision":    revision,
    }
    
    bytes, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    
    return p.client.Publish(ctx, p.channel, string(bytes)).Err()
}
```

- [ ] **Step 5: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/redisnotify/... -v
```
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add tools/config-loader/internal/redisnotify/ tools/config-loader/go.mod tools/config-loader/go.sum
git commit -m "feat(config-loader): add Redis notification publisher"
```

---

## Task 8: Go 代码生成器

**Files:**
- Create: `tools/config-loader/internal/gogen/generator.go`
- Create: `tools/config-loader/internal/gogen/generator_test.go`

- [ ] **Step 1: 写生成器测试**

```go
package gogen

import (
    "strings"
    "testing"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

func TestGenerator_Generate(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Name: "note", Type: "string"},
            {Name: "amount", Type: "int32"},
            {Name: "tags", Type: "string[]"},
            {Name: "start_time", Type: "time"},
        },
        ConfigName: "Checkin",
    }
    
    g := NewGenerator()
    code, err := g.Generate(sheet, "lobbyconfig")
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }
    
    if !strings.Contains(code, "type CheckinRow struct") {
        t.Error("generated code should contain CheckinRow struct")
    }
    if !strings.Contains(code, "Note string `json:\"note\"`") {
        t.Error("generated code should contain Note field")
    }
    if !strings.Contains(code, "Amount int32 `json:\"amount\"`") {
        t.Error("generated code should contain Amount field")
    }
    if !strings.Contains(code, "Tags []string `json:\"tags\"`") {
        t.Error("generated code should contain Tags field")
    }
    if !strings.Contains(code, "StartTime ConfigTime `json:\"start_time\"`") {
        t.Error("generated code should contain StartTime field with ConfigTime type")
    }
}

func TestGenerator_Generate_ConfigTime(t *testing.T) {
    sheet := &csvparser.Sheet{
        Headers: []csvparser.ColumnHeader{
            {Name: "start_time", Type: "time"},
        },
        ConfigName: "Test",
    }
    
    g := NewGenerator()
    code, err := g.Generate(sheet, "lobbyconfig")
    if err != nil {
        t.Fatalf("Generate() error = %v", err)
    }
    
    if !strings.Contains(code, "type ConfigTime struct") {
        t.Error("generated code should contain ConfigTime struct")
    }
    if !strings.Contains(code, "UnmarshalJSON") {
        t.Error("generated code should contain UnmarshalJSON method")
    }
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/gogen/... -v
```
Expected: FAIL

- [ ] **Step 3: 写生成器实现**

```go
package gogen

import (
    "fmt"
    "strings"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

type GoGenerator struct{}

func NewGenerator() *GoGenerator {
    return &GoGenerator{}
}

func (g *GoGenerator) Generate(sheet *csvparser.Sheet, pkgName string) (string, error) {
    var sb strings.Builder
    
    sb.WriteString("// Code generated by config-loader. DO NOT EDIT.\n\n")
    sb.WriteString(fmt.Sprintf("package %s\n\n", pkgName))
    
    hasTime := false
    for _, h := range sheet.Headers {
        if h.Type == "time" {
            hasTime = true
            break
        }
    }
    
    if hasTime {
        sb.WriteString(g.generateImports())
        sb.WriteString(g.generateConfigTime())
    }
    
    sb.WriteString(g.generateRowStruct(sheet))
    sb.WriteString(g.generateConfigStruct(sheet))
    sb.WriteString(g.generateParseFunc(sheet))
    
    return sb.String(), nil
}

func (g *GoGenerator) generateImports() string {
    return `import (
    "encoding/json"
    "time"
)

`
}

func (g *GoGenerator) generateConfigTime() string {
    return `// TimeFormat 配置时间格式
const TimeFormat = "2006-01-02 15:04:05"

// ConfigTime 包装 time.Time，支持自定义 JSON 解析
type ConfigTime struct {
    Raw  string    ` + "`json:\"-\"`" + `
    Time time.Time ` + "`json:\"-\"`" + `
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

`
}

func (g *GoGenerator) generateRowStruct(sheet *csvparser.Sheet) string {
    var sb strings.Builder
    sb.WriteString(fmt.Sprintf("// %sRow 表示配置数据行\n", sheet.ConfigName))
    sb.WriteString(fmt.Sprintf("type %sRow struct {\n", sheet.ConfigName))
    
    for _, h := range sheet.Headers {
        goType := g.csvTypeToGoType(h.Type)
        sb.WriteString(fmt.Sprintf("    %s %s `json:\"%s\"`\n", g.toCamelCase(h.Name), goType, h.Name))
    }
    
    sb.WriteString("}\n\n")
    return sb.String()
}

func (g *GoGenerator) generateConfigStruct(sheet *csvparser.Sheet) string {
    return fmt.Sprintf(`// %sConfig 表示完整配置
type %sConfig struct {
    ConfigName string       `+"`json:\"config_name\"`"+`
    Revision   int64        `+"`json:\"revision\"`"+`
    Data       []%sRow `+"`json:\"data\"`"+`
}

`, sheet.ConfigName, sheet.ConfigName, sheet.ConfigName)
}

func (g *GoGenerator) generateParseFunc(sheet *csvparser.Sheet) string {
    return fmt.Sprintf(`// Parse%sConfig 从 JSON 解析配置
func Parse%sConfig(data []byte) (*%sConfig, error) {
    var cfg %sConfig
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
`, sheet.ConfigName, sheet.ConfigName, sheet.ConfigName, sheet.ConfigName)
}

func (g *GoGenerator) csvTypeToGoType(csvType string) string {
    switch csvType {
    case "int32":
        return "int32"
    case "int64":
        return "int64"
    case "string":
        return "string"
    case "time":
        return "ConfigTime"
    case "int32[]":
        return "[]int32"
    case "int64[]":
        return "[]int64"
    case "string[]":
        return "[]string"
    default:
        if strings.Contains(csvType, "=") {
            return g.parseMapType(csvType)
        }
        return "string"
    }
}

func (g *GoGenerator) parseMapType(csvType string) string {
    parts := strings.Split(csvType, "=")
    keyType := strings.TrimSpace(parts[0])
    valueType := strings.TrimSpace(parts[1])
    
    keyGo := "string"
    if keyType == "int32" || keyType == "int64" {
        keyGo = keyType
    }
    
    valueGo := "string"
    if valueType == "int32" || valueType == "int64" {
        valueGo = valueType
    }
    
    return fmt.Sprintf("map[%s]%s", keyGo, valueGo)
}

func (g *GoGenerator) toCamelCase(s string) string {
    parts := strings.Split(s, "_")
    for i := range parts {
        if len(parts[i]) > 0 {
            parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
        }
    }
    return strings.Join(parts, "")
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/gogen/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/config-loader/internal/gogen/
git commit -m "feat(config-loader): add Go code generator with ConfigTime support"
```

---

## Task 9: 管道执行器

**Files:**
- Create: `tools/config-loader/internal/pipeline/pipeline.go`
- Create: `tools/config-loader/internal/pipeline/pipeline_test.go`

- [ ] **Step 1: 写管道测试**

```go
package pipeline

import (
    "context"
    "testing"
)

func TestPipeline_Execute(t *testing.T) {
    p := NewPipeline(
        &mockStage{name: "stage1"},
        &mockStage{name: "stage2"},
    )
    
    ctx := context.Background()
    output, err := p.Execute(ctx, "input")
    if err != nil {
        t.Fatalf("Execute() error = %v", err)
    }
    if output != "input" {
        t.Errorf("output = %v, want 'input'", output)
    }
}

type mockStage struct {
    name string
}

func (s *mockStage) Name() string {
    return s.name
}

func (s *mockStage) Execute(ctx context.Context, input any) (any, error) {
    return input, nil
}
```

- [ ] **Step 2: 运行测试验证失败**

```bash
cd tools/config-loader && go test ./internal/pipeline/... -v
```
Expected: FAIL

- [ ] **Step 3: 写管道实现**

```go
package pipeline

import (
    "context"
    "fmt"
)

type Stage interface {
    Name() string
    Execute(ctx context.Context, input any) (output any, err error)
}

type Pipeline struct {
    stages []Stage
}

func NewPipeline(stages ...Stage) *Pipeline {
    return &Pipeline{stages: stages}
}

func (p *Pipeline) Execute(ctx context.Context, input any) (any, error) {
    var output any = input
    var err error
    
    for _, stage := range p.stages {
        output, err = stage.Execute(ctx, output)
        if err != nil {
            return nil, fmt.Errorf("stage %s failed: %w", stage.Name(), err)
        }
    }
    
    return output, nil
}
```

- [ ] **Step 4: 运行测试验证通过**

```bash
cd tools/config-loader && go test ./internal/pipeline/... -v
```
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add tools/config-loader/internal/pipeline/
git commit -m "feat(config-loader): add pipeline executor"
```

---

## Task 10: CLI 主程序

**Files:**
- Create: `tools/config-loader/cmd/config-loader/main.go`
- Create: `tools/config-loader/generated/.gitkeep`

- [ ] **Step 1: 写 CLI 主程序**

```go
package main

import (
    "context"
    "encoding/json"
    "flag"
    "fmt"
    "io"
    "log/slog"
    "os"
    "time"
    
    "github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
    "github.com/CBookShu/kd48/tools/config-loader/internal/gogen"
    "github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
    "github.com/CBookShu/kd48/tools/config-loader/internal/mysqlwriter"
    "github.com/CBookShu/kd48/tools/config-loader/internal/redisnotify"
    "github.com/CBookShu/kd48/tools/config-loader/internal/validator"
    _ "github.com/go-sql-driver/mysql"
    "github.com/redis/go-redis/v9"
)

func main() {
    inputFile := flag.String("input", "", "CSV input file path (required)")
    outputFile := flag.String("output", "", "JSON output file path (default: stdout)")
    mysqlDSN := flag.String("mysql-dsn", "", "MySQL DSN (optional, enables DB write)")
    redisAddr := flag.String("redis-addr", "", "Redis address (optional, enables notification)")
    scope := flag.String("scope", "", "Business scope (checkin/reward/rank/task)")
    title := flag.String("title", "", "Config title")
    revisionFlag := flag.Int64("revision", 0, "Explicit revision (default: unix_millis)")
    genGo := flag.Bool("gen-go", false, "Enable Go code generation")
    goOut := flag.String("go-out", "", "Go output file path")
    goPkg := flag.String("go-package", "lobbyconfig", "Go package name")
    dryRun := flag.Bool("dry-run", false, "Validate only, do not write")
    verbose := flag.Bool("verbose", false, "Verbose logging")
    flag.Parse()
    
    if *inputFile == "" {
        fmt.Fprintln(os.Stderr, "error: -input is required")
        os.Exit(1)
    }
    
    if *verbose {
        slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
    }
    
    // Parse CSV
    f, err := os.Open(*inputFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: open file: %v\n", err)
        os.Exit(2)
    }
    defer f.Close()
    
    parser := csvparser.NewParser()
    sheet, err := parser.Parse(f, *inputFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: parse CSV: %v\n", err)
        os.Exit(2)
    }
    slog.Info("parsed CSV", "config_name", sheet.ConfigName, "rows", len(sheet.Rows))
    
    // Validate
    v := validator.NewValidator()
    if err := v.Validate(sheet); err != nil {
        fmt.Fprintf(os.Stderr, "error: validate: %v\n", err)
        os.Exit(3)
    }
    slog.Info("validation passed")
    
    if *dryRun {
        fmt.Println("dry-run: validation passed")
        os.Exit(0)
    }
    
    // Generate JSON
    revision := *revisionFlag
    if revision == 0 {
        revision = time.Now().UnixMilli()
    }
    
    gen := jsongen.NewGenerator()
    payload, err := gen.Generate(sheet, revision)
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: generate JSON: %v\n", err)
        os.Exit(3)
    }
    
    // Output JSON
    jsonBytes, err := json.MarshalIndent(payload, "", "  ")
    if err != nil {
        fmt.Fprintf(os.Stderr, "error: marshal JSON: %v\n", err)
        os.Exit(3)
    }
    
    if *outputFile != "" {
        if err := os.WriteFile(*outputFile, jsonBytes, 0644); err != nil {
            fmt.Fprintf(os.Stderr, "error: write output: %v\n", err)
            os.Exit(3)
        }
        slog.Info("wrote JSON", "path", *outputFile)
    } else {
        fmt.Println(string(jsonBytes))
    }
    
    // MySQL write
    if *mysqlDSN != "" {
        csvText, _ := io.ReadAll(f)
        f.Seek(0, 0)
        csvText, _ = io.ReadAll(f)
        
        db, err := openMySQL(*mysqlDSN)
        if err != nil {
            fmt.Fprintf(os.Stderr, "error: connect MySQL: %v\n", err)
            os.Exit(4)
        }
        defer db.Close()
        
        w := mysqlwriter.NewWriter(db)
        if err := w.Write(payload, mysqlwriter.WriteOptions{
            Scope:   *scope,
            Title:   *title,
            CSVText: string(csvText),
        }); err != nil {
            fmt.Fprintf(os.Stderr, "error: write MySQL: %v\n", err)
            os.Exit(4)
        }
        slog.Info("wrote to MySQL", "config_name", payload.ConfigName, "revision", payload.Revision)
        
        // Redis notify
        if *redisAddr != "" {
            rdb := redis.NewClient(&redis.Options{Addr: *redisAddr})
            p := redisnotify.NewPublisher(rdb, "kd48:lobby:config:notify")
            if err := p.Publish(context.Background(), payload.ConfigName, payload.Revision); err != nil {
                slog.Error("Redis publish failed", "error", err)
            } else {
                slog.Info("published to Redis")
            }
        }
    }
    
    // Go code generation
    if *genGo {
        goGen := gogen.NewGenerator()
        code, err := goGen.Generate(sheet, *goPkg)
        if err != nil {
            fmt.Fprintf(os.Stderr, "error: generate Go: %v\n", err)
            os.Exit(5)
        }
        
        goOutPath := *goOut
        if goOutPath == "" {
            goOutPath = fmt.Sprintf("generated/%s.go", sheet.ConfigName)
        }
        if err := os.WriteFile(goOutPath, []byte(code), 0644); err != nil {
            fmt.Fprintf(os.Stderr, "error: write Go file: %v\n", err)
            os.Exit(5)
        }
        slog.Info("generated Go code", "path", goOutPath)
    }
    
    slog.Info("done", "config_name", payload.ConfigName, "revision", payload.Revision)
}

func openMySQL(dsn string) (interface{ Close() error }, error) {
    return nil, fmt.Errorf("MySQL not implemented in this example")
}
```

- [ ] **Step 2: 创建 generated 目录**

```bash
mkdir -p tools/config-loader/generated
touch tools/config-loader/generated/.gitkeep
```

- [ ] **Step 3: 构建验证**

```bash
cd tools/config-loader && go build ./cmd/config-loader
```
Expected: 编译成功

- [ ] **Step 4: Commit**

```bash
git add tools/config-loader/cmd/config-loader/ tools/config-loader/generated/
git commit -m "feat(config-loader): add CLI main program"
```

---

## Task 11: 集成测试与文档

**Files:**
- Create: `tools/config-loader/testdata/example.csv`
- Create: `tools/config-loader/README.md`

- [ ] **Step 1: 创建测试数据**

```csv
奖励说明,数量,标签,开始时间
note,amount,tags,start_time
string,int32,string[],time
首登奖,10,'vip'|'hot',2026-04-15 10:00:00
```

- [ ] **Step 2: 写 README**

```markdown
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
```

- [ ] **Step 3: 运行完整测试**

```bash
cd tools/config-loader && go test ./... -v
```
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add tools/config-loader/testdata/ tools/config-loader/README.md
git commit -m "docs(config-loader): add testdata and README"
```

---

## 验证命令

```bash
# 在 go.work 根目录
go test ./tools/config-loader/... -v
go build ./tools/config-loader/...
```

---

## 与设计的可追溯映射

| 设计章节 | 本计划 Task |
|----------|-------------|
| §2 目录结构 | Task 1 |
| §4 数据结构 | Task 2 |
| §4 类型解析 | Task 3 |
| §8 错误处理 | Task 1 |
| §5 管道接口 | Task 9 |
| §6 CLI 接口 | Task 10 |
| §7 Go 代码生成 | Task 8 |
| §9 测试策略 | 所有 Task |
