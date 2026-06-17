# SECURITY_HARDENING.md — 全链路加密与零信任加固实施文档

> 本文档用于指导后续 agent 对 `arbitrage-system` 做安全加固。
>
> 目标：把当前“单机 Docker Compose 内网明文通信”的架构升级为 **全链路加密、服务强身份认证、高危交易指令非对称签名、防重放、最小暴露、可审计** 的安全架构。
>
> 处理本文档中的任一任务前，必须先读：
>
> 1. [`AGENTS.md`](./AGENTS.md)
> 2. [`ENGINEERING_GUIDE.md`](./ENGINEERING_GUIDE.md)
> 3. [`docs/ARCHITECTURE.md`](./docs/ARCHITECTURE.md)
> 4. [`contracts/README.md`](./contracts/README.md)
> 5. 涉及服务自己的 `README.md`

---

## 0. 当前风险结论

当前项目中，多条关键链路仍是明文或直接暴露到宿主机端口：

| 链路 / 组件 | 当前情况 | 风险 |
|---|---|---|
| 浏览器 → dashboard_web | `http://<host>:8000`，nginx `listen 80` | 明文 HTTP，可被监听 / 篡改 |
| 浏览器 → orchestrator REST | 前端默认 `http://localhost:8080` | 明文 HTTP |
| 浏览器 → orchestrator WebSocket | 前端默认 `ws://localhost:8080/ws` | 明文 WebSocket |
| orchestrator | `8080:8080` 暴露到宿主机 | API / WS 可被直接打到 |
| trading_agent gRPC | `9090:9090` 暴露到宿主机 | 核心交易控制面明文且可直连 |
| trading_agent healthz/metrics | `8081:8081` 暴露到宿主机 | 内部状态泄露风险 |
| Redis | `6379:6379` 暴露到宿主机，无 TLS/ACL | 状态可被读取 / 污染 |
| InfluxDB | `8086:8086` 暴露到宿主机 | 指标 / 审计数据泄露风险 |
| llm_scorer | `5000:5000` 暴露到宿主机 | 内部 AI 服务暴露 |

结论：

> 当前通信默认不能视为加密。上线前必须至少完成 P0 安全任务。

---

## 1. 安全目标

### 1.1 总体目标

最终通信模型：

```text
Browser
  ↓ HTTPS / WSS / 用户认证
nginx / dashboard_web
  ↓ internal Docker network，优先 mTLS 或至少仅内网 HTTP
orchestrator
  ↓ gRPC mTLS + Ed25519 指令签名 + timestamp/nonce/request_id 防重放
trading_agent
  ↓ TLS / mTLS / 最小权限
broker adapter / CTP

其他服务：
  ↔ Redis TLS + ACL
  ↔ InfluxDB HTTPS + 最小权限 token
  ↔ healthz/metrics over HTTPS 或 mTLS
```

### 1.2 安全原则

1. **默认不信任任何内部网络**。
2. **生产环境禁止明文暴露核心控制面**。
3. **所有服务间通信必须有加密与身份认证**。
4. **高危交易指令必须有消息级非对称签名**。
5. **trading_agent 拥有最终风控否决权**，不能无条件信任 orchestrator。
6. **Redis 是可被污染的数据平面，不是可信控制平面**。
7. **密钥、证书、token 不得写入镜像或提交到 Git**。
8. **所有认证失败、高危操作、风控拒绝必须审计**。

---

## 2. 推荐密码学方案

### 2.1 链路加密

服务间连接统一使用：

```text
TLS 1.3 only
mTLS required for internal service-to-service calls
```

优先算法建议：

| 用途 | 推荐 |
|---|---|
| 服务证书 | ECDSA P-256 或 Ed25519 |
| 密钥交换 | X25519 / ECDHE |
| 对称加密 | AES-256-GCM 或 ChaCha20-Poly1305 |
| Hash | SHA-256 或更高 |

如果必须使用 RSA：

```text
RSA-3072 或 RSA-4096
签名使用 RSA-PSS + SHA-256
禁止自行实现 RSA 加密业务 payload
```

### 2.2 高危指令签名

高危交易指令使用 **Ed25519 非对称签名**。

推荐理由：

- 私钥只在调用方保存；
- trading_agent 只保存公钥；
- key 短、签名短、性能好；
- 比手写 RSA 更不容易踩 padding 坑；
- 适合多调用方、多 key 轮换。

### 2.3 可选消息级加密

如果后续要求“即使中间代理也不能看到 payload”，可在 mTLS 外叠加消息级加密：

| 方案 | 建议 |
|---|---|
| HPKE | 推荐，适合服务间 payload 加密 |
| JWE | 可选，JSON 生态友好但库质量需评估 |
| libsodium sealed box | 可选，实现简单 |

