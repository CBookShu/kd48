# Lobby 服务实现计划

> **For agentic workers:** 按 [`AGENTS.md`](../../../AGENTS.md) 执行：**TDD**（先失败测试再实现）、**verification-before-completion**（声称完成前须跑通约定命令）。多 Task 建议按 **subagent-driven-development** 一 Task 一会话或子代理。  
> **设计依据**：[Lobby 服务设计](../specs/2026-04-15-lobby-service-design.md)。

**Goal:** 落地 Lobby：**无状态 gRPC 服务**（Etcd 注册 `kd48/lobby-service`）、**MySQL 存 CSV+JSON 配置权威**、**Redis 推送变更**（Lobby **不** 以轮询 MySQL 作变更主路径）、**启动 bootstrap 读 MySQL**；经 **Gateway `GatewayIngress`** 暴露至少一条可调用 RPC（M0 打通链路）；**打表工具与 CSV→Go 代码生成** 可在后续 Task 或独立仓库实现，本计划先预留 **强类型反序列化** 的接口与占位生成物。

**Scope 边界**：本计划 **不** 实现完整任务/签到/排行业务逻辑；**不** 锁死各活动 Redis/MySQL 模型。首版以 **服务骨架 + 配置加载管线 + Ingress 路由打通** 为主。

## 批准记录（人类门闩）

- **状态**：待批准  
- **批准范围**：（手填，例如「全文」或「Task 1～4」）  
- **批准人 / 日期**：（手填）  
- **TDD**：强制（若豁免须写理由）  
- **Subagent**：按任务拆分 / 本步单会话豁免（若豁免须写理由）  

---

**Architecture:**

- **进程**：`services/lobby`，显式 DI（`sql.DB`、`redis.UniversalClient`、配置、OTel），`grpc.NewServer` 注册 **`lobby.v1.LobbyService`**（首版最小 RPC）与 **`gateway.v1.GatewayIngress`**（按 `IngressRequest.route` 用 **protojson** 分发给 Lobby RPC，与 `services/user/cmd/user/ingress.go` 同模式）。  
- **配置**：MySQL 表（名在迁移中最终确定）存 `config_id`、`revision`（或单调版本）、`csv_text`、`json_payload`（`LONGTEXT`/JSON）、`updated_at` 等；Lobby 内 **`atomic.Value` 或 `sync.RWMutex`** 持有只读快照。  
- **变更路径**：外部打表工具 **事务写 MySQL → `PUBLISH`（或 `XADD`）Redis**；Lobby **订阅** → 收到 `config_id`+`revision` → **单条 SELECT 拉 JSON** → 校验 → `json.Unmarshal` 到 **生成或手写的 struct**（首版可用 **`LobbyConfigEnvelope` + `Data` 为 `[]LobbySheetRow`** 或等价，与 **§B** `json_payload` 中 **`data` 数组** 对齐）。  
- **网关**：Etcd meta 增加 `kd48/lobby-service` 的 `ServiceType` + 至少一条 `WsRouteSpec`（`IngressRoute` 指向 `/lobby.v1.LobbyService/…`）；`seed-gateway-meta` 或等价种子更新。  
- **`go.work`**：追加 `./services/lobby`。

**Tech Stack:** 与 User 服务对齐：Go 1.26、`golang-migrate`、`sqlc`（若本计划引入查询则加 `sqlc.yaml`；否则手写极小 DAL + 单测）、`go-redis/v9`、OTel、`pkg/registry`、`pkg/conf` 模式。

---

## 配置与消息格式规范（M0 可执行约定）

本节把设计文档里「有规则 CSV / MySQL 双存 / Redis 通知 / Go 强类型」落实为 **实现与打表工具可照着做** 的固定格式。**CSV→JSON 解析规则为全局一套**（由打表工具与 Lobby **同版本演进**），**不在** `json_payload` 内携带 `config_format_version` 等「文法版本」字段；若将来出现不兼容文法变更，通过 **打表工具 / Lobby 发版** 与 **另起 `config_id` 或迁移脚本** 处理，而非按条 JSON 分支。

