# 小鹿童装 Fawn Mall · 腾讯云部署方案

> 形态:**GitHub Actions(CI/CD)→ 腾讯云镜像仓库 TCR → SSH 自动部署到腾讯云服务器(CVM/轻量)**。
> 服务器只做 `pull + up`,不在服务器上构建。前置条件:腾讯云服务器与镜像仓库已就绪。
>
> **多项目共存**:方案按「服务器上已有其他项目」设计 —— 本项目不独占任何公网端口,
> 80/443 复用宿主机上**已有的** Nginx(新增一个虚拟主机按域名分流),本项目全部端口
> 只绑 127.0.0.1 且默认高位(13306/16379/25672/18080/18090),可在 .env 逐个覆盖避让。

## 1. 架构与产物链路

```
push main / 打 tag v*.*.*
        │
        ▼  .github/workflows/cd.yml(GitHub Actions)
┌────────────────────────────────┐
│ 构建多阶段镜像(静态产物)      │
│  server: Go → 静态二进制       │──► TCR:${REGISTRY}/${NAMESPACE}/fawn-server
│  web: Vue → Nginx 静态资源     │──► TCR:${REGISTRY}/${NAMESPACE}/fawn-web
└────────────────────────────────┘    tag: main / sha-xxxxxxx / v1.2.3 / latest
        │  DEPLOY_ENABLED=true 时自动 SSH(默认关,防止误发布)
        ▼
┌── 腾讯云服务器(Docker,与其他项目共存)───────────────────┐
│ 宿主 Nginx(80/443,已有实例,多域名虚拟主机分流)        │
│   ├─ other.example.com → 已有项目                          │
│   └─ mall.example.com  → 127.0.0.1:18090(本项目 web)     │
│ Docker 网络 fawn-mall:web → server → mysql/redis/rabbitmq  │
│ 本项目端口仅 127.0.0.1:13306/16379/25672/18080/18090      │
│ 数据卷:mysql / redis / rabbitmq / server-logs(独立命名) │
└───────────────────────────────────────────────────────────┘
```

镜像内置约定(服务器无需安装 Go/Node):

- 后端:CGO_ENABLED=0 静态二进制,非 root 运行;镜像内含 `config.example.yaml` 兜底,
  真实配置全部经 `APP_*` 环境变量注入(见 `server/pkg/config`);
- 前端:Nginx 托管 dist,`/api` 反代到服务名 `server:8080`(生产编排沿用该服务名);
- 启动即强校验:prod 下 `APP_JWT_SECRET` ≥ 32 字节、密码类非空,不合规直接拒绝启动。

## 2. 服务器与镜像仓库要求

| 项 | 要求 |
| --- | --- |
| 机型 | CVM 或轻量应用服务器;本项目额外占用约 **1GB 内存**(MySQL+RabbitMQ+Redis+前后端),已有项目同机时建议合计 ≥4G 或加 swap |
| 系统 | Ubuntu 22.04 / Debian 12(CVM);轻量可选「Docker 应用镜像」 |
| 软件 | Docker Engine ≥ 24(含 `docker compose` v2 插件);宿主 Nginx + certbot(**若已有项目占 80/443,直接复用它的 Nginx,不要重复安装**) |
| 安全组 | 公网入站仅 **22 / 80 / 443**(与其他项目共用即可);本项目所有端口只绑 127.0.0.1,无需新增放行 |
| 磁盘 | 本项目镜像 + 数据 + 日志约 5-8GB,预留 15GB 以上 |
| TCR | 已建命名空间(建议小写);**与服务器同地域**(走内网拉取免公网流量费) |

**TCR 凭证(CI/CD 必须长期有效)**:

- 企业版 TCR(推荐):域名 `<实例名>-tcr.tencentcloudcr.com`,控制台
  「容器镜像服务 → 访问凭证」创建**长期凭证**,填入 GitHub Secrets;
- 个人版(`ccr.ccs.tencentyun.com`):控制台获取的密码若为**临时密码(约 1 小时有效)**,
  无法用于 CI —— 需确认可设置长期/固定密码,或升级企业版标准版。