当前 P0 不强制做 HPKE，先做 mTLS + Ed25519 签名。

---

## 3. 目标网络拓扑

### 3.1 Docker network 分区

推荐把当前单一 `arbitrage-net` 拆成：

```yaml
networks:
  edge:
    driver: bridge

  internal:
    driver: bridge
    internal: true

  trading:
    driver: bridge
    internal: true

  data:
    driver: bridge
    internal: true
```

### 3.2 服务归属

| 服务 | edge | internal | trading | data |
|---|---:|---:|---:|---:|
| dashboard_web / nginx | ✅ |  |  |  |
| orchestrator | ✅ | ✅ | ✅ | ✅ |
| trading_agent |  |  | ✅ | ✅ |
| ctp_tick |  |  | ✅ | ✅ |
| redis |  |  |  | ✅ |
| influxdb |  |  |  | ✅ |
| db_writer |  | ✅ |  | ✅ |
| watchdog |  | ✅ | ✅ | ✅ |
| llm_scorer |  | ✅ |  | ✅ |
| intel_collector |  | ✅ |  | ✅ |
| stock_tick |  | ✅ | ✅ | ✅ |

### 3.3 端口暴露原则

生产环境只允许对外暴露：

```text
80/tcp   仅用于跳转 HTTPS
443/tcp  dashboard HTTPS + WSS
22/tcp   SSH，必须限源 IP + key only，最好改管理入口或走堡垒机
```

以下端口不得对公网暴露：

```text
8080 orchestrator REST/WS
8081 trading_agent healthz/metrics
9090 trading_agent gRPC
6379 Redis
8086 InfluxDB
5000 llm_scorer
其他内部服务端口
```

Compose 中应从 `ports:` 改为 `expose:` 或仅内部监听。

---

## 4. Web 与服务器通信加密

### 4.1 目标

浏览器访问统一改为：

```text
https://<domain>/
https://<domain>/api/v1/...
wss://<domain>/ws
```

禁止生产环境使用：

```text
http://<host>:8000
http://<host>:8080
ws://<host>:8080/ws
```

### 4.2 dashboard_web nginx 要求

`services/dashboard_web/nginx.conf` 需要支持：

1. `listen 443 ssl http2`；
2. `listen 80` 自动跳转 HTTPS；
3. 静态 SPA 路由；
4. `/api/v1/` 反向代理到 `orchestrator:8080`；
5. `/ws` 反向代理 WebSocket 到 `orchestrator:8080/ws`；
6. 安全响应头。

参考结构：

```nginx
server {
    listen 80;
    server_name example.com;
    return 301 https://$host$request_uri;
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/nginx/certs/fullchain.pem;
    ssl_certificate_key /etc/nginx/certs/privkey.pem;
    ssl_protocols TLSv1.3 TLSv1.2;

    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-Frame-Options "DENY" always;
    add_header Referrer-Policy "no-referrer" always;

    root /usr/share/nginx/html;
    index index.html;

    location /static/ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    location /api/v1/ {
        proxy_pass http://orchestrator:8080/api/v1/;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Request-ID $request_id;
    }

    location /ws {
        proxy_pass http://orchestrator:8080/ws;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-Proto https;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Request-ID $request_id;
    }

    location / {
        try_files $uri $uri/ /index.html;
    }
}
```

### 4.3 前端 API 配置要求

`services/dashboard_web/src/theme/tokens.js` 目前默认是：

```js
BASE_URL: process.env.REACT_APP_API_URL || 'http://localhost:8080',
WS_URL: process.env.REACT_APP_WS_URL || 'ws://localhost:8080/ws',
```

生产应改为同源默认：

```js
const wsProtocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';

export const API = {
  BASE_URL: process.env.REACT_APP_API_URL || '/api/v1',
  WS_URL: process.env.REACT_APP_WS_URL || `${wsProtocol}//${window.location.host}/ws`,
  REQUEST_TIMEOUT: 10000,
};
```

注意：

- 生产环境浏览器不直连 `:8080`；
- 所有 REST 走 `/api/v1`；
- WebSocket 走同源 `/ws`，HTTPS 下自动为 `wss://`。

---

## 5. 服务间 mTLS

### 5.1 服务身份

每个服务必须有独立证书身份：

```text
orchestrator
trading_agent
ctp_tick
db_writer
watchdog
llm_scorer
intel_collector
redis
influxdb
dashboard_web
```

证书 subject / SAN 必须能标识服务，例如：

```text
DNS:orchestrator
DNS:trading_agent
DNS:redis
URI:spiffe://arbitrage-system/prod/orchestrator
```

短期可以用私有 CA 签发，长期可考虑 SPIFFE/SPIRE。

