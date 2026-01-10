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