**§A 所述「三行头 CSV」**在文档与评审中可仍称 **`sheet_v1`**，仅表示 **该 CSV 形态的名称**，**不**写入 `json_payload`。

### A. 策划 CSV（三行头 / `sheet_v1`，写入 MySQL 的 `csv_text` 原文）

**编码与分隔**

- 文件：**UTF-8**（**不推荐** UTF-8 BOM）；换行 **LF**。  
- 列分隔：**逗号 `,`**；单元格内含逗号、引号、换行时按 **RFC 4180** 用双引号包裹字段。  
- **禁止** 使用旧版 `##SCHEMA` / `##DATA` 分段格式（已从本计划移除）。

**三行固定表头 + 数据行（单表、不嵌多子表）**

| 行号 | 含义 | 约束 |
|------|------|------|
| **第 1 行** | 中文说明（给人看） | 列数 = `N`；**不参与类型解析**；可与第 2、3 行列对齐校验 |
| **第 2 行** | **变量名**（JSON 键 / Go 字段名） | **全表唯一**；**`snake_case`**，正则 **`^[a-z][a-z0-9_]*$`** |
| **第 3 行** | **类型** | 见下文 **内置类型表**；未注册标识符视为 **自定义类型**（打表工具插件实现，失败则报错） |
| **第 4 行起** | **数据行** | 每行一条逻辑记录；**每行列数恒为 `N`**；表内 **不允许空数据行**（实现可在解析前 trim 并拒绝仅空白行） |

**可选列**：类型字面量后缀 **`?`**（如 `string?`、`int32[]?`）。**空单元格**：若带 `?` → JSON 该行对象中 **省略该键**；不带 `?` 且空 → **报错**。

---

**内置类型（定稿）**

**1）标量**

| 类型 | 单元格 → JSON |
|------|----------------|
| `int32` | JSON number（十进制，范围 int32） |
| `int64` | JSON number（十进制，范围 int64） |
| `string` | JSON string（建议 **trim 首尾空白** 后写入 JSON） |

**2）标量数组 `T[]`**（`T` ∈ `int32` / `int64` / `string`）

- 单元格内 **多个元素用 `|` 分隔**；`|` 两侧可有空白，解析时 trim。  
- **`int32[]` / `int64[]`**：分段后每段 trim；**不得出现空段**（如 `1||2` 非法）；每段解析为整数。  
- **`string[]`**：**每个元素必须** 用 **`'...'`** 或 **`"..."`** **整体包裹**（与 `|` 语法解耦）；**同一元素外层引号不成对**（如 `'ab"`）非法。  
  - 外层为 **`'`** 时，元素内单引号写 **`''`**（加倍）；或改用外层 **`"`** 包裹该元素。  
  - 外层为 **`"`** 时，元素内双引号写 **`""`**；或改用 **`'`** 包裹该元素。  
  - 允许空串元素 **`''`** / `""`。  
- **解析顺序**：先 RFC 4180 得到该列「逻辑字符串」，再在该字符串上做 **`|` 切分** 与 **引号块** 解析。

**3）Map：`键类型 = 值类型`（类型行写法）**

- 第三行中 **允许 `=` 两侧有任意空白**（trim 后解析）。  
- **M0 合法组合（定稿）**：`int32 = string`、`string = int64`、`string = string`。  
- 后续扩展（如 `int64 = string`）经 **自定义类型注册** 或 **统一升级打表工具与 Lobby** 实现；不依赖 `json_payload` 内文法版本字段。

**Map 单元格内容（定稿）**

- **多条键值对**：**`键 = 值`** 重复，条目之间用 **`|`** 分隔；`|` 与 `=` 两侧可有空白。  
- **单条内**：以 **「第一个不在引号内的 `=`」** 分割 **键部分** 与 **值部分**（键、值 trim）。  
- **键字面量**：  
  - **`int32` / `int64` 键**：**不加引号**，十进制整数。  
  - **`string` 键**：**必须用 `''` 或 `""` 包裹**（规则同 `string[]` 元素）。  