### 5.2 orchestrator ↔ trading_agent

这是最高优先级链路。

目标：

```text
orchestrator --gRPC mTLS--> trading_agent
```

要求：

1. trading_agent gRPC server 启用 TLS；
2. trading_agent 要求 client cert；
3. trading_agent 校验 client cert 由可信 CA 签发；
4. trading_agent 校验 caller identity 是否允许调用对应 RPC；
5. orchestrator gRPC client 使用自己的 client cert；
6. 生产禁用明文 gRPC；
7. 测试覆盖无证书、错误证书、过期证书、合法证书。

推荐配置项：

```env
GRPC_TLS_ENABLED=true
GRPC_TLS_MIN_VERSION=1.3
GRPC_CA_FILE=/run/secrets/ca.crt
GRPC_CERT_FILE=/run/secrets/trading_agent.crt
GRPC_KEY_FILE=/run/secrets/trading_agent.key
GRPC_REQUIRE_CLIENT_CERT=true
```

orchestrator：

```env
TRADING_AGENT_GRPC=trading_agent:9090
TRADING_AGENT_GRPC_TLS=true
GRPC_TLS_MIN_VERSION=1.3
GRPC_CA_FILE=/run/secrets/ca.crt
GRPC_CERT_FILE=/run/secrets/orchestrator.crt
GRPC_KEY_FILE=/run/secrets/orchestrator.key
```

### 5.3 healthz / metrics

内部健康检查和指标不应裸露详细信息。

目标：

```text
watchdog --mTLS/HTTPS--> service /healthz
watchdog --mTLS/HTTPS--> service /metrics
```

要求：

- 未认证请求最多返回极简状态；
- 详细依赖状态只允许 watchdog client cert 访问；
- `/metrics` 不暴露公网；
- 认证失败要记录日志和告警计数。

---

## 6. 高危交易指令 Ed25519 签名

### 6.1 适用范围

以下 RPC / API 必须加消息级签名：

```text
EmergencyClose
PauseStrategy
ResumeStrategy
UpdateRiskParams
CreateStrategy
UpdateStrategy
DeleteStrategy
PromoteStrategy
PlaceOrder
CancelOrder
任何未来会影响交易行为或风控边界的 mutation
```

### 6.2 签名 envelope

建议在 proto 中引入统一 envelope，或者在现有 request 中加入 metadata 字段。

推荐字段：

```proto
message SignedCommandMeta {
  string request_id = 1;
  int64 timestamp_ms = 2;
  string nonce = 3;
  string operator = 4;
  string caller = 5;
  string key_id = 6;
  string payload_sha256 = 7;
  string signature = 8;
}
```

业务请求示意：

```proto
message EmergencyCloseRequest {
  SignedCommandMeta meta = 1;
  string reason = 2;
  string symbol = 3;
  string request_by = 4;
}
```

### 6.3 canonical signing string

禁止直接对非确定性 JSON 文本随意签名。

推荐签名原文：

```text
v1
rpc=<RPC_NAME>
request_id=<UUID>
timestamp_ms=<TIMESTAMP_MS>
nonce=<NONCE>
caller=<CALLER>
operator=<OPERATOR>
payload_sha256=<SHA256_OF_CANONICAL_PAYLOAD>
```

签名：

```text
signature = base64(Ed25519Sign(private_key, signing_string))
```

验签：

```text
Ed25519Verify(public_key_for_key_id, signing_string, signature)
```

### 6.4 trading_agent 验证流程

trading_agent 收到高危命令后必须按顺序验证：

1. mTLS client identity 是否可信；
2. `key_id` 是否存在且未吊销；
3. `caller` 与 mTLS identity 是否匹配；
4. caller 是否有权调用该 RPC；
5. `timestamp_ms` 是否在允许窗口内，建议 ±30 秒；
6. `nonce` 是否未使用过；
7. `request_id` 是否未处理过；
8. `payload_sha256` 是否匹配实际 payload；
9. Ed25519 signature 是否正确；
10. trading_agent 硬风控是否允许执行；
11. 写 audit log；
12. 执行或拒绝。

任何一步失败均拒绝执行，并写审计。

### 6.5 防重放存储

nonce / request_id 可存：

- trading_agent 内存 LRU + TTL；
- Redis 独立 namespace，需 TLS + ACL；
- 本地轻量持久化，用于服务重启后短期防重放。

最低要求：

```text
nonce TTL >= 5 分钟
request_id TTL >= 24 小时或交易日内有效
```

---

## 7. Redis TLS + ACL

### 7.1 目标

Redis 不能裸奔。目标：

```text
Redis TLS
requirepass / ACL
每个服务独立 user
按 key pattern 最小授权
不暴露公网端口
```

