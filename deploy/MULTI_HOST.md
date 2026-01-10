# 多机部署（两台服务器起步）

本文档提供一套“拆分部署”的落地方式：把 `PostgreSQL/Redis` 放在一台“状态机”，把 `FluxCode + 入口(Caddy)` 放在另一台“应用/入口机”。后续如果要加更多应用节点，也给出扩展方法。

## 目录与文件

- `deploy/docker-compose.infra.yml`：仅部署 PostgreSQL + Redis（状态机用）
- `deploy/docker-compose.app.yml`：部署 Caddy + FluxCode（入口机用）
- `deploy/docker-compose.node.yml`：仅部署 FluxCode，并把 8080 暴露给入口层（新增应用节点用）
- `deploy/Caddyfile.multihost`：入口反代与粘性会话配置
- `deploy/.env.infra.example`：状态机环境变量模板
- `deploy/.env.app.example`：入口机环境变量模板
- `deploy/.env.node.example`：应用节点环境变量模板

## 推荐架构（两台服务器）

- **服务器 A（状态机）**：PostgreSQL + Redis  
  - 对外（内网）开放：`5432`、`6379`（只允许服务器 B 访问）
- **服务器 B（入口机）**：Caddy + FluxCode  
  - 对外开放：`HTTP_PORT/HTTPS_PORT`（默认 `80/443`，建议用域名访问）

> 说明：你也可以在服务器 A 上再跑一个 FluxCode 作为第二个应用节点，但 2c2g 下 PG/Redis 会和应用抢资源；建议优先“状态机只跑状态组件”，等业务量上来再加第三台应用节点。

## 0. 准备（强烈建议）

1) **准备固定密钥（后续扩容必须一致）**

- `JWT_SECRET`：建议 `openssl rand -hex 32`
- `ADMIN_PASSWORD`：建议固定（避免首启并发时日志里出现多个不同的随机密码）

2) **规划内网地址**

- 记下服务器 A 的内网 IP（下文示例：`10.0.0.10`）
- 记下服务器 B 的内网 IP（用于防火墙放行）

3) **防火墙（最重要）**

- 服务器 A：只允许服务器 B 访问 `5432/6379`（不要暴露公网）
- 服务器 B：只对公网开放 `80/443`（或你在 `.env` 里自定义的 `HTTP_PORT/HTTPS_PORT`）

（示例，使用 `ufw`，按需调整）

```bash
# 服务器 A（状态机）上：
sudo ufw allow from <服务器B内网IP> to any port 5432 proto tcp
sudo ufw allow from <服务器B内网IP> to any port 6379 proto tcp
sudo ufw deny 5432/tcp
sudo ufw deny 6379/tcp
```

## 1. 服务器 A：部署 PostgreSQL + Redis（状态机）

在服务器 A 上：

```bash
cd deploy
cp .env.infra.example .env
```

编辑 `.env`，至少设置：

- `POSTGRES_PASSWORD=...`
- `REDIS_PASSWORD=...`
- （可选）`POSTGRES_BIND_HOST` / `REDIS_BIND_HOST`：建议绑定内网 IP

启动：

```bash
docker compose -f docker-compose.infra.yml up -d
docker compose -f docker-compose.infra.yml ps
```

## 2. 服务器 B：部署 Caddy + FluxCode（入口机）

在服务器 B 上：

```bash
cd deploy
cp .env.app.example .env
```

编辑 `.env`，至少设置：

- `DATABASE_HOST=10.0.0.10`（服务器 A 内网 IP）
- `REDIS_HOST=10.0.0.10`（服务器 A 内网 IP）
- `POSTGRES_PASSWORD=...`（必须与服务器 A 一致）
- `REDIS_PASSWORD=...`（必须与服务器 A 一致）
- `CADDY_ADDRESS=你的域名`（例如 `fluxcode.example.com`）
- `JWT_SECRET=...`（固定值，后续扩容所有节点一致）
- `ADMIN_PASSWORD=...`（固定值，后续扩容所有节点一致）

证书申请前提（否则 Caddy 可能无法签发证书/HTTPS 不可用）：

- 域名 A/AAAA 记录指向服务器 B 的公网 IP
- 服务器 B 的 `80/443` 入站可达，且未被其他服务占用

启动：

```bash
docker compose -f docker-compose.app.yml up -d
docker compose -f docker-compose.app.yml ps
```

访问：

- `https://你的域名/`（推荐）
- `http://你的域名/`（会自动跳转到 https，前提是 80/443 可达）
- 如果暂时还没配域名：先把 `CADDY_ADDRESS` 留空，用 `http://服务器B公网IP/`（仅 HTTP，不会自动签发 HTTPS 证书）