- **值字面量**：  
  - **`string` 值**：**必须用 `''` 或 `""` 包裹**。  
  - **`int32` / `int64` 值**：**不加引号**。  

**Map 示例（类型 `int32 = string`）**

```text
32='15' | 45 = "hello"
```

表示 `32→"15"`、`45→"hello"`（JSON 对象键一律为字符串，`int32` 键在 Go 中由解析层转换）。

**自定义类型**

- 第三行出现 **非上表内置** 且 **非 `K = V` 合法 map 形式** 的标识符时，视为 **自定义类型**；由打表工具注册表提供 **校验 + JSON 片段生成 +（可选）Go 类型名**；未注册 → **报错**。

**打表工具职责（与 Lobby 边界）**

- 校验：三行头 + 列数 + 每数据行按第三行类型解析；失败则 **不写库**、不发 Redis。  
- 产出：`json_payload`（**§B**）与（Task 7）**Go struct**（`json` tag 与 **第 2 行变量名** 一致）；**Lobby 只消费 `json_payload`**。

**最小完整 CSV 示例**

```text
奖励说明,数量,标签,某映射
note,amount,tags,extra_map
string,int32,string[],int32 = string
首登奖,10,'vip'|'hot',32='15' | 45 = "hello"
```

---

### B. MySQL：`json_payload` 与强类型 Go 的 JSON 形状（与 §A 三行头 CSV 对应）

**根对象（必须字段）**

| JSON 键 | 类型 | 说明 |
|---------|------|------|
| `config_id` | string | 与表字段 `config_id` 一致（便于与行数据自检；亦可仅依赖 DB 列，实现阶段二选一，**须与 Lobby 解析一致**） |
| `revision` | number | 与表字段 `revision` 一致（整数） |
| `data` | **array** | **对象数组**：CSV 第 4 行起每一行对应 `data` 中 **一个** JSON object，键为第 2 行变量名 |

**刻意不包含**：`config_format_version`（或等价「文法版本」字段）。**文法**由 **本仓库约定的解析实现 + 发版** 保证。

**示例（`json_payload`）**

```json
{
  "config_id": "rewards_pack_a",
  "revision": 2,
  "data": [
    {
      "note": "首登奖",
      "amount": 10,
      "tags": ["vip", "hot"],
      "extra_map": {"32": "15", "45": "hello"}
    }
  ]
}
```

**说明**：`int32 = string` 等 map 在 JSON 中自然为 **object**（键均为 string）；Go 侧可用 `map[int32]string` 等时在 **反序列化后** 做键转换，或生成物直接使用 `map[string]string` 再在业务层转换（实现阶段二选一再钉死一种并写测试）。

**Go 侧（M0 占位，可手写后与 Task 7 生成物对齐）**

- 定义 `LobbyConfigEnvelope`（`ConfigID`、`Revision`、`Data []LobbySheetRow` 或 `Data json.RawMessage` 二次解析；**无** `ConfigFormatVersion` 字段）。  
- `json.Unmarshal`：**允许未知字段** 忽略；**`data` 必须为 JSON array**。  
- **`revision` / `config_id` 与行不一致**：以 **MySQL 行** 为准；Lobby 打 **Warn** 并修正内存 envelope（实现写进 Task 4）。

---

### C. MySQL 表结构（建议名与列，迁移时可微调但须同步本文档）

**表名（建议）**：`lobby_config_revision`（**一行 = 某 `config_id` 的某一版 revision**）。

**刻意不包含（已定案）**

- **`env` / 环境列**：**不同环境用不同数据库实例**，不在表内区分。  
- **`status` / 草稿发布列**：**不做**「配置状态机」；是否可上线由 **发布流程 + 写库时机** 控制，**Lobby 不负责**在库内切 draft/published。