### 7.2 ACL 权限建议

| 服务 | 允许读 | 允许写 | 禁止 |
|---|---|---|---|
| orchestrator | `services:*`, `position:*`, `ai_score:*`, `sentiment:*` | `write:audit` | 直接写交易状态 |
| trading_agent | `ai_score:*`, 必要状态 | `position:*`, `trading_state:*`, `order_event:*` | 写 `ai_score:*` |
| ctp_tick | 必要行情 key/stream | tick stream | 风控 / 持仓 / 策略 key |
| llm_scorer | `intel_event:*`, tick/feature 输入 | `ai_score:*` | 持仓 / 订单 / 策略 key |
| intel_collector | 必要源状态 | `intel_event:*` | 交易状态 |
| db_writer | `write:*`, tick/order streams | 一般不写业务状态 | 修改控制面 |
| watchdog | `services:*`, health/metrics | 告警 stream，可选 | 交易状态 |

### 7.3 Compose 要求

生产环境：

- 移除 `6379:6379`；
- Redis 只接入 `data` internal network；
- 开启 TLS 配置；
- ACL 文件通过 secret / readonly mount 注入；
- Redis password / ACL 不提交 Git。

---

## 8. InfluxDB HTTPS + token 最小权限

目标：

```text
InfluxDB HTTPS
不暴露公网
每个写入方使用最小权限 token
```

要求：

1. 移除生产环境 `8086:8086` 公网暴露；
2. InfluxDB 只在 `data` internal network；
3. db_writer 使用 HTTPS URL；
4. token 通过 secrets 注入；
5. 不同 bucket / org 的 token 最小授权；
6. 审计日志 bucket 不与行情高频 bucket 共用高权限 token。

---

## 9. trading_agent 硬风控

加密和签名不能替代风控。trading_agent 必须把 orchestrator 当成“可能被攻破的请求方”。

### 9.1 必须内置的硬限制

至少包括：

- 单笔最大手数；
- 单合约最大持仓；
- 单日最大成交手数；
- 单日最大亏损；
- 最大下单频率；
- 最大撤单频率；
- 禁止未知合约；
- 非交易时段拒绝普通交易操作；
- 交易时段禁止放宽风控参数；
- AI score 异常 / 过期时降级；
- CTP 回报与内部状态不一致时进入 safe mode。

### 9.2 最终否决权

原则：

```text
orchestrator 可以请求交易动作；trading_agent 必须独立判断是否允许执行。
```

即使：

- mTLS 合法；
- Ed25519 签名合法；
- operator 有权限；

只要触发硬风控，trading_agent 必须拒绝。

---

## 10. 密钥、证书与 secrets 管理

### 10.1 必须保存的私钥分类

本系统至少会有以下几类私钥。它们的用途、存放位置和泄露后果不同，必须分开管理。

| 私钥类型 | 示例 | 用途 | 推荐保存位置 | 是否放云服务器 | 泄露严重性 |
|---|---|---|---|---:|---|
| Root CA 私钥 | `root-ca.key` | 签发所有服务 mTLS 证书 | 用户本地离线 / 加密介质 / 密码管理器附件 | ❌ 绝不放 | 极高 |
| 服务 mTLS 私钥 | `orchestrator.key`, `trading_agent.key`, `redis.key` | 证明服务身份，建立 mTLS | 对应服务的 Docker secrets / 只读挂载 | ✅ 仅对应服务需要 | 高 |
| Ed25519 指令签名私钥 | `orchestrator-command-2026-06.key` | 给高危交易命令签名 | orchestrator secrets + 用户本地离线备份 | ✅ orchestrator 需要 | 极高 |
| SSH 私钥 | `ecs-admin-ed25519` | 登录云服务器 | 用户电脑 / 密码管理器 | ❌ 不放 | 极高 |
| dashboard HTTPS 私钥 | `privkey.pem` | 浏览器 HTTPS/WSS 入口 | nginx secrets / 云证书托管 | ✅ 通常需要 | 高 |
| Redis / Influx token | `redis.acl`, `influx-token` | 数据服务认证 | secrets / KMS | ✅ 运行服务需要 | 中高 |

关键原则：

```text
Root CA 私钥留在用户手里离线保存；
服务私钥只放对应服务；
交易指令签名私钥单独管理并定期轮换；
SSH 私钥绝不上传服务器。
```

### 10.2 Root CA 私钥管理

Root CA 私钥是整个 mTLS 身份体系的根。它用于签发：

```text
orchestrator.crt
trading_agent.crt
redis.crt
influxdb.crt
watchdog.crt
其他服务证书
```

