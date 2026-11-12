# 小鹿童装 Fawn Mall · 部署方案

> 目标形态:**单台云服务器 + Docker Compose**,镜像由 GitHub Actions(CD)自动构建推送至
> ghcr.io,服务器只做 `pull + up`。适合本项目体量;后续横向扩展见文末「演进路径」。

## 1. 架构与产物链路

```
push main / 打 tag v*.*.*
        │
        ▼  .github/workflows/cd.yml
┌─────────────────────────────┐
│ CI 构建(多阶段,静态产物)  │
│  server: Go → 静态二进制    │──► ghcr.io/ljl0628/tz-mall/fawn-server
│  web: Vue → Nginx 静态资源  │──► ghcr.io/ljl0628/tz-mall/fawn-web
└─────────────────────────────┘        tag: main / sha-xxxxxxx / v1.2.3 / latest
        │(可选)SSH 自动部署,默认禁用
        ▼
┌── 生产服务器(2C4G,Docker)───────────────────────────────┐
│ 宿主 Nginx(80/443,TLS 终结,certbot 自动续期)          │
│   ├─ / → 127.0.0.1:8090  web 容器(SPA + /api 反代)    │
│   └─ (可选)直通 127.0.0.1:8080/healthz 做探活          │
│ Docker 网络 fawn-mall:web → server → mysql/redis/rabbitmq│
│ 基础设施端口仅绑定 127.0.0.1(3316/6381/5673/15673)      │
│ 数据卷:mysql / redis / rabbitmq / server-logs           │
└──────────────────────────────────────────────────────────┘
```

镜像内置约定(无需服务器装 Go/Node):

- 后端:CGO_ENABLED=0 静态二进制,非 root 运行;镜像内含 `config.example.yaml` 兜底,
  真实配置全部经 `APP_*` 环境变量注入(见 `server/pkg/config`);
- 前端:Nginx 托管 dist,`/api` 反代到服务名 `server:8080`(生产编排沿用该服务名);
- 启动即强校验:prod 下 `APP_JWT_SECRET` ≥ 32 字节、密码类非空,不合规直接拒绝启动。

## 2. 服务器要求

| 项 | 要求 |
| --- | --- |
| 规格 | 最低 1C2G(演示),推荐 2C4G + 40GB 系统盘 |
| 系统 | Ubuntu 22.04 / Debian 12 等主流发行版 |
| 软件 | Docker Engine ≥ 24(含 `docker compose` v2 插件) |
| 端口开放 | 公网仅 80 / 443 / 22;**3306/6379/5672/8080/8090 不得对公网开放** |

```bash
# Ubuntu 初始化示例
sudo apt update && sudo apt install -y docker.io docker-compose-plugin nginx certbot python3-certbot-nginx ufw
sudo ufw allow 22,80,443/tcp && sudo ufw enable
```

## 3. 首次部署(约 10 分钟)

### 3.1 准备目录与配置

```bash
sudo mkdir -p /opt/fawn-mall && cd /opt/fawn-mall
# 方式一:git clone(推荐,CD 自动部署依赖仓库内的 docker-compose.prod.yml)
git clone https://github.com/ljl0628/tz-mall.git .

# 生成生产密钥(全部口令禁止沿用开发默认值)
cp .env.example .env && chmod 600 .env
openssl rand -hex 24   # ×4:分别用于 MYSQL_ROOT_PASSWORD / MYSQL_PASSWORD / RABBITMQ_PASSWORD
openssl rand -hex 32   # JWT_SECRET(≥32 字节硬性要求)

# 固定镜像版本(强烈建议;不写则默认 latest)
echo "TAG=sha-xxxxxxx" >> .env   # 或正式版 v1.2.3
```

### 3.2 拉起

```bash
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml ps   # 五个容器均 Up(healthy)
```

> **ghcr 私有镜像提示**:Actions 首次推送的包默认私有。要么在 GitHub → Packages →
> fawn-server/fawn-web → Package settings 设为 Public(最简单),要么在服务器
> `docker login ghcr.io`(用户名 + PAT,需 read:packages 权限)。

### 3.3 验证

```bash
curl -s http://127.0.0.1:8080/healthz          # {"status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8090/   # 200
docker exec fawn-server tail -5 /app/logs/fawn-mall.log          # JSON 日志正常写入
# 浏览器访问 http://<ip>:8090(临时),admin / admin123 登录后立刻改密
```

### 3.4 HTTPS(强烈建议,上线必做)

宿主 Nginx 终结 TLS,反代到 web 容器:

```bash
sudo tee /etc/nginx/sites-available/fawn-mall <<'EOF'
server {
    listen 80;
    server_name mall.example.com;          # 换成你的域名
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
sudo certbot --nginx -d mall.example.com    # 自动配置 443 + 续期
```

之后 `docker-compose.prod.yml` 里 web 的 `127.0.0.1:8090` 映射保持不变即可,
公网流量统一走 80/443。

## 4. 生产配置清单(.env)

| 变量 | 说明 | 生产要求 |
| --- | --- | --- |
| `TAG` | 前后端镜像 tag | 固定为 `sha-xxxxxxx` 或 `v1.2.3`,可回滚 |
| `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` | 数据库口令 | 随机 ≥24 位 |
| `RABBITMQ_PASSWORD` | MQ 口令 | 随机 ≥24 位 |
| `JWT_SECRET` | 登录态签名密钥 | 随机 ≥32 字节(启动硬校验) |
| `MYSQL_DATABASE` / `MYSQL_USER` / `RABBITMQ_USER` | 账号库表名 | 默认即可 |