| 列名 | 类型 | 作用（具体） |
|------|------|----------------|
| `id` | `BIGINT` PK AUTO_INCREMENT | 行代理键。 |
| `config_id` | `VARCHAR(64)` NOT NULL | **稳定逻辑名**；与 **§C.1 命名规范** 一致，便于人读与检索。 |
| `revision` | `BIGINT` NOT NULL | **该 `config_id` 内**单调递增版本号（打表工具保证）。 |
| **`scope`** | `VARCHAR(64)` NOT NULL | **业务域**，用于筛选（如 `checkin`、`reward`、`rank`、`task`）；与 `config_id` 前缀 **应对齐**（见 §C.1），打表工具可做校验。 |
| **`title`** | `VARCHAR(256)` NULL | **列表/搜索用短标题**（人读），可与 CSV 内某展示名一致或独立维护。 |
| **`tags`** | `JSON` NULL | **标签 JSON 数组**（如 `["s3","pvp"]`），供 `JSON_CONTAINS` 或应用层筛选；无则 `NULL`。 |
| **`effective_from`** | `DATETIME(3)` NULL | **生效起始**（含）；`NULL` = 不限制。 |
| **`effective_until`** | `DATETIME(3)` NULL | **生效结束**（建议语义为 **不含** 该时刻，或实现时钉死「含/不含」并写测试）；`NULL` = 不限制。 |
| `csv_text` | `MEDIUMTEXT` NOT NULL | 策划 CSV 原文（审计）。 |
| `json_payload` | `JSON` NOT NULL | 打表生成的 JSON（Lobby 主读）。 |
| `created_at` | `DATETIME(3)` NOT NULL DEFAULT CURRENT_TIMESTAMP(3) | 插入时间。 |

**约束与索引**

- `UNIQUE KEY uk_config_revision (`config_id`, `revision`)`  
- `KEY idx_config_latest (`config_id`, `revision` DESC)` — 取某配置最新版。  
- **`KEY idx_scope_config_rev (`scope`, `config_id`, `revision` DESC)`** — 按业务域 **枚举配置**、再取最新 revision。  
- **`KEY idx_scope_effective (`scope`, `effective_from`, `effective_until`)`** — 按域 + **时间窗** 筛「某时刻应考虑的配置行」（具体谓词由运营/Lobby 查询约定）。  
- （可选）在 `title` 上建 **FULLTEXT** — 仅当确实需要中文分词/全文再引入，M0 可只用 `LIKE` + `title` 非空约束。

**查询约定（Lobby bootstrap / 通知后拉取）**

- **按 id 取最新**：`WHERE config_id = ? ORDER BY revision DESC LIMIT 1`。  
- **按通知精确拉**：`WHERE config_id = ? AND revision = ?`。  
- **按域 + 当前时刻取候选集**（若 Lobby 需要）：`WHERE scope = ? AND (effective_from IS NULL OR effective_from <= NOW()) AND (effective_until IS NULL OR NOW() < effective_until)` 再按业务规则取 `revision` 最大者等——**实现阶段写死一种语义**，避免「最大 revision」与「时间窗」混用产生歧义。

---

### C.1 `config_id` 命名规范（**仅** `scope` + **稳定业务名**；**不含时间**）

**目的**：`config_id` **长期稳定**，不随档期、赛季、活动起止而改名（否则引用它的网关 meta、脚本、Lobby 配置键都得跟着改）。**何时生效** 只由列 **`effective_from` / `effective_until`**（以及 **`revision`** 滚数据版本）表达，**禁止**把日期、赛季、周次等 **编码进 `config_id`**。

**推荐形态**

```text
{scope}_{slug}
```

| 段 | 规则 | 示例 |
|----|------|------|
| `{scope}` | **小写**，与表列 **`scope` 取值一致**（如 `checkin`、`reward`）。 | `checkin` |
| `{slug}` | **小写蛇形** `[a-z][a-z0-9_]*`，同一业务线下 **语义稳定** 的短名（如 `daily`、`vip_line`、`double_card`）。档期变化 → **改 `effective_*` 或增 `revision`**，**不换 `slug`**。 | `daily` |

