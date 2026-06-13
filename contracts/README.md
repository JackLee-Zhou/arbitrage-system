# contracts/ — 跨服务通信契约（SSOT）

> **本目录是项目的"宪法"之一**。所有微服务之间的数据结构定义都在这里。
> Go 和 Python 的代码**自动生成**，禁止手写跨服务结构体。

---

## 定位

`contracts/` 是 Single Source of Truth（唯一真相源）：

- 改字段 → 改一个 `.proto` 文件 → `make gen` → 两端代码同步更新
- 编译期发现字段不一致 → 不会跑到生产才暴雷
- 跨语言（Go / Python / 未来其他）通过同一份契约对话

---

## 工具链

| 工具 | 版本 | 用途 |
|---|---|---|
| **buf** | latest | Protobuf 工具链（lint + breaking 检测 + 代码生成） |
| **protoc** | 由 buf 调用 | 底层编译器 |
| **protoc-gen-go** | latest | Go 代码生成插件 |
| **protoc-gen-python** | latest | Python 代码生成插件 |

### 安装（M1 阶段做一次）

```bash
# macOS
brew install bufbuild/buf/buf

# Linux
BIN="/usr/local/bin" && \
VERSION="1.32.0" && \
curl -sSL "https://github.com/bufbuild/buf/releases/download/v${VERSION}/buf-$(uname -s)-$(uname -m)" \
  -o "${BIN}/buf" && chmod +x "${BIN}/buf"
```

---

## 目录结构

```
contracts/
├── README.md                  ← 本文件
├── buf.yaml                   ← buf 工具链配置
├── buf.gen.yaml               ← 代码生成配置
├── Makefile                   ← make gen / make lint / make breaking
│
├── ai_score.proto             ← AI 评分（llm_scorer → trading_agent）
├── tick.proto                 ← 行情数据（ctp_tick → indicator/trading_agent）
├── indicator.proto            ← 技术指标（trading_agent 内部 + 持久化）
├── order.proto                ← 报单（trading_agent → broker）
├── position.proto             ← 持仓
├── alert.proto                ← 告警（watchdog → 钉钉）
├── healthz.proto              ← 统一健康检查响应格式
├── orchestrator.proto         ← orchestrator gRPC 服务定义
├── intel_event.proto          ← 舆情事件（intel_collector → llm_scorer）
├── rss_source.proto           ← RSS 源配置（intel_collector 动态管理）
│
└── generated/                 ← 自动生成产物（不入 git）
    ├── go/                    ← 各 Go 服务通过 go.mod replace 引用
    └── python/                ← Python 服务通过 pyproject.toml 路径引用
```

---

## .proto 文件命名约定

- 文件名：`<domain>.proto`（snake_case）
- package 名：`arbitrage.v1`（**版本前缀强制带**，便于未来 v2）
- option go_package：`github.com/<org>/arbitrage-system/contracts/generated/go/<domain>`

例子：

```protobuf
// contracts/ai_score.proto
syntax = "proto3";
package arbitrage.v1;

option go_package = "github.com/jacklee/arbitrage-system/contracts/generated/go/aiscore";

message AIScore {
  string  symbol      = 1;   // "MA2509"
  int64   timestamp   = 2;   // unix ms
  int64   expires_at  = 3;
  double  score       = 4;   // [-1, +1]
  double  confidence  = 5;   // [0, 1]
  string  horizon     = 6;
  string  regime      = 7;
  repeated Driver  key_drivers = 8;
  repeated string  model_chain = 9;
  repeated string  sources     = 10;
  string  raw_text    = 11;
}

message Driver {
  string factor    = 1;
  double weight    = 2;
  string direction = 3;  // "+" / "-"
}
```

---

## 字段编号规则（核心）

Protobuf 的字段编号 = 二进制兼容性的关键。

### ✅ 允许的修改

- **新增字段**：分配新编号
- **删除字段**：保留编号，标记 `reserved`
- **重命名字段**：编号不变，仅改名

### ❌ 禁止的修改

- **改字段类型**：会让旧数据解析错误
- **改字段编号**：等同删除旧字段 + 新增字段
- **复用 reserved 编号**：会导致旧版客户端 crash

例子：