```bash
# CVM 初始化示例(Ubuntu)
sudo apt update && sudo apt install -y docker.io docker-compose-plugin nginx certbot python3-certbot-nginx
# 安全组在腾讯云控制台配置,无需服务器内再开 ufw
```

## 3. 首次部署(约 15 分钟)

### 3.1 配置 CI/CD(仓库侧,先做,否则下次 push CD 会失败)

仓库 **Settings → Secrets and variables → Actions**:

| 类型 | 名称 | 值 |
| --- | --- | --- |
| Variable | `TCR_REGISTRY` | 如 `ccr.ccs.tencentyun.com` 或 `<实例名>-tcr.tencentcloudcr.com` |
| Variable | `TCR_NAMESPACE` | TCR 命名空间(小写) |
| Variable | `DEPLOY_ENABLED` | `true`(启用自动部署;不配则只构建推送) |
| Secret | `TCR_USERNAME` | TCR 访问凭证用户名 |
| Secret | `TCR_PASSWORD` | TCR 访问凭证密码(长期) |
| Secret | `SSH_HOST` | 服务器公网 IP |
| Secret | `SSH_USER` | 登录用户(如 `ubuntu` / `root`) |
| Secret | `SSH_KEY` | 私钥**全文**(对应服务器已装的公钥;CVM 绑定密钥对即可) |

> 生产发布想加人工确认:Settings → Environments → `production` → Required reviewers。

### 3.2 初始化服务器

```bash
ssh <user>@<server-ip>
sudo mkdir -p /opt/fawn-mall && sudo chown $USER /opt/fawn-mall && cd /opt/fawn-mall
git clone https://github.com/ljl0628/tz-mall.git .    # 需要 docker-compose.prod.yml 与 .env 模板

# 生成生产密钥(全部口令禁止沿用开发默认值)
cp .env.example .env && chmod 600 .env
openssl rand -hex 24   # ×4:分别用于 MYSQL_ROOT_PASSWORD / MYSQL_PASSWORD / RABBITMQ_PASSWORD
openssl rand -hex 32   # JWT_SECRET(≥32 字节硬性要求)

# 镜像仓库参数(与 CI 的 Variables 保持一致)
cat >> .env <<'EOF'
REGISTRY=ccr.ccs.tencentyun.com      # 换成你的 TCR 域名
NAMESPACE=<你的命名空间>
TAG=main                              # 自动跟随主干;固定版本用 v1.2.3 / sha-xxxxxxx
# 与同机其他项目端口相撞时,取消注释并改端口:
# SERVER_PORT=18080                   # 后端 healthz 直连口(127.0.0.1)
# WEB_PORT=18090                      # 前端入口,宿主 Nginx 反代到此(127.0.0.1)
# MYSQL_PORT=13306                    # MySQL 调试口(SSH 隧道用,127.0.0.1)
# REDIS_PORT=16379
# MQ_MGMT_PORT=25672                  # RabbitMQ 管理台(SSH 隧道用,127.0.0.1)
EOF

# TCR 私有镜像:服务器登录一次(凭证会存入 ~/.docker/config.json)
docker login $REGISTRY
```

### 3.3 拉起并验证

```bash
# 先手动跑一次 CD(或本地 push 触发),确保 TCR 里已有镜像,然后:
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml ps            # 五个容器 Up(healthy)

curl -s http://127.0.0.1:18080/healthz                  # {"status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18090/   # 200
docker exec fawn-server tail -5 /app/logs/fawn-mall.log # JSON 日志正常写入
docker ps --format '{{.Names}}\t{{.Status}}' | sort     # 与其他项目容器互不影响
```

对外访问统一走域名 + 443(§3.4);`admin / admin123` 登录后立刻修改密码。

### 3.4 HTTPS:在【已有】Nginx 上新增虚拟主机(不新装、不重启其他站点)

服务器上的 Nginx 大概率已在为其他项目服务 —— 只需**追加**一个 server 块,
按域名(server_name)分流,互不影响:

```bash
sudo tee /etc/nginx/sites-available/fawn-mall <<'EOF'
server {
    listen 80;
    server_name mall.example.com;              # 本项目域名,解析到本机公网 IP
    location / {
        proxy_pass http://127.0.0.1:18090;     # 本项目 web 容器(SPA + /api 同源反代)
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
sudo ln -sf /etc/nginx/sites-available/fawn-mall /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx    # reload 不断连接,其他站点无感
sudo certbot --nginx -d mall.example.com        # 自动改写 443 + 定时续期
```

