# 小鹿童装 Fawn Mall · 腾讯云部署方案

> 形态:**GitHub Actions(CI/CD)→ 腾讯云镜像仓库 TCR → SSH 自动部署到腾讯云服务器**。
> 服务器只做 `pull + up`,不在服务器上构建。
>
> **本部署的实际拓扑**(与同机已有 blog 项目共存):
> - **MySQL 用腾讯云数据库 CDB**(内网直连,不占服务器内存,不进编排);
> - **80/443 复用同机已有的 blog-nginx 容器**:本项目 web 容器加入其网络,
>   nginx 新增 `mall.ljl0628.cn` 虚拟主机按域名分流,零新增公网端口;
> - 本项目其余端口只绑 127.0.0.1 高位口(16379/25672/18080/18090),可在 .env 覆盖。

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
┌── 腾讯云服务器(Docker,与 blog 项目共存)──────────────────────────┐
│ blog-nginx 容器(持有 80/443,多域名虚拟主机分流)                 │
│   ├─ ljl0628.cn / www     → blog 项目(已有,不动)               │
│   ├─ admin.ljl0628.cn     → blog-admin(已有,不动)              │
│   └─ mall.ljl0628.cn      → fawn-web:80(经 blog_default 网络)  │
│ fawn-mall 编排(独立网络/卷/容器名前缀):                        │
│   web ──/api──► server ──► Redis / RabbitMQ(容器)               │
│                        └──► 腾讯云 MySQL 172.16.0.3(内网)       │
│ 本项目端口仅 127.0.0.1:16379/25672/18080/18090                  │
│ 数据卷:redis / rabbitmq / server-logs(独立命名)                │
└──────────────────────────────────────────────────────────────────┘
```

镜像内置约定(服务器无需安装 Go/Node):

- 后端:CGO_ENABLED=0 静态二进制,非 root 运行;镜像内含 `config.example.yaml` 兜底,
  真实配置全部经 `APP_*` 环境变量注入(见 `server/pkg/config`);
- 前端:Nginx 托管 dist,`/api` 反代到服务名 `server:8080`(生产编排沿用该服务名);
- 启动即强校验:prod 下 `APP_JWT_SECRET` ≥ 32 字节、密码类非空,不合规直接拒绝启动。

## 2. 服务器与镜像仓库要求

| 项 | 要求 |
| --- | --- |
| 机型 | CVM 或轻量应用服务器;本项目额外占用约 **600MB 内存**(Redis+RabbitMQ+前后端,MySQL 已在云上) |
| 软件 | Docker Engine ≥ 24(含 `docker compose` v2 插件);**80/443 由同机 blog-nginx 容器承载,复用它,不新装 Nginx** |
| 安全组 | 公网入站仅 **22 / 80 / 443**;本项目所有端口只绑 127.0.0.1,无需新增放行 |
| 磁盘 | 本项目镜像 + 数据 + 日志约 3-5GB,预留 10GB 以上 |
| TCR | 已建命名空间(小写);**与服务器同地域**(走内网拉取免公网流量费) |
| CDB | MySQL 8.0,与服务器**同 VPC**(内网地址如 172.16.0.3);已建库 `fawn_mall`;**建议建专用账号仅授权该库**,避免用 root |

**TCR 凭证(CI/CD 必须长期有效)**:

- 企业版 TCR(推荐):域名 `<实例名>-tcr.tencentcloudcr.com`,控制台
  「容器镜像服务 → 访问凭证」创建**长期凭证**,填入 GitHub Secrets;
- 个人版(`ccr.ccs.tencentyun.com`):控制台获取的密码若为**临时密码(约 1 小时有效)**,
  无法用于 CI —— 需确认可设置长期/固定密码,或升级企业版标准版。

**TLS 证书**:与 blog 项目同款 —— 腾讯云控制台「SSL 证书」申请**免费 TrustAsia DV 证书**
(DNS 验证,几分钟签发),域名 `mall.ljl0628.cn`;下载 nginx 格式,得到
`mall.ljl0628.cn_bundle.crt` + `mall.ljl0628.cn.key`,放到服务器 `/opt/blog/nginx/certs/`。

## 3. 首次部署(约 15 分钟)

### 3.1 配置 CI/CD(仓库侧,先做,否则下次 push CD 会失败)

仓库 **Settings → Secrets and variables → Actions → Secrets**(镜像仓库地址与命名空间为非敏感项,
已直接固化在 `.github/workflows/cd.yml` 顶部 `env`,不依赖仓库 Variables):

| 类型 | 名称 | 值 |
| --- | --- | --- |
| Secret | `TCR_USERNAME` | TCR 访问凭证用户名 |
| Secret | `TCR_PASSWORD` | TCR 访问凭证密码(长期) |
| Secret | `SSH_HOST` | 服务器公网 IP |
| Secret | `SSH_USER` | 登录用户(如 `ubuntu` / `root`) |
| Secret | `SSH_KEY` | 私钥**全文**(对应服务器已装的公钥;CVM 绑定密钥对即可) |

> 自动部署默认开启;仓库 Variable `DEPLOY_ENABLED=false` 可整体停用(只构建推送)。
> 生产发布想加人工确认:Settings → Environments → `production` → Required reviewers。

### 3.2 初始化服务器

```bash
ssh <user>@<server-ip>
sudo mkdir -p /opt/fawn-mall && sudo chown $USER /opt/fawn-mall && cd /opt/fawn-mall
git clone https://github.com/ljl0628/tz-mall.git .    # 需要 docker-compose.prod.yml