Root CA 私钥泄露后，攻击者可以伪造任意服务证书，整个 mTLS 体系失效。因此：

- `root-ca.key` 不得放到云服务器；
- `root-ca.key` 不得放入 Docker 镜像；
- `root-ca.key` 不得提交 Git；
- 服务证书应在用户本地或受控 CI/KMS 中签发；
- 服务器只接收运行所需的 `service.crt`、`service.key` 和 `ca.crt`；
- `ca.crt` 可以分发，`root-ca.key` 不能分发。

推荐用户本地保存方式：

```text
加密磁盘镜像 / 加密 U 盘 / 1Password 或 Bitwarden 安全附件 / 离线备份
```

### 10.3 服务 mTLS 私钥管理

每个服务必须有独立私钥，禁止多个服务共用同一把 key。

示例：

```text
orchestrator.key
trading_agent.key
redis.key
influxdb.key
watchdog.key
```

要求：

- 只部署到对应服务；
- 使用 Docker secrets、云 Secrets Manager 或只读 bind mount；
- 文件权限 `0400`；
- 不 COPY 进镜像；
- 不提交 Git；
- 不在日志中打印；
- 定期轮换。

泄露处理：

| 泄露对象 | 处理 |
|---|---|
| 单个服务私钥 | 立即停止该服务，重新签发该服务证书，移除旧证书信任，检查 audit log |
| 多个服务私钥 | 进入事故响应，轮换所有受影响证书和相关 token |
| trading_agent 私钥 | 立即停止交易，轮换 trading_agent 证书，检查所有交易指令审计 |
| orchestrator 私钥 | 轮换 orchestrator 证书，并同时检查 Ed25519 指令签名私钥是否泄露 |

### 10.4 Ed25519 指令签名私钥管理

指令签名私钥和 mTLS 私钥不是一回事，禁止复用。

```text
mTLS 身份私钥：orchestrator.key
命令签名私钥：orchestrator-command-2026-06.key
```

用途区别：

- mTLS 私钥证明“连接来自 orchestrator”；
- Ed25519 私钥证明“这条高危交易指令由授权方签发”。

要求：

- orchestrator 持有 Ed25519 私钥；
- trading_agent 只保存 Ed25519 公钥；
- 请求中必须带 `key_id`；
- trading_agent 按 `key_id` 查找对应公钥和权限；
- 私钥需要用户本地离线备份；
- 30-90 天轮换一次；
- 泄露时立即吊销对应 `key_id`。

推荐 trusted callers 配置：

```yaml
trusted_callers:
  - key_id: orchestrator-command-2026-06
    caller: orchestrator
    public_key: "base64-ed25519-public-key"
    allowed_rpcs:
      - EmergencyClose
      - PauseStrategy
      - ResumeStrategy
      - UpdateRiskParams
```

平滑轮换流程：

1. 生成 `orchestrator-command-2026-07.key/.pub`；
2. trading_agent 新增信任 `key_id=orchestrator-command-2026-07`；
3. orchestrator 切换到新私钥签名；
4. 观察无旧 key 流量；
5. trading_agent 删除 `orchestrator-command-2026-06` 公钥；
6. 安全归档或销毁旧私钥。

### 10.5 SSH 私钥管理

SSH 私钥是服务器管理入口，不能放服务器上。

要求：

- SSH 私钥只保存在用户电脑 / 密码管理器；
- SSH 私钥必须有 passphrase；
- 云服务器禁用密码登录；
- 安全组限制 SSH 来源 IP；
- 推荐使用堡垒机、VPN 或临时安全组授权；
- 不要把 SSH 私钥上传到任何容器或服务器目录。

### 10.6 dashboard HTTPS 私钥管理

如果使用 Let’s Encrypt，nginx 通常需要：

```text
fullchain.pem
privkey.pem
```

其中 `privkey.pem` 是 HTTPS 私钥。

要求：

- 通过 certbot / acme.sh / 云证书托管自动续期；
- 只允许 nginx 读取；
- 不提交 Git；
- 不 COPY 进镜像；
- 生产建议由云负载均衡 / 证书管理服务托管，减少服务器本机私钥管理压力。

### 10.7 推荐目录

用户本地离线安全目录示例：

```text
~/secure/arbitrage-system/
  ca/
    root-ca.crt
    root-ca.key
  mtls/
    orchestrator.crt
    orchestrator.key
    trading_agent.crt
    trading_agent.key
    redis.crt
    redis.key
    influxdb.crt
    influxdb.key
  signing/
    orchestrator-command-2026-06.key
    orchestrator-command-2026-06.pub
  ssh/
    ecs-admin-ed25519
```

服务器运行 secrets 目录示例，仅保存运行必须项：