两种特殊情况:

- **已有项目用非标准目录结构**(如所有站点写在 nginx.conf 里):把上面 server 块
  追加到对应 http {} 内即可,效果等同;
- **已有项目的 Nginx 跑在 Docker 里**占了 80/443:不要另起宿主 Nginx(端口必撞)。
  把本项目的 server 块挂进那个容器的配置挂载目录;或者临时方案 —— 安全组放行
  18090 并在 compose 中把 web 改为 `0.0.0.0:18090:80` 直接裸跑(尽快补 TLS)。

```bash
sudo tee /etc/nginx/sites-available/fawn-mall <<'EOF'
server {
    listen 80;
    server_name mall.example.com;          # 换成你的域名(需已解析到本机公网 IP)
    location / {
        proxy_pass http://127.0.0.1:8090;  # web 容器(SPA + /api 同源反代)
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF
sudo ln -sf /etc/nginx/sites-available/fawn-mall /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d mall.example.com    # 自动配置 443 + 定时续期
```

## 4. CI/CD 自动部署运行方式

配置完成后,**每次 push main**:

1. CI 构建 fawn-server / fawn-web,推送 TCR(tag:`main` + `sha-xxxxxxx`);
2. deploy job SSH 到服务器:`docker login TCR → compose pull → up -d`,输出最终容器状态;
3. 服务器 `.env` 的 `TAG` 决定拉哪个版本:

| TAG 值 | 行为 | 适用 |
| --- | --- | --- |
| `main` | 每次自动部署最新主干 | 测试环境 / 个人项目 |
| `v1.2.3`(打 tag 触发,附带 `latest`) | 固定正式版 | 生产推荐 |
| `sha-xxxxxxx` | 固定某次提交,精确回滚 | 事故回滚 |

**打正式版**:`git tag v1.1.0 && git push origin v1.1.0` → CI 出镜像 → 服务器
`sed -i 's/^TAG=.*/TAG=v1.1.0/' .env && docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d`。

**回滚**:同上把 TAG 改回旧值即可(数据库卷不动,秒级完成)。

注意事项:

- 后端启动时 AutoMigrate 自动加表/加列,**不删列不改类型**;跨版本回滚建议先在测试环境验证;
- 每次升级前执行一次数据库备份(§5);
- deploy job 挂了可看 Actions 日志;常见原因:安全组未放行 22、SSH_KEY 不匹配、TCR 密码过期。

### .env 生产配置清单

| 变量 | 说明 | 生产要求 |
| --- | --- | --- |
| `REGISTRY` / `NAMESPACE` | TCR 地址与命名空间 | 与 CI Variables 一致 |
| `TAG` | 镜像版本 | `main` / `v1.2.3` / `sha-xxxxxxx` |
| `SERVER_PORT` / `WEB_PORT` / `MYSQL_PORT` / `REDIS_PORT` / `MQ_MGMT_PORT` | 本机端口覆盖(默认 18080/18090/13306/16379/25672) | 仅与其他项目相撞时改 |
| `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` | 数据库口令 | 随机 ≥24 位 |
| `RABBITMQ_PASSWORD` | MQ 口令 | 随机 ≥24 位 |
| `JWT_SECRET` | 登录态签名密钥 | 随机 ≥32 字节(启动硬校验) |

**与其他项目的隔离**:本项目独立 compose project(`fawn-mall`)、独立 Docker 网络、
独立数据卷、容器名前缀 `fawn-`;CD 的 `up -d --remove-orphans` 只作用于
`/opt/fawn-mall` 目录,不会触碰同机其他项目的容器与网络。

应用参数(支付超时、连接池、日志保留等)如需调整:在 `server/config/config.example.yaml`
基础上建 `config.yaml` 挂载进容器,或追加 `APP_*` 环境变量(优先级最高)。
常用项:`APP_ORDER_PAY_TIMEOUT_SECONDS`、`APP_LOG_LEVEL`(支持**改配置文件热加载**)、
`APP_LOG_MAX_SIZE_MB`。