# 生产配置:以模板为底,填真实值(数据库连接 + 随机口令),全部禁止沿用开发默认值
cp .env.prod.example .env && chmod 600 .env
openssl rand -hex 24   # → RABBITMQ_PASSWORD
openssl rand -hex 32   # → JWT_SECRET(≥32 字节硬性要求)

# TCR 私有镜像:服务器登录一次(凭证会存入 ~/.docker/config.json)
docker login $REGISTRY
```

`.env` 关键项(完整清单见 §4):`REGISTRY`/`NAMESPACE` 与 GitHub Variables 一致;
`DB_HOST/DB_PORT/DB_NAME/DB_USER/DB_PASSWORD` 填 CDB 内网连接信息。

### 3.3 拉起并验证

```bash
# 先手动跑一次 CD(或本地 push 触发),确保 TCR 里已有镜像,然后:
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
docker compose -f docker-compose.prod.yml ps            # 四个容器 Up(healthy)

curl -s http://127.0.0.1:18080/healthz                  # {"status":"ok"}
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:18090/   # 200
docker exec fawn-server tail -5 /app/logs/fawn-mall.log # JSON 日志正常写入
docker ps --format '{{.Names}}\t{{.Status}}' | sort     # 与 blog 项目容器互不影响
```

首次启动 AutoMigrate 会在 CDB 的 `fawn_mall` 库自动建表(只加不改删)。

### 3.4 对外接入:在 blog-nginx 上新增 mall 虚拟主机

blog-nginx 是**容器**,配置按文件挂载(`/opt/blog/nginx/*.conf` → `/etc/nginx/conf.d/`),
证书手动放 `/opt/blog/nginx/certs/`(已有 ljl0628.cn / admin 两对,TrustAsia 免费证书)。
照同样模式加 mall —— **只新增文件 + blog compose 加一行挂载,不改任何已有站点**:

```bash
# ① 证书:控制台申请 mall.ljl0628.cn 免费证书(nginx 格式),上传到服务器:
#    /opt/blog/nginx/certs/mall.ljl0628.cn_bundle.crt + mall.ljl0628.cn.key

# ② 新增虚拟主机(HTTP 301 → HTTPS,反代到 fawn-web 容器名)
sudo tee /opt/blog/nginx/mall.conf <<'EOF'
server {
    listen 80;
    server_name mall.ljl0628.cn;
    return 301 https://$host$request_uri;
}
server {
    listen 443 ssl;
    http2 on;
    server_name mall.ljl0628.cn;
    ssl_certificate     /etc/nginx/certs/mall.ljl0628.cn_bundle.crt;
    ssl_certificate_key /etc/nginx/certs/mall.ljl0628.cn.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    client_max_body_size 10m;

    # SPA + /api 同源反代,一转发到底(fawn-web 容器内 nginx 自行分流 /api → server)
    location / {
        proxy_pass http://fawn-web:80;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
EOF

# ③ blog compose 增加一行挂载(nginx.volumes 列表里,与 blog.conf 并列):
#      - ./nginx/mall.conf:/etc/nginx/conf.d/mall.conf:ro
#    然后只重建 nginx 容器(1-2 秒,blog 其他容器无感):
cd /opt/blog && docker compose -f compose.prod.yml up -d nginx

# ④ 验证
curl -sI https://mall.ljl0628.cn/ | head -3            # 200/301 正常
docker logs blog-nginx --tail 20                        # 无证书/DNS 报错
```

> 证书到期前(免费证书 90 天)在控制台续签下载,覆盖 certs/ 下同名文件后
> `docker exec blog-nginx nginx -s reload` 即可,无需重建容器。

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

**回滚**:同上把 TAG 改回旧值即可(数据不动,秒级完成)。

注意事项:

- 后端启动时 AutoMigrate 自动加表/加列,**不删列不改类型**;跨版本回滚建议先在测试环境验证;
- 每次升级前确认 CDB 备份策略已开(§5);
- deploy job 挂了可看 Actions 日志;常见原因:安全组未放行 22、SSH_KEY 不匹配、TCR 密码过期。

### .env 生产配置清单

| 变量 | 说明 | 生产要求 |
| --- | --- | --- |
| `REGISTRY` / `NAMESPACE` | TCR 地址与命名空间 | 与 cd.yml 顶部 env 固定值一致 |
| `TAG` | 镜像版本 | `main` / `v1.2.3` / `sha-xxxxxxx` |
| `DB_HOST` / `DB_PORT` / `DB_NAME` | 腾讯云 MySQL 内网地址/端口/库名 | 同 VPC 内网直连 |
| `DB_USER` / `DB_PASSWORD` | 数据库账号 | **建议专用账号仅授权 fawn_mall 库**,密码随机 ≥24 位 |
| `SERVER_PORT` / `WEB_PORT` / `REDIS_PORT` / `MQ_MGMT_PORT` | 本机端口覆盖(默认 18080/18090/16379/25672) | 仅与其他项目相撞时改 |
| `RABBITMQ_PASSWORD` | MQ 口令 | 随机 ≥24 位 |
| `JWT_SECRET` | 登录态签名密钥 | 随机 ≥32 字节(启动硬校验) |
| `QINIU_AK` / `QINIU_SK` / `QINIU_BUCKET` / `QINIU_DOMAIN` / `QINIU_REGION` | 七牛对象存储(商品图上传,可选) | 整体留空=功能关闭;启用则五项全填 |

**七牛对象存储(商品图上传)**:管理端上传图片走「浏览器 → 后端中转 → 七牛 Kodo」,
数据库只存外链 URL(如 `https://static.ljl0628.cn/products/20260112/xxx.jpg`)。
启用步骤:七牛建空间(记下存储区域代码)→ 空间绑定加速域名并 CNAME → 把
`QINIU_*` 五项写入服务器 `.env` → `docker compose -f docker-compose.prod.yml up -d server`
重建生效。区域代码:`z0` 华东 / `z1` 华北 / `z2` 华南 / `cn-east-2` 华东-浙江2 /
`na0` 北美 / `as0` 东南亚。未配置时上传接口返回业务码 60014(前端明确提示),其余功能不受影响。

**与其他项目的隔离**:本项目独立 compose project(`fawn-mall`)、独立 Docker 网络
(default)、独立数据卷、容器名前缀 `fawn-`;CD 的 `up -d --remove-orphans` 只作用于
`/opt/fawn-mall` 目录,不会触碰 blog 项目的容器与卷。唯一的交点是 web 容器以
`external` 方式加入 `blog_default` 网络(只多一条 DNS 解析,不改变 blog 任何行为)。

应用参数(支付超时、连接池、日志保留等)如需调整:在 `server/config/config.example.yaml`
基础上建 `config.yaml` 挂载进容器,或追加 `APP_*` 环境变量(优先级最高)。
常用项:`APP_ORDER_PAY_TIMEOUT_SECONDS`、`APP_LOG_LEVEL`(支持**改配置文件热加载**)、
`APP_LOG_MAX_SIZE_MB`。

## 5. 数据备份与恢复

数据库在腾讯云 CDB,首选**云上自动备份**:

- 控制台「云数据库 MySQL → 备份」开启自动备份(每日全量,保留 7 天默认可调),
  恢复用控制台的「克隆/回档」,分钟级整机回滚,优先使用;

服务器侧手工备份(升级前快照一份,双重保险):

```bash
docker run --rm mysql:8.0 mysql -h${DB_HOST} -P${DB_PORT} -u${DB_USER} -p"${DB_PASSWORD}" \
  --single-transaction fawn_mall | gzip > /var/backups/fawn-mall-$(date +%F).sql.gz
```

RabbitMQ/Redis 数据可重建(队列拓扑、缓存由服务启动重建),一般无需备份。

## 6. 日志与监控

- **应用日志**:容器 `/app/logs/fawn-mall.log`(JSON,lumberjack 100MB 切割/14 天/10 份/gzip),
  已挂载 `fawn-server-logs` 卷,容器重建不丢;
  `docker exec fawn-server tail -f /app/logs/fawn-mall.log`;
  排障按 `request_id` 串联一次请求的 访问→业务→错误 日志;
- **级别热切换**:`docker exec fawn-server sh -c 'sed "s/level: info/level: debug/" config/config.yaml > /tmp/t && cat /tmp/t > config/config.yaml'`
  ——仅 log.level 即时生效,其余配置需重启(用 `cat >` 原地覆写,避免重命名文件导致监听失效);
- **健康检查**:compose healthchecks + `GET /healthz`;可接入腾讯云「云监控」拨测 443 域名;
- **资源观察**:`docker stats`;磁盘 `docker system df`;CDB 的 CPU/内存/连接数监控在
  腾讯云控制台自带,建议配告警(连接数 >80%、磁盘 >80%);
- **可选进阶**:腾讯云 CLS 日志服务采集 JSON 日志文件,或轻量方案 Dozzle(Web 面板)。

## 7. 故障排查速查

| 现象 | 排查 |
| --- | --- |
| CD build job 失败 | Actions 日志;多为 TCR 登录失败(密码过期/凭证无权限/Variable 未配置) |
| CD deploy job 失败 | 安全组 22 未放行、SSH_KEY 与服务器密钥对不匹配、服务器磁盘满 |
| 端口被同机其他项目占用 | `ss -lntp | grep 18090` 找占用者,在 .env 覆盖对应 *_PORT 再 `up -d` |
| server 反复重启 | `docker logs fawn-server`:配置校验失败会 zap fatal 指明字段(JWT_SECRET 过短、.env 缺项) |
| 登录/接口 50001 | 查 fawn-server 日志 error 级:DB/Redis/MQ 连接失败带 op+stack;DB 类先在服务器 `docker run --rm mysql:8.0 mysql -h$DB_HOST -u$DB_USER -p -e "SELECT 1"` 验证连通 |
| 域名 502 | `docker ps` 看 fawn-web/fawn-server 是否健康;`docker exec blog-nginx curl -sI http://fawn-web:80`(容器名解析与反代链路);blog-nginx 日志;DNS 解析 |
| 域名打不开(非 502) | mall.conf 是否已挂载进 blog-nginx(`docker exec blog-nginx ls /etc/nginx/conf.d/`);证书文件是否存在;`docker exec blog-nginx nginx -t` |
| MQ 队列堆积 | `ssh -L 25672:127.0.0.1:25672 user@host` 后浏览器开 http://localhost:25672 |
| 数据库连不上 | CDB 与服务器是否同 VPC;安全组/白名单是否放行;`.env` 口令;账号主机限制(是否仅允许某 IP 段) |
| 升级后起不来 | 回滚 TAG(§4);仍失败看 `docker logs` fatal 信息 |

## 8. 演进路径(体量增长后)

- **多实例**:`server` 无状态(登录态 JWT、共享数据在 Redis/MySQL),
  `docker compose up --scale server=2` + blog-nginx upstream 负载均衡;
- **托管中间件进阶**:Redis → 腾讯云 Redis,MQ → TDMQ RabbitMQ,服务器只跑无状态前后端
  (MySQL 已在 CDB,这一步完成后服务器零有状态服务);
- **K8s**:镜像产物不变,迁腾讯云 TKE;健康检查 `/healthz`,日志可经
  `APP_LOG_FORMAT` 改 stdout 由容器日志采集,或 Filebeat 采 `/app/logs`。