```protobuf
// ❌ 禁止：从 string 改成 int64
// 之前：string score = 4;
// 之后：int64 score = 4;  // ← 旧数据全解析失败

// ✅ 推荐：新增字段 + 标记旧字段为 deprecated
message AIScore {
  string score_legacy = 4 [deprecated = true];
  double score = 12;        // 新编号
  reserved 4;               // 退役后改为 reserved
}
```

---

## 契约变更流程（必须遵守）

### Step 1: 改 .proto

```bash
vim contracts/ai_score.proto
```

### Step 2: lint 检查

```bash
cd contracts/ && make lint
```

### Step 3: breaking change 检测

```bash
make breaking
# 如果会破坏向后兼容，会报错
```

### Step 4: 生成代码

```bash
make gen
# 生成 generated/go/ 和 generated/python/
```

### Step 5: 同步 Go 服务

```bash
cd ../services/trading_agent/
go mod tidy && go build ./... && go test ./...
```

### Step 6: 同步 Python 服务

```bash
cd ../services/llm_scorer/
uv sync && pytest tests/unit/
```

### Step 7: 提交

```bash
git add contracts/ai_score.proto services/.../
git commit -m "feat(contracts): AIScore 增加 emergency_reason 字段"
```

---

## 跨服务对接示例

### Go 端使用（trading_agent）

```go
import (
    aiscore "github.com/jacklee/arbitrage-system/contracts/generated/go/aiscore"
    "google.golang.org/protobuf/proto"
)

func (e *Engine) onAIScoreUpdate(data []byte) error {
    var score aiscore.AIScore
    if err := proto.Unmarshal(data, &score); err != nil {
        return fmt.Errorf("解析 AI 评分失败: %w", err)
    }
    e.applyScore(&score)
    return nil
}
```

### Python 端使用（llm_scorer）

```python
from contracts.generated.python.aiscore_pb2 import AIScore, Driver

def write_score(symbol: str, score: float, confidence: float):
    msg = AIScore(
        symbol=symbol,
        timestamp=int(time.time() * 1000),
        expires_at=int(time.time() * 1000) + 15 * 60 * 1000,
        score=score,
        confidence=confidence,
    )
    redis.set(f"ai_score:{symbol}:live", msg.SerializeToString())
```

---

## 已定义的契约文件清单

| 文件 | 生产者 | 消费者 | 用途 |
|---|---|---|---|
| `ai_score.proto` | llm_scorer | trading_agent | AI 评分 |
| `tick.proto` | ctp_tick | trading_agent / db_writer | 行情 Tick |
| `indicator.proto` | trading_agent | db_writer / dashboard | 技术指标值 |
| `order.proto` | trading_agent | db_writer / dashboard | 报单事件 |
| `position.proto` | trading_agent | dashboard / orchestrator | 持仓 |
| `alert.proto` | watchdog / 各服务 | watchdog | 告警事件 |
| `healthz.proto` | 所有服务 | watchdog / orchestrator | 健康检查响应 |
| `orchestrator.proto` | trading_agent (server) | orchestrator (client) | gRPC 服务 |
| `intel_event.proto` | intel_collector | llm_scorer / db_writer | 舆情事件 |
| `rss_source.proto` | intel_collector | dashboard（M8 加管理 UI） | RSS 源配置 |

---

## 版本管理

- 当前版本：v1（package `arbitrage.v1`）
- 重大变更（如完全重构 AIScore）：升 `arbitrage.v2`，新旧并存一段时间
- 小版本变更（加字段）：v1 内部演进，编号递增

---

## 与其他文档的关联

- `ENGINEERING_GUIDE.md`：规则 2 强制使用 contracts
- `docs/ARCHITECTURE.md` → 「关键代码契约」章节：Protobuf + buf 选型
- 各服务 README：列出该服务用到的契约

---

## AI Agent 工作准则

### 改契约前必读

1. 阅读本文件 + `ENGINEERING_GUIDE.md` + `docs/ARCHITECTURE.md` 中「关键代码契约」章节
2. 确认改动是否破坏向后兼容（跑 `make breaking`）
3. 确认两端服务都更新了

### 禁止行为

- ❌ 直接改 `generated/` 目录下的代码（会被下次 `make gen` 覆盖）
- ❌ 在 Go/Python 服务里手写 cross-service struct 绕过契约
- ❌ 复用已 `reserved` 的字段编号
- ❌ 改字段类型而不升 v2

---

## 文档变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.0 | 2026-06-13 | 初版，定义 10 个 .proto 文件 + 工具链 + 变更流程 |