**完整示例**

| `config_id` | 建议 `scope` | `title` 示例 | 说明 |
|-------------|--------------|--------------|------|
| `checkin_daily` | `checkin` | 每日签到参数 | 生效窗用 `effective_*`；换赛季仍用同一 `config_id` 亦可，靠 revision + 列。 |
| `reward_double_card` | `reward` | 双倍卡活动 | **勿** 写成 `reward_double_202604` 这类带日期 id；2026-04 档期只写在 **`effective_from`/`effective_until`** 与 `title`。 |

**校验（打表工具推荐）**

- **`config_id` 不得匹配日期/赛季类后缀**（可实现为：禁止 `_20\d{2}` 等简单模式，或人工 code review + CI 名单）。  
- `strings.HasPrefix(config_id, scope + "_")` 或 **`config_id` 首段 = `scope`** 的拆分规则与列 **`scope`** 一致。

---

### D. Redis：通知频道与消息体（仅事件，不含正文）

**频道名（M0 固定）**

- `kd48:lobby:config:notify`  
- 多租户或极大量配置时再拆为 `kd48:lobby:config:notify:{config_id}`；M0 单频道足够，消息内带 `config_id`。

**消息体：单行 JSON UTF-8（一行一个完整 JSON 对象）**

| 键 | 类型 | 必填 | 说明 |
|----|------|------|------|
| `kind` | string | 是 | **M0 固定 `lobby_config_published`** |
| `config_id` | string | 是 | 与 MySQL 一致 |
| `revision` | number | 是 | 与刚写入 MySQL 的行一致 |
| `sha256` | string | 否 | `json_payload` 或整行 canonical 的校验（供对账，Lobby 可选用） |

**示例**

```json
{"kind":"lobby_config_published","config_id":"global","revision":3}
```

**顺序（再次强调）**

1. `BEGIN` → `INSERT` MySQL → `COMMIT` 成功。  
2. 再执行 `PUBLISH kd48:lobby:config:notify '<json>'`。  
3. Lobby **禁止**用定时任务轮询 MySQL 比较变更；**允许**订阅重连后 **单次** `ORDER BY revision DESC LIMIT 1` 对账。

---

### E. Lobby `config.yaml`（运行时）

在 `services/lobby/config.yaml`（示例）增加 **仅与配置管线相关** 项，与现有 `ServerConf` / MySQL / Redis 并列，例如：

```yaml
lobby_config:
  config_id: "global"
  redis_notify_channel: "kd48:lobby:config:notify"
```

（字段名实现时可映射到 `pkg/conf` 结构体；若与现有 YAML 风格冲突，以 `services/user/config.yaml` 为命名参考微调，但 **语义** 不变。）

---

### F. Task 与格式的对应关系（补充）

| Task | 须落实的本节条目 |
|------|------------------|
| Task 2 | §C 表 DDL；§B `json_payload` 与索引 |
| Task 4 | §B envelope + `data`；§C 查询；§E `config_id` |
| Task 5 | §D 频道与消息 JSON；重连对账 |
| Task 7 | §A CSV 解析与校验；写 §C；发 §D |

---

## 文件结构（创建 / 修改）

| 路径 | 职责 |
|------|------|
| `api/proto/lobby/v1/lobby.proto` | Lobby 业务契约（首版最小 RPC，如 `Ping`） |
| `gen_proto.sh` | 纳入 `lobby/v1/lobby.proto` |
| `services/lobby/go.mod` | Lobby 独立 module |
| `services/lobby/cmd/lobby/main.go` | 监听、gRPC 注册、Etcd 注册、生命周期 |
| `services/lobby/cmd/lobby/ingress.go` | `GatewayIngress` 分发 |
| `services/lobby/internal/config/…` | 配置快照、bootstrap、Redis 监听与 MySQL 拉取 |
| `services/lobby/migrations/…` | 配置表 DDL |
| `services/lobby/config.yaml`（示例） | 与 user 对齐的最小配置结构 |
| `go.work` | `use ./services/lobby` |
| `gateway/cmd/seed-gateway-meta/main.go`（或等价） | 注册 Lobby 服务类型与示例路由 |

