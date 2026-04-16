# Lobby 策划配置：CSV 格式与打表工具规格

> **状态**：已落盘。  
> **关联**：[Lobby 服务设计](./2026-04-15-lobby-service-design.md) §4；[Lobby 实现计划](../plans/2026-04-15-lobby-service-implementation-plan.md)（MySQL **§C** 起、Redis、Task 映射）。  
> **日期**：2026-04-16  

本文档为 **配置 CSV（`sheet_v1`）** 与 **`json_payload`** 及 **打表工具** 的 **单一信源**；Lobby 运行时、打表 CLI、CI 校验须与本篇一致。

---

## 1. 文档范围

| 本文档覆盖 | 本文档不覆盖（见实现计划） |
|------------|----------------------------|
| 三行头 CSV 文法、类型、空值默认、map/array/string 规则 | **`lobby_config_revision` 表 DDL**、索引、**Redis** 频道细节 → 实现计划 **§C、§D** |
| **`json_payload`** 根对象与 `data[]` 形状 | Lobby 进程内 **ConfigLoader** 代码结构 |
| 打表工具 **输入/输出/顺序/报错** | 网关路由、Etcd、**Lobby gRPC** 业务 |

---

## 2. CSV 格式（`sheet_v1`）

### 2.1 编码与分隔

- 文件：**UTF-8**（不推荐 UTF-8 BOM）；换行 **LF**。  
- 列分隔：**逗号 `,`**；单元格内含逗号、引号、换行按 **RFC 4180** 用双引号包裹。  
- **禁止** 使用已废弃的 `##SCHEMA` / `##DATA` 分段格式。

### 2.2 三行表头 + 数据行

| 行号 | 含义 | 约束 |
|------|------|------|
| **第 1 行** | 中文说明 | 列数 = `N`；不参与类型解析 |
| **第 2 行** | **变量名**（JSON 键 / Go 字段名） | 全表唯一；**`snake_case`**，`^[a-z][a-z0-9_]*$` |
| **第 3 行** | **类型** | 见 **§2.5**；未识别为内置或 `K = V` map 形式 → **自定义类型**（须注册） |
| **第 4 行起** | **数据行** | 每行列数恒为 `N`；不允许仅空白的数据行 |

### 2.3 空值与默认（**不使用 `?` 后缀**）

以下指：**该列在表头中存在**，且某数据行单元格 **trim 后为空**。

| 类型 / 形态 | 空单元格 → JSON |
|-------------|-----------------|
| `int32` / `int64` | **`0`**（非法非空仍报错） |
| `string` | **`""`** |
| `int32[]` / `int64[]` / `string[]` | **`[]`** |
| `int32 = string` 等 map | **`{}`** |
| `time` | **不允许空** → **打表工具报错** |

**数组整格非空且含 `|`**：`int32[]` / `int64[]` 中 **空段**（如 `1||3`）→ 该元素 **`0`**。`string[]` 每段须 **`''`/`""` 包裹**，**裸空段非法**（空串元素写 `''`）。

**少一列**：第二行无某变量名 → 另一套表结构；**不**与「同表空格」混谈。

### 2.4 `time` 类型

- 单元格形态：**`YYYY-MM-DD HH:MM:SS`**（日期与时间之间 **一个空格**；24 小时制；**不含时区**）。  
- JSON：**同形 string**。  
- 解析：由部署约定 **单一时区**（如 Lobby `time_location`）解释；格式不符 **报错**。

### 2.5 内置类型与 Map

**标量**：`int32`、`int64`、`string`、`time`（见 §2.3、§2.4）。

**数组 `T[]`**：`T` ∈ `int32` / `int64` / `string`；`|` 分隔；`string[]` 元素须引号包裹（规则同实现计划历史稿：内嵌引号加倍或换外层引号）。

**Map 类型行**：`键类型 = 值类型`（`=` 两侧可空白）；M0 合法：`int32 = string`、`string = int64`、`string = string`。

**Map 单元格**：整格空 → `{}`；多条 `键 = 值` 用 `|` 分隔；单条内以 **第一个不在引号内的 `=`** 分键值；`string` 键/值须引号；`int32`/`int64` 键值不加引号。

**自定义类型**：未匹配内置/map 语法 → 打表工具注册表；未注册 → **报错**。

---

## 3. `json_payload` 形状（打表产出 / Lobby 消费）