排查：

```bash
docker compose -f docker-compose.app.yml logs -f fluxcode
docker compose -f docker-compose.app.yml logs -f caddy
```

## 3. 后续扩容：增加更多“应用节点”（加第三台服务器时）

当你新增一台服务器 C（作为应用节点）时：

### 3.1 服务器 C：启动 FluxCode 节点

在服务器 C 上：

```bash
cd deploy
cp .env.node.example .env
docker compose -f docker-compose.node.yml up -d
```

关键点：

- `.env` 里 `DATABASE_HOST/REDIS_HOST` 仍指向服务器 A
- `JWT_SECRET/ADMIN_PASSWORD` 必须与入口机一致
- `docker-compose.node.yml` 会把节点 `8080` 映射到宿主机；请用防火墙只允许入口机访问

### 3.2 服务器 B：把新节点加入入口负载均衡

编辑 `deploy/Caddyfile.multihost`，把 upstream 改成多节点列表（示例）：

```caddyfile
reverse_proxy {
    # 方式 B：多机（手动维护节点 IP:PORT 列表）
    # 注意：使用方式 B 时，需要把 Caddyfile.multihost 里默认的 dynamic a 注释掉
    to fluxcode:8080 10.0.0.12:8080 10.0.0.13:8080
    lb_policy cookie fluxcode_sticky
}
```

然后重启入口容器：

```bash
docker compose -f docker-compose.app.yml restart caddy
```

> 提示：如果入口机也跑了本机 `fluxcode`（`docker-compose.app.yml` 默认包含），你可以把本机 upstream 写成 `fluxcode:8080`，再追加其他节点 IP。

## 4. 调优建议（2c2g 常见设置）

- 连接池总量会随应用节点数线性增长：总连接 ≈ 节点数 × 单节点连接池  
  建议在 `.env.app` / `.env.node` 里下调：
  - `DATABASE_MAX_OPEN_CONNS`、`DATABASE_MAX_IDLE_CONNS`
  - `REDIS_POOL_SIZE`、`REDIS_MIN_IDLE_CONNS`
- Redis 一定要有密码 + 防火墙（跨主机暴露 `6379` 非常危险）

## 5. 在本地电脑模拟多机（可选）

你可以在一台电脑上用同一套 compose 文件“分项目启动”来模拟：

- infra：PostgreSQL + Redis（映射到宿主机端口）
- app：Caddy + FluxCode（通过 `host.docker.internal` 访问“宿主机端口”，相当于访问远端）

> 注意：**本地通常无法完整验证 Caddy 的 Let’s Encrypt 自动 HTTPS**（需要域名解析到当前机器公网 IP，且公网可访问 `80/443`）。建议先用 HTTP 验证联通与功能，生产环境再切到真实域名 + `80/443`。

### 5.1 准备本地 env 文件（避免覆盖 `deploy/.env`）

```bash
cd deploy
cp .env.infra.example .env.infra.local
cp .env.app.example .env.app.local
```

### 5.2 启动 infra（本地模拟“状态机”）

编辑 `.env.infra.local`：

- 建议本地只绑定回环：`POSTGRES_BIND_HOST=127.0.0.1`、`REDIS_BIND_HOST=127.0.0.1`
- 如果你本机已有 Postgres/Redis 占用 `5432/6379`，可改成 `POSTGRES_EXPOSE_PORT=15432`、`REDIS_EXPOSE_PORT=16379`

启动：

```bash
docker compose -p fluxcode-infra --env-file .env.infra.local -f docker-compose.infra.yml up -d
docker compose -p fluxcode-infra --env-file .env.infra.local -f docker-compose.infra.yml ps
```

### 5.3 启动 app（本地模拟“入口机”）

编辑 `.env.app.local`：

- `DATABASE_HOST=host.docker.internal`
- `REDIS_HOST=host.docker.internal`
- `DATABASE_PORT` / `REDIS_PORT` 与上一步映射端口保持一致
- 本地避免占用 `80/443`：可设 `HTTP_PORT=8080`、`HTTPS_PORT=8443`
- 本地测试建议先留空 `CADDY_ADDRESS=`（只走 HTTP）

启动：

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml up -d
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml ps
```

访问：

- `http://localhost:8080/`（如果你改了 `HTTP_PORT`，端口按你的值）

### 5.4（可选）在本机模拟“新增应用节点”

如果你想验证“入口反代到多个节点”的方式 B（手动 upstream 列表）：

1) 再准备一个节点 env，并把 `NODE_PORT` 改成非 `8080`（例如 `8081`）：