---

### Task 1: `lobby.v1` Proto 与代码生成

**Files:** `api/proto/lobby/v1/lobby.proto`、`gen_proto.sh`、生成物 `*.pb.go`、`*_grpc.pb.go`

- [ ] **Step 1（TDD 前置）**：无需业务测试；完成后 `go build ./...`（`api/proto` 模块）。

- [ ] **Step 2**：新增 `lobby.proto`（`package lobby.v1`，`go_package` 与 user 风格一致），至少：

  - `rpc Ping(PingRequest) returns (PingReply);`  
  - 消息体含可选 `string detail` 等便于 Ingress 联调。

- [ ] **Step 3**：更新 `gen_proto.sh` 增加 `lobby/v1/lobby.proto`，执行 `bash gen_proto.sh`。

- [ ] **Step 4**：`cd api/proto && go build ./...` 通过。

- [ ] **Step 5**：Commit（建议信息：`feat(proto): add lobby.v1 minimal API`）。

---

### Task 2: MySQL 迁移（配置权威表）

**Files:** `services/lobby/migrations/000001_lobby_config.up.sql`、`.down.sql`（编号以仓库惯例为准）

- [ ] **Step 1（TDD）**：无迁移前可写 **集成测试跳过**（`testing.Short()`）或仅文档；迁移落地后补 **repository 单测**（Task 4）用 **sqlmock** 或嵌入式 DB。

- [ ] **Step 2**：按上文 **§C + §C.1** 建表：含 **`scope`、`title`、`tags`、`effective_from`、`effective_until`**；**不含** `env`、`status`；索引含 `idx_scope_config_rev`、`idx_scope_effective`；`UNIQUE(config_id,revision)` 保留。

- [ ] **Step 3**：本地 `migrate up` 验证（命令与 `spec.md` / README 一致）。

- [ ] **Step 4**：Commit。

---

### Task 3: `services/lobby` 骨架与 gRPC 注册

**Files:** `services/lobby/go.mod`、`cmd/lobby/main.go`、LobbyService 实现文件（如 `cmd/lobby/server.go`）

- [ ] **Step 1（TDD）**：`lobby_test.go` 或 `server_test.go`：对 **LobbyService.Ping** 用 **bufconn** / 内存 gRPC 调一次，断言返回码与字段（先写红）。

- [ ] **Step 2**：实现 `Ping`（可返回固定 `pong` + 可选从配置快照读 `revision` 以证明 DI 接通）。

- [ ] **Step 3**：`main` 中加载 `config.yaml`、MySQL、Redis、OTel（照抄 user 的骨架并删减无关部分）、`grpc.NewServer` + `otelgrpc`、监听、`registry.Register`。

- [ ] **Step 4**：`go test ./...`（`services/lobby`）与 `go build` 本 module 通过。

- [ ] **Step 5**：`go.work` 加入 `./services/lobby`；根目录 `go work sync`（若项目使用）。

- [ ] **Step 6**：Commit。

---

### Task 4: 配置加载（Bootstrap + 强类型 JSON）

**Files:** `services/lobby/internal/config/store.go`、`loader.go` 等

- [ ] **Step 1（TDD）**：对 **LoadFromRow(JSON bytes) → struct** 写单测；对 **SELECT 最新 revision** 用 sqlmock。

- [ ] **Step 2**：按 **§B** 实现 `LobbyConfigEnvelope` + **`Data` 为对象数组**（如 `[]LobbySheetRow`，字段与 CSV 第 2 行一致）；`json.Unmarshal` 失败时 **不替换** 旧快照并打错误日志（行为写进测试或注释）。

- [ ] **Step 3**：`Bootstrap(ctx)`：启动时查询当前 `config_id`（可从静态配置或环境读取）对应 **最大 `revision`** 一行，填充 `atomic.Value`。