应用参数(支付超时、连接池、日志保留等)如需调整:在 `server/config/config.example.yaml`
基础上建 `config.yaml` 挂载进容器,或追加 `APP_*` 环境变量(优先级最高)。
常用项:`APP_ORDER_PAY_TIMEOUT_SECONDS`、`APP_LOG_LEVEL`(支持**改配置文件热加载**)、
`APP_LOG_MAX_SIZE_MB`。

## 5. 升级与回滚

```bash
# 发布:打 tag → CD 自动出镜像 → 服务器换 tag
git tag v1.1.0 && git push origin v1.1.0        # 等 Actions 镜像构建完成
cd /opt/fawn-mall
sed -i 's/^TAG=.*/TAG=v1.1.0/' .env
docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d

# 回滚:改回上一个 tag 重复上述 pull + up(数据库卷不动,秒级完成)
sed -i 's/^TAG=.*/TAG=v1.0.0/' .env && docker compose -f docker-compose.prod.yml pull && docker compose -f docker-compose.prod.yml up -d
```

注意:

- 后端启动时 AutoMigrate 自动加表/加列,**不删列不改类型**;跨版本回滚前端镜像时,
  若新版已新增列,旧代码忽略多余列通常无害,但仍建议先在测试环境验证;
- 每次升级前执行一次数据库备份(见 §6)。

### 自动部署(可选)

`cd.yml` 的 deploy job 已预置(默认 `if: false` 禁用)。启用:

1. 仓库 Settings → Secrets and variables → Actions 添加
   `SSH_HOST` / `SSH_USER` / `SSH_KEY`(私钥全文);
2. 删除 deploy job 的 `if: false` 行;
3. 之后每次 push main,CD 构建完镜像自动在服务器执行
   `cd /opt/fawn-mall && docker compose -f docker-compose.prod.yml pull && up -d`。

建议正式环境用「打 tag 手动触发 + environment 审批」而非 main 直推:.github 里给 deploy
job 的 `environment: production` 在仓库 Settings → Environments 配置 Required reviewers。

## 6. 数据备份与恢复

```bash
# 每日 03:00 备份 MySQL(保留 7 天)
sudo crontab -e
0 3 * * * docker exec fawn-mysql sh -c 'mysqldump -uroot -p"$MYSQL_ROOT_PASSWORD" --single-transaction fawn_mall' | gzip > /var/backups/fawn-mall-$(date +\%F).sql.gz && find /var/backups -name 'fawn-mall-*.sql.gz' -mtime +7 -delete

# 恢复
gunzip < /var/backups/fawn-mall-2026-11-12.sql.gz | docker exec -i fawn-mysql sh -c 'mysql -uroot -p"$MYSQL_ROOT_PASSWORD" fawn_mall'
```

RabbitMQ/Redis 数据可重建(队列拓扑、缓存由服务启动重建),一般无需备份;
如需:`docker run --rm -v fawn-mall_fawn-rabbitmq-data:/data -v /var/backups:/bk alpine tar czf /bk/rabbitmq.tgz -C /data .`

## 7. 日志与监控

- **应用日志**:容器 `/app/logs/fawn-mall.log`(JSON,lumberjack 100MB 切割/14 天/10 份/gzip),
  已挂载 `fawn-server-logs` 卷,容器重建不丢;
  `docker exec fawn-server tail -f /app/logs/fawn-mall.log`;
  排障按 `request_id` 串联一次请求的 访问→业务→错误 日志;
- **级别热切换**:`docker exec fawn-server sh -c 'sed "s/level: info/level: debug/" config/config.yaml > t && cat t > config/config.yaml'`
  ——仅 log.level 即时生效,其余配置需重启(注意:不要用会重写文件的编辑器做原子替换,
  `cat >` 原地覆写最稳);
- **健康检查**:compose healthchecks + `GET /healthz`;可接入 UptimeRobot / 阿里云监控打 443 域名;
- **资源观察**:`docker stats`;磁盘看 `docker system df`,日志卷膨胀先调小 `APP_LOG_MAX_SIZE_MB`;
- **可选进阶**:Dozzle(`docker logs` Web 面板)或 Loki + Promtail 采集 JSON 日志文件。

## 8. 故障排查速查

| 现象 | 排查 |
| --- | --- |
| server 反复重启 | `docker logs fawn-server`:多为配置校验失败(zap fatal 会指明字段,如 JWT_SECRET 过短、.env 缺项) |
| 前端 502 | `docker ps` 看 web/server 是否健康;`curl 127.0.0.1:8080/healthz`;宿主 Nginx `error.log` |
| 登录 50001 | 查 `fawn-server` 日志 error 级:DB/Redis/MQ 连接失败会有 op+stack |
| MQ 队列堆积 | `ssh -L 15673:127.0.0.1:15673 user@host` 后浏览器开 http://localhost:15673 |
| 数据库连不上 | `.env` 口令与 MYSQL 容器初始化不一致:首次启动后改 .env 不会同步改库,需 `docker compose down` 删 `fawn-mysql-data` 卷重初始化(**慎用,会丢数据**)或进容器改密 |
| 升级后起不来 | 回滚 TAG(§5);仍失败看 `docker logs` fatal 信息 |

## 9. 演进路径(体量增长后)

- **多实例**:`server` 无状态(登录态 JWT、共享数据都在 Redis/MySQL),直接 `docker compose up --scale server=2` + 宿主 Nginx upstream 负载均衡即可;
- **托管中间件**:MySQL → RDS,Redis → ElastiCache/阿里云 Redis,RabbitMQ → MQ 服务,compose 只跑无状态前后端;
- **K8s**:镜像产物不变,补 Deployment/Ingress/HPA manifests;健康检查用 `/healthz`,
  日志走 stdout 之外另接 Filebeat 采 `/app/logs` 或改 `APP_LOG_FORMAT` 双写。