```text
secrets/
  ca/
    ca.crt
  certs/
    orchestrator.crt
    trading_agent.crt
  keys/
    orchestrator.key
    trading_agent.key
  signing/
    orchestrator-command-2026-06.key
    trading-agent-trusted-callers.yaml
```

服务器上不应保存：

```text
root-ca.key
SSH 私钥
其他不属于该服务的私钥
```

`secrets/` 必须加入 `.gitignore`。

### 10.8 文件权限

```bash
chmod 700 ~/secure/arbitrage-system
chmod 0400 *.key
chmod 0444 *.crt
```

容器内服务尽量非 root 运行。

### 10.9 禁止事项

禁止：

- 把私钥 COPY 进 Docker 镜像；
- 把证书私钥提交 Git；
- 把 Redis password / Influx token 写入 README 示例之外的真实配置；
- 多个服务共用同一私钥；
- 复用 mTLS 私钥作为交易指令签名私钥；
- 把 Root CA 私钥放到云服务器；
- 把 SSH 私钥放到云服务器；
- 用长期不轮换的生产证书。

### 10.10 轮换策略

最低要求：

| 类型 | 建议有效期 |
|---|---:|
| Root CA | 离线保存，1-3 年 |
| Service cert | 7-30 天 |
| Ed25519 signing key | 30-90 天 |
| Redis / Influx token | 30-90 天或按事件轮换 |
| SSH key | 视风险轮换，人员/设备变化时必须轮换 |

### 10.11 服务器被攻破后的处理

如果单个容器被攻破：

1. 停止该容器；
2. 假设该服务私钥和 token 已泄露；
3. 重新签发该服务证书；
4. 轮换该服务 Redis / Influx token；
5. 检查该服务 audit log；
6. 排查是否横向移动。

如果宿主机 root 被攻破：

1. 立即停止交易；
2. 假设服务器上所有服务私钥、token、Ed25519 signing key 已泄露；
3. 吊销 / 移除所有服务证书信任；
4. 重新部署干净服务器；
5. 重新签发所有服务证书；
6. 轮换 Ed25519 指令签名 key；
7. 轮换 Redis ACL、Influx token、外部 API key；
8. 检查 CTP / 交易账户；
9. 对照 audit log 和交易所订单记录做事故复盘。

如果 Root CA 私钥泄露：

1. 立即废弃旧 CA；
2. 生成新 Root CA；
3. 重新签发所有服务证书；
4. 全服务替换 `ca.crt`；
5. 轮换所有服务私钥；
6. 复盘 Root CA 私钥存储方式。

---

## 11. 审计与告警

### 11.1 必审事件

必须审计：

- 所有高危 RPC；
- 所有签名验证失败；
- 所有 mTLS client cert 验证失败；
- 所有防重放拒绝；
- 所有风控拒绝；
- Redis ACL 拒绝；
- InfluxDB 写入 token 拒绝；
- healthz/metrics 未授权访问；
- 服务证书即将过期；
- 证书 / key 轮换事件。

### 11.2 audit 字段

建议字段：

```text
ts
request_id
rpc / endpoint
caller
operator
source_ip
mtls_subject
client_cert_fingerprint
key_id
payload_sha256
result
reject_reason
risk_check_result
service_version
```

### 11.3 告警

以下事件应推送到手机 / 邮件 / Telegram / 企业微信：

- trading_agent 收到未知 client cert；
- 高危 RPC 签名失败；
- nonce 重放；
- 短时间大量下单 / 撤单；
- 风控参数被修改；
- 证书即将过期；
- Redis / Influx 出现未授权访问；
- trading_agent 进入 safe mode。

---

## 12. 分阶段任务清单

### 🔴 SEC-001 全链路加密策略与配置基线 [P0]

**目标**：明确并落实所有链路的加密方式和生产禁止项。

**范围**：

- 更新 `docs/ARCHITECTURE.md` 安全拓扑；
- 更新各服务 README；
- 增加生产安全配置说明；
- 明确禁止生产明文端口。

**验收**：

- 文档列出所有链路与加密方式；
- Docker Compose 生产配置不暴露内部端口；
- `.env.example` 增加 TLS/security 相关配置项；
- 明文模式只能在本地开发显式开启。

---

### 🔴 SEC-002 Web HTTPS + WSS 入口加密 [P0]

**目标**：浏览器到服务器统一 HTTPS/WSS。

**涉及文件**：

- `services/dashboard_web/nginx.conf`
- `services/dashboard_web/src/theme/tokens.js`
- `docker-compose.yml` 或新增 `docker-compose.prod.yml`
- `services/dashboard_web/README.md`

**要求**：