- [ ] **Step 4**：`go test ./...` 通过。

- [ ] **Step 5**：Commit。

---

### Task 5: Redis 变更通知（非轮询 MySQL）

**Files:** `internal/config/watcher.go` 或与 loader 同包

- [ ] **Step 1（TDD）**：用 **miniredis** 或 fake：模拟 `PUBLISH` 后，watcher 触发 **一次** `LoadByConfigID`（可注入 `Loader` 接口并记录调用次数）；**禁止**在测试中依赖「定时轮询 MySQL」作为主触发。

- [ ] **Step 2**：实现 **Redis `SUBSCRIBE`** 至 **§D** 频道；解析 **§D** 消息 JSON（校验 `kind == lobby_config_published`）；非法消息打日志并丢弃。

- [ ] **Step 3**：收到通知后 **仅从 MySQL 拉取** 对应 `revision`（或拉最大 revision 并比对）；成功后 **原子替换** 快照。

- [ ] **Step 4**：**重连**：订阅断开时退避重连；重连成功后 **可选** 执行一次 **全量 reload**（对账，非高频轮询）。

- [ ] **Step 5**：`go test ./...`；Commit。

---

### Task 6: `GatewayIngress` 与网关元数据种子

**Files:** `services/lobby/cmd/lobby/ingress.go`、`ingress_test.go`、`gateway/cmd/seed-gateway-meta/main.go`

- [ ] **Step 1（TDD）**：`ingress_test.go`：`/lobby.v1.LobbyService/Ping` 路径下 protojson 往返（先红后绿）。

- [ ] **Step 2**：`RegisterGatewayIngressServer`，`switch route` 分发至 `LobbyService.Ping`。

- [ ] **Step 3**：`main.go` 注册 Ingress + LobbyService。

- [ ] **Step 4**：更新 **seed-gateway-meta**：新增 `kd48/lobby-service` 类型（`UseGatewayIngress: true`）、一条 WS 路由指向 `Ping` 的 `IngressRoute`。

- [ ] **Step 5**：`go test ./...`（至少 `services/lobby`、`gateway` 受影响部分）；`go build` gateway + lobby。

- [ ] **Step 6**：Commit。

---

### Task 7（可选 / 后续）：打表工具与 Go 结构代码生成

- [ ] 独立 CLI 或子模块：解析 **§A** 三行头 CSV → 生成 **§B** `json_payload` → 按 **§C** 写 MySQL → 按 **§D** `PUBLISH`；顺序 **先 MySQL 再 Redis**（与设计一致）。  
- [ ] **从三行头生成 Go**（`make gen-config`）：第 2 行 → 字段名与 **`json` tag**；第 3 行 → Go 类型（含 `[]T`、`map[...]...` 等与 JSON 形状映射）；CI 校验生成物已提交。  
- [ ] 本 Task **可与 Lobby 运行时开发并行**，不阻塞 Task 1～6 的 M0 打通。

---

## 验证命令（默认）

在仓库根（`go.work`）：

```bash
go test ./...
```

受影响模块单独：

```bash
cd services/lobby && go test ./...
cd gateway && go test ./...
cd api/proto && go build ./...
```

---

## 与设计的可追溯映射

| 设计规格 § | 本计划 Task |
|------------|-------------|
| §3 拓扑、仅经 Gateway | Task 6 |
| §4.2 MySQL 权威 | Task 2、4 |
| §4.3 Redis 仅通知 | Task 5 |
| §4.4 bootstrap + 事件拉取 | Task 4、5 |
| §4.5 强类型 JSON | Task 4、7 |
| §5 观测与 Etcd | Task 3 |

---

## 备注

- 人类批准本计划后，实现会话须在文首 **批准记录** 中更新状态与范围。  
- 若网关侧 **AtomicRouter** 对多 target 已有完整支持，Task 6 仅需种子数据；若缺项，在 Task 6 中 **显式列出差额文件** 并修补（不静默跳过）。