## 5. 数据备份与恢复

```bash
# 每日 03:00 备份 MySQL(保留 7 天)
sudo crontab -e
0 3 * * * docker exec fawn-mysql sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction fawn_mall' | gzip > /var/backups/fawn-mall-$(date +\%F).sql.gz && find /var/backups -name 'fawn-mall-*.sql.gz' -mtime +7 -delete

# 恢复
gunzip < /var/backups/fawn-mall-2026-11-12.sql.gz | docker exec -i fawn-mysql sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" fawn_mall'
```

建议快照加持:腾讯云控制台对系统盘/数据盘配置**定期快照策略**(每日 1 次,保留 7 天),
整机能级恢复比单库备份更全。

RabbitMQ/Redis 数据可重建(队列拓扑、缓存由服务启动重建),一般无需备份。

## 6. 日志与监控

- **应用日志**:容器 `/app/logs/fawn-mall.log`(JSON,lumberjack 100MB 切割/14 天/10 份/gzip),
  已挂载 `fawn-server-logs` 卷,容器重建不丢;
  `docker exec fawn-server tail -f /app/logs/fawn-mall.log`;
  排障按 `request_id` 串联一次请求的 访问→业务→错误 日志;
- **级别热切换**:`docker exec fawn-server sh -c 'sed "s/level: info/level: debug/" config/config.yaml > /tmp/t && cat /tmp/t > config/config.yaml'`
  ——仅 log.level 即时生效,其余配置需重启(用 `cat >` 原地覆写,避免重命名文件导致监听失效);
- **健康检查**:compose healthchecks + `GET /healthz`;可接入腾讯云「云监控」拨测 443 域名,
  或 CloudWatch/UptimeRobot 类产品;
- **资源观察**:`docker stats`;磁盘 `docker system df`;腾讯云控制台自带的 CPU/内存/云盘
  监控与告警阈值建议配上(磁盘 >80% 告警);
- **可选进阶**:腾讯云 CLS 日志服务采集 JSON 日志文件,或轻量方案 Dozzle(Web 面板)。

## 7. 故障排查速查

| 现象 | 排查 |
| --- | --- |
| CD build job 失败 | Actions 日志;多为 TCR 登录失败(密码过期/凭证无权限/Variable 未配置) |
| CD deploy job 失败 | 安全组 22 未放行、SSH_KEY 与服务器密钥对不匹配、服务器磁盘满 |
| 端口被同机其他项目占用 | `docker compose -f docker-compose.prod.yml ps` 后 `ss -lntp | grep 18090` 找占用者,在 .env 覆盖对应 *_PORT 再 `up -d` |
| server 反复重启 | `docker logs fawn-server`:配置校验失败会 zap fatal 指明字段(JWT_SECRET 过短、.env 缺项) |
| 前端 502 | `docker ps` 看 web/server 是否健康;`curl 127.0.0.1:18080/healthz`;宿主 Nginx `error.log`;域名解析与已有项目的 server_name 是否冲突 |
| 登录 50001 | 查 fawn-server 日志 error 级:DB/Redis/MQ 连接失败带 op+stack |
| MQ 队列堆积 | `ssh -L 25672:127.0.0.1:25672 user@host` 后浏览器开 http://localhost:25672 |
| 数据库连不上 | `.env` 口令与 MySQL 容器首次初始化不一致:改 .env 不会同步改库;需进容器改密或重置数据卷(慎用,丢数据) |
| 升级后起不来 | 回滚 TAG(§4);仍失败看 `docker logs` fatal 信息 |

## 8. 演进路径(体量增长后)

- **多实例**:`server` 无状态(登录态 JWT、共享数据在 Redis/MySQL),
  `docker compose up --scale server=2` + 宿主 Nginx upstream 负载均衡;
- **托管中间件**:MySQL → 腾讯云数据库 MySQL,Redis → 腾讯云 Redis,MQ → TDMQ RabbitMQ,
  服务器只跑无状态前后端;
- **K8s**:镜像产物不变,迁腾讯云 TKE;健康检查 `/healthz`,日志可经
  `APP_LOG_FORMAT` 改 stdout 由容器日志采集,或 Filebeat 采 `/app/logs`。