**根对象（必须）**

| 键 | 类型 | 说明 |
|----|------|------|
| `config_id` | string | 与 MySQL 行一致（自检；实现可二选一只信 DB，须全链一致） |
| `revision` | number | 与 MySQL 行一致 |
| `data` | **array of object** | 每个元素 = CSV 一条数据行；键 = 第 2 行变量名 |

**不包含**：`config_format_version`（文法由工具 + Lobby **同版本**保证）。

---

## 4. 配置示例与转换结果

### 例 A：单数据行（与仓库 `exp.csv` 对齐）

**CSV（`csv_text` 原文）**

```csv
奖励说明,数量,标签,某映射
note,amount,tags,extra_map
string,int32,string[],int32 = string
首登奖,10,'vip'|'hot',32='15' | 45 = "hello"
```

**假定**打表入参 `config_id = "reward_demo"`、`revision = 1`，**生成的 `json_payload`（仅逻辑体，可嵌入 MySQL 行）**：

```json
{
  "config_id": "reward_demo",
  "revision": 1,
  "data": [
    {
      "note": "首登奖",
      "amount": 10,
      "tags": ["vip", "hot"],
      "extra_map": { "32": "15", "45": "hello" }
    }
  ]
}
```

### 例 B：空单元格 → 默认（含 `time` 必填）

**CSV**

```csv
说明,数量,标签,映射,开始
desc,qty,tags,meta,starts_at
string,int32,string[],int32 = string,time
,0,,,2026-04-15 10:00:00
```

**转换说明**（第 4 行）：

- `desc` 空 → `""`  
- `qty` 空 → `0`  
- `tags` 空 → `[]`  
- `meta` 空 → `{}`  
- `starts_at` 必填 → 有值  

**`json_payload`**

```json
{
  "config_id": "example_b",
  "revision": 1,
  "data": [
    {
      "desc": "",
      "qty": 0,
      "tags": [],
      "meta": {},
      "starts_at": "2026-04-15 10:00:00"
    }
  ]
}
```

（`config_id` / `revision` 由打表命令或流水线注入，与 DB 行一致。）

### 例 C：两行数据 + `int32[]` 空段 → `0`

**CSV**

```csv
标题,得分列表
title,scores
string,int32[]
第一行,'a'|1||3
第二行,'b'|10
```

**第一行** `scores`：`1||3` → `[1, 0, 3]`。

**`json_payload`（节选 `data`）**

```json
"data": [
  { "title": "第一行", "scores": [1, 0, 3] },
  { "title": "第二行", "scores": [10] }
]
```

---

## 5. 打表工具设计

### 5.1 定位

- **输入**：符合本篇 **§2** 的 CSV 文件（及命令行参数：`config_id`、`revision`、目标 DB 连接等）。  
- **输出**：**校验后的** `json_payload`；**写入 MySQL** `lobby_config_revision`（含 `csv_text`、列 `scope`/`title`/`tags`/`start_time`/`end_time` 等由 CLI 或配套配置提供，**表结构见实现计划 §C**）；**成功后** 向 Redis **`PUBLISH`**（见实现计划 **§D**）。

### 5.2 处理流水线（须实现）

1. **读入 CSV** → 校验三行头、列数、变量名正则。  
2. **逐数据行**：按第三行类型解析单元格；违反 **§2.3**（如 `time` 空）→ **退出码非 0**，**不写库**。  
3. **组装** `json_payload`（§3）。  
4. **事务**：`INSERT` MySQL（顺序：**先提交数据**）。  
5. **`PUBLISH`** Redis 通知（**仅**在 MySQL 提交成功后）。

### 5.3 与 Lobby 的边界

- **Lobby**：只 **消费** 已落库的 `json_payload`；**不**解析 CSV。  
- **打表工具**：**不**实现 Lobby 业务 RPC；**不**替代 Redis 订阅逻辑。

### 5.4 Go 代码生成（可选 Task）

- 由第 2、3 行生成 **`LobbySheetRow` 等价 struct** 与 `json` tag；与 **§3** 中 `data[]` 对象键一致。  
- CI：生成物已提交或 `make gen-config` 可复现。

---

## 6. 自检

- 三例（§4）与 **§2.3** 无矛盾。  
- 已标明 **`config_format_version` 不在 JSON**；**`time` 格式** 为 **`YYYY-MM-DD HH:MM:SS`** 无时区。