```bash
cp .env.node.example .env.node1.local
# 编辑 .env.node1.local：
# - DATABASE_HOST/REDIS_HOST=host.docker.internal
# - NODE_PORT=8081
# - JWT_SECRET/ADMIN_PASSWORD 必须与 .env.app.local 一致
docker compose -p fluxcode-node1 --env-file .env.node1.local -f docker-compose.node.yml up -d
```

2) 修改 `deploy/Caddyfile.multihost`：注释 `dynamic a fluxcode 8080`，改用方式 B，例如：

```caddyfile
reverse_proxy {
    to fluxcode:8080 host.docker.internal:8081
    lb_policy cookie fluxcode_sticky
}
```

然后重启入口容器：

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml restart caddy
```

### 5.5 清理

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml down -v
docker compose -p fluxcode-node1 --env-file .env.node1.local -f docker-compose.node.yml down -v
docker compose -p fluxcode-infra --env-file .env.infra.local -f docker-compose.infra.yml down -v
```

> `host.docker.internal` 在 macOS/Windows Docker Desktop 默认可用；Linux 上如不可用，可改用 `172.17.0.1`（或自行添加 `host-gateway` 映射）。

## 6. 两台本地电脑“真·多机”联调（推荐）

如果你手上已经有两台电脑（同一局域网），并且 **PostgreSQL/Redis 已经是独立部署**（可被两台电脑同时访问），可以用下面方式最接近真实“加一台机器扩容”的体验：

### 6.1 规划角色

- **电脑 A（入口机）**：跑 `Caddy + FluxCode`（负责对外入口与负载均衡）
- **电脑 B（新增节点）**：只跑 `FluxCode`（把 `8080` 暴露给入口机）

> 你也可以让电脑 A 上的 `fluxcode` 作为其中一个 upstream（默认就是这样），电脑 B 作为第二个 upstream。

### 6.2 电脑 A：启动入口机（HTTP 本地测试）

在电脑 A 上：

```bash
cd deploy
cp .env.app.example .env.app.local
```

编辑 `deploy/.env.app.local`（关键项）：

- `DATABASE_HOST` / `REDIS_HOST`：填你的外部 PG/Redis 地址
- `POSTGRES_PASSWORD` / `REDIS_PASSWORD`：填实际密码
- `JWT_SECRET` / `ADMIN_PASSWORD`：**固定值**（后续所有节点必须一致）
- 建议本地先用 HTTP：`CADDY_ADDRESS=`（留空），`HTTP_PORT=8080`（避免占用 80）

启动：

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml up -d
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml ps
```

### 6.3 电脑 B：启动新增应用节点

在电脑 B 上：

```bash
cd deploy
cp .env.node.example .env.node.local
```

编辑 `deploy/.env.node.local`（关键项）：

- `DATABASE_HOST` / `REDIS_HOST`：与电脑 A 一样，指向外部 PG/Redis
- `POSTGRES_PASSWORD` / `REDIS_PASSWORD`：与电脑 A 一样
- `JWT_SECRET` / `ADMIN_PASSWORD`：**必须与电脑 A 完全一致**
- `NODE_BIND_HOST=0.0.0.0`，`NODE_PORT=8080`（或你自定义端口）

启动：

```bash
docker compose -p fluxcode-node1 --env-file .env.node.local -f docker-compose.node.yml up -d
docker compose -p fluxcode-node1 --env-file .env.node.local -f docker-compose.node.yml ps
```

在电脑 B 上先自检：

```bash
curl http://localhost:8080/health
```

### 6.4 电脑 A：把电脑 B 加入负载均衡

1) 在电脑 A 上编辑 `deploy/Caddyfile.multihost`：

- 注释掉方式 A：
  - `dynamic a fluxcode 8080`
- 启用方式 B，并加入电脑 B 的内网 IP（示例：`192.168.1.23`）：

```caddyfile
reverse_proxy {
    to fluxcode:8080 192.168.1.23:8080
    lb_policy cookie fluxcode_sticky {$CADDY_LB_COOKIE_SECRET}
}
```

2) 重启入口机 Caddy：

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml restart caddy
```

### 6.5 验证是否生效

- 从任意机器访问入口机（电脑 A）：
  - `http://<电脑A局域网IP>:8080/health`
- 看入口机 Caddy 日志是否有请求分发：

```bash
docker compose -p fluxcode-app --env-file .env.app.local -f docker-compose.app.yml logs -f caddy
```

### 6.6 安全/联通性检查（强烈建议）

- 电脑 B 的 `8080` **不要暴露公网**，只给电脑 A 访问（防火墙放行入口机 IP）。
- 外部 PG/Redis 同理：只允许应用节点所在网段/入口机访问。