- nginx 支持 443 TLS；
- 80 自动跳转 443；
- `/api/v1/` 反代 orchestrator；
- `/ws` 支持 WebSocket upgrade；
- 前端默认同源 `/api/v1` 和 `/ws`；
- 生产不再暴露 orchestrator `8080` 给公网。

**验收**：

```text
curl -I http://domain 返回 301 到 https
curl -I https://domain 返回 200
浏览器 REST 请求为 https://domain/api/v1/...
浏览器 WS 连接为 wss://domain/ws
公网无法访问 http://domain:8080
```

---

### 🔴 SEC-003 Docker 网络隔离与端口收敛 [P0]

**目标**：拆分 edge/internal/trading/data 网络，内部服务不暴露公网。

**涉及文件**：

- `docker-compose.yml`
- 可选：`docker-compose.prod.yml`
- 各服务 README

**要求**：

- 移除生产环境 Redis `6379:6379`；
- 移除 InfluxDB `8086:8086`；
- 移除 trading_agent `9090:9090` / `8081:8081`；
- 移除 llm_scorer `5000:5000`；
- orchestrator 只 `expose: 8080`，由 dashboard nginx 反代；
- 只对外暴露 80/443。

**验收**：

```text
docker compose config 中内部服务无公网 ports
从宿主机无法直接连 redis:6379 / trading_agent:9090
nginx 容器能访问 orchestrator:8080
orchestrator 容器能访问 trading_agent:9090
```

---

### 🔴 SEC-004 orchestrator ↔ trading_agent gRPC mTLS [P0]

**目标**：核心交易控制链路加 mTLS。

**涉及文件**：

- `services/trading_agent`
- `services/orchestrator`
- `contracts/` 如需扩展配置文档
- `docker-compose.yml` / prod compose
- `scripts/security/`

**要求**：

- trading_agent gRPC server 支持 TLS；
- trading_agent 要求 client cert；
- orchestrator gRPC client 支持 client cert；
- 生产默认 `TLS enabled`；
- 本地开发可显式 `ALLOW_INSECURE_GRPC=true`，生产禁止。

**验收**：

- 无证书调用 trading_agent gRPC 失败；
- 错误 CA 证书失败；
- 合法 orchestrator 证书成功；
- 明文 gRPC 在生产配置下失败；
- 单元测试 / 集成测试覆盖上述场景。

---

### 🔴 SEC-005 高危交易 RPC Ed25519 签名与防重放 [P0]

**目标**：即使 mTLS 通过，高危命令仍需消息级签名和防重放。

**涉及文件**：

- `contracts/orchestrator.proto` 或相关 trading proto
- `services/orchestrator`
- `services/trading_agent`
- `services/orchestrator/README.md`
- `services/trading_agent/README.md`

**要求**：

- 定义 `SignedCommandMeta`；
- 所有 mutation RPC 携带 meta；
- orchestrator 使用 Ed25519 私钥签名；
- trading_agent 使用公钥验签；
- timestamp ±30 秒；
- nonce 防重放；
- request_id 幂等；
- 拒绝原因写 audit。

**验收**：

- 缺签名拒绝；
- 错签名拒绝；
- 过期 timestamp 拒绝；
- 重复 nonce 拒绝；
- 重复 request_id 不重复执行；
- 合法签名 + 合法权限 + 通过风控才执行。

---

### 🔴 SEC-006 trading_agent 硬风控最终否决权 [P0]

**目标**：trading_agent 不无条件信任 orchestrator。

**涉及文件**：

- `services/trading_agent/internal/domain` 或现有风控模块
- `services/trading_agent/README.md`

**要求**：

- 单笔 / 单日 / 单合约硬限制；
- 非交易时段规则；
- 风控参数变更规则；
- CTP 状态不一致进入 safe mode；
- 所有拒绝写 audit。

**验收**：

- 超限命令即使签名合法也被拒；
- safe mode 下拒绝开仓类命令；
- 风控拒绝有明确 reason；
- 单元测试覆盖主要边界。

---

### 🟡 SEC-007 Redis TLS + ACL [P1]

**目标**：Redis 加密、认证、最小权限。

**涉及文件**：

- `docker-compose.yml` / prod compose
- 所有连接 Redis 的服务配置
- `pkg/redis` 或各语言 Redis client 配置
- Redis ACL 文件 / secrets

**要求**：

- Redis 启用 TLS；
- 每服务独立 Redis user；
- key pattern 最小授权；
- 禁止匿名访问；
- 不暴露公网。

**验收**：

- 无密码连接失败；
- 错误用户访问未授权 key 失败；
- TLS 连接成功；
- 明文连接在生产失败。

---

### 🟡 SEC-008 InfluxDB HTTPS + 最小权限 token [P1]

**目标**：InfluxDB 数据写入加密且最小权限。

**要求**：

- InfluxDB HTTPS；
- 不暴露公网；
- token secrets 注入；
- db_writer token 只有必要 bucket 写权限。

**验收**：

- HTTP 明文生产配置不可用；
- 错 token 写入失败；
- 正确 token 可写指定 bucket；
- 无法写非授权 bucket。

---

### 🟡 SEC-009 healthz / metrics mTLS [P1]

**目标**：内部健康检查和指标加密鉴权。

**要求**：

- watchdog 使用 client cert；
- 未授权访问只返回极简状态或 401/403；
- metrics 不暴露公网；
- 认证失败计数进入告警。

**验收**：

- 无 cert 访问详细 healthz 失败；
- watchdog cert 成功；
- metrics 仅内部可见。

---

### 🟡 SEC-010 证书生成、轮换、吊销脚本 [P1]

**目标**：把证书生命周期工程化。

**建议文件**：

```text
scripts/security/gen-root-ca.sh
scripts/security/gen-service-cert.sh
scripts/security/gen-ed25519-key.sh
scripts/security/rotate-service-cert.sh
scripts/security/verify-certs.sh
```

**要求**：

- 支持生成本地开发 CA；
- 支持每服务独立证书；
- 支持 SAN；
- 输出权限正确；
- 不把 secrets 加入 Git；
- README 说明生产应使用 KMS / Secrets Manager。

**验收**：

- 一条命令生成开发证书；
- docker compose 可挂载使用；
- 证书过期检查脚本可运行。

---

### 🟢 SEC-011 SPIFFE/SPIRE 或 Service Mesh 评估 [P2]

**目标**：为未来多机 / Kubernetes 迁移准备自动身份系统。

**候选**：

- SPIFFE/SPIRE；
- Linkerd；
- Istio STRICT mTLS；
- Consul Connect。

**当前建议**：

单机 Docker Compose 阶段不强制引入，先用私有 CA + mTLS + scripts 实现。

---

### 🟢 SEC-012 HPKE/JWE 消息级加密评估 [P2]

**目标**：对极敏感 payload 做端到端消息加密。

**候选**：

- HPKE：X25519 + HKDF-SHA256 + AES-256-GCM；
- JWE：ECDH-ES + A256GCM；
- libsodium sealed box。

**当前建议**：

P0 阶段先完成 mTLS + Ed25519 签名。HPKE/JWE 作为未来增强项。

---

## 13. 实施顺序建议

推荐顺序：

```text
1. SEC-002 Web HTTPS + WSS
2. SEC-003 Docker 网络隔离与端口收敛
3. SEC-004 orchestrator ↔ trading_agent gRPC mTLS
4. SEC-005 高危交易 RPC Ed25519 签名与防重放
5. SEC-006 trading_agent 硬风控最终否决权
6. SEC-007 Redis TLS + ACL
7. SEC-008 InfluxDB HTTPS + token
8. SEC-009 healthz / metrics mTLS
9. SEC-010 证书轮换脚本
```

最低上线门槛：

```text
SEC-002 + SEC-003 + SEC-004 + SEC-005 + SEC-006
```

在最低上线门槛前，不建议接入实盘交易。

---

## 14. Agent 实现注意事项

1. 改跨服务协议前必须修改 `contracts/*.proto` 并运行生成流程。
2. 新增 REST endpoint 必须同步 `services/orchestrator/README.md` 和 `API_REQUESTS.md`。
3. 安全相关配置必须写入 `.env.example`，但不能写真实 secret。
4. Docker 生产配置建议单独放 `docker-compose.prod.yml`，避免破坏本地开发体验。
5. 明文 fallback 只能用于本地开发，变量名必须显式危险，例如：

```env
ALLOW_INSECURE_GRPC_FOR_LOCAL_DEV=true
ALLOW_INSECURE_REDIS_FOR_LOCAL_DEV=true
```

6. 所有高危拒绝路径都要测试。
7. 不要把交易决策逻辑写进 orchestrator。
8. 不要把 Redis 当成可信交易输入。
9. 不要跳过审计。
10. 每完成一个 SEC 任务，必须在本文档底部追加变更记录。

---

## 15. 变更记录

| 版本 | 日期 | 变更 |
|---|---|---|
| v1.1 | 2026-06-17 | 补充私钥保存与分层管理：Root CA、服务 mTLS 私钥、Ed25519 指令签名私钥、SSH 私钥、HTTPS 私钥；新增泄露后的事故处理流程 |
| v1.0 | 2026-06-17 | 初版：整理全链路加密、Web HTTPS/WSS、mTLS、Ed25519 签名、防重放、Docker 网络隔离、Redis/Influx 加固、硬风控与实施任务清单 |
