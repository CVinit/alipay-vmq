# 部署指南

## 目录

- [架构概览](#架构概览)
- [前置条件](#前置条件)
- [方式一：完整部署（含 PostgreSQL）](#方式一完整部署含-postgresql)
- [方式二：接入已有 PostgreSQL](#方式二接入已有-postgresql)
- [Nginx 反向代理配置](#nginx-反向代理配置)
- [Dujiao-Next 对接配置](#dujiao-next-对接配置)
- [VMQ-Go 侧配置](#vmq-go-侧配置)
- [验证部署](#验证部署)
- [常见问题](#常见问题)

---

## 架构概览

```
                    ┌─────────────────────────────────────────────┐
                    │              Docker shared-db-net            │
                    │                                             │
 用户浏览器 ◀──────▶│  Nginx ──▶ alipay-vmq (:8081)              │
                    │                │         │                  │
 Dujiao-Next ──────▶│               │         │                  │
                    │               ▼         ▼                  │
 支付宝服务器 ─────▶│         VMQ (:8080)  shared-postgres (:5432)│
                    │                                             │
                    └─────────────────────────────────────────────┘
```

所有服务通过 `shared-db-net` Docker 网络互通，PostgreSQL 由一个容器统一管理多个数据库。

---

## 前置条件

- Docker 24+ 和 Docker Compose v2
- 一个公网域名（用于支付宝回调），已配置 DNS 解析
- 支付宝开放平台应用（网页/移动应用类型），已获取：
  - APP_ID
  - RSA2 应用私钥
  - 支付宝公钥
- VMQ-Go 已部署并运行
- （可选）Dujiao-Next 已部署

---

## 方式一：完整部署（含 PostgreSQL）

适用于首次部署，或希望由本项目统一管理 PostgreSQL 的场景。

### 1. 克隆项目

```bash
git clone https://github.com/CVinit/alipay-vmq.git
cd alipay-vmq
```

### 2. 配置环境变量

```bash
cp .env.example .env
```

编辑 `.env`，**必须修改**以下内容：

```bash
# PostgreSQL 超级用户密码
POSTGRES_SUPERPASS=你的强密码

# 各数据库密码
VMQ_DB_PASSWORD=vmq的数据库密码
DUJIAO_DB_PASSWORD=dujiao的数据库密码
ALIPAY_VMQ_DB_PASSWORD=alipay_vmq的数据库密码

# 支付宝配置
ALIPAY_APP_ID=2021000000000000
ALIPAY_PRIVATE_KEY=MIIEvQIBADANBgkqhki...（完整私钥内容，不含头尾标记）
ALIPAY_PUBLIC_KEY=MIIBIjANBgkqhki...（支付宝公钥内容）

# VMQ 连接
VMQ_BASE_URL=http://vmq:8080
VMQ_KEY=你在VMQ后台设置的商户通讯密钥
VMQ_DEVICE_KEY=你在VMQ后台设置的监控端密钥

# Epay 商户配置（Dujiao-Next 用这个对接）
EPAY_MERCHANT_ID=1000
EPAY_MERCHANT_KEY=生成一个至少32位的随机字符串

# 公网地址（支付宝回调用）
PUBLIC_BASE_URL=https://pay.example.com

# 数据库连接（与上面的密码保持一致）
DATABASE_DRIVER=postgres
DATABASE_URL=postgres://alipay_vmq:alipay_vmq的数据库密码@shared-postgres:5432/alipay_vmq?sslmode=disable
```

### 3. 启动服务

```bash
docker compose up -d
```

### 4. 查看日志

```bash
docker compose logs -f alipay-vmq
docker compose logs -f postgres
```

---

## 方式二：接入已有 PostgreSQL

如果 PostgreSQL 已经由其他 compose 项目（如 VMQ-Go）管理，且 `shared-db-net` 网络已存在。

### 1. 确认网络存在

```bash
docker network ls | grep shared-db-net
```

如果不存在，先启动管理 PostgreSQL 的那个 compose 项目。

### 2. 手动创建数据库（如果 initdb 脚本未执行过）

```bash
docker exec -it shared-postgres psql -U postgres -d postgres -c "
  CREATE DATABASE alipay_vmq;
  CREATE ROLE alipay_vmq WITH LOGIN PASSWORD '你的密码';
  GRANT ALL PRIVILEGES ON DATABASE alipay_vmq TO alipay_vmq;
  ALTER DATABASE alipay_vmq OWNER TO alipay_vmq;
"
```

### 3. 配置并启动

```bash
cp .env.example .env
# 编辑 .env，填入正确的配置（参考方式一）

# 使用 standalone 模式（不启动 postgres 容器）
docker compose -f docker-compose.standalone.yml up -d
```

### 4. 确认连通性

```bash
docker exec -it alipay-vmq wget -qO- http://shared-postgres:5432 2>&1 | head -1
# 应该能连通（即使返回错误内容，说明网络通）
```

---

## Nginx 反向代理配置

### 基础配置

创建 `/etc/nginx/sites-available/pay.example.com`：

```nginx
upstream alipay_vmq {
    server 127.0.0.1:8081;
    keepalive 16;
}

server {
    listen 443 ssl http2;
    server_name pay.example.com;

    ssl_certificate     /etc/letsencrypt/live/pay.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/pay.example.com/privkey.pem;
    ssl_protocols       TLSv1.2 TLSv1.3;
    ssl_ciphers         HIGH:!aNULL:!MD5;

    # 安全头
    add_header X-Content-Type-Options nosniff always;
    add_header X-Frame-Options DENY always;
    add_header Referrer-Policy strict-origin-when-cross-origin always;

    location / {
        proxy_pass http://alipay_vmq;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Connection "";

        # 支付宝回调超时设置
        proxy_connect_timeout 10s;
        proxy_read_timeout 30s;
        proxy_send_timeout 10s;
    }

    # 禁止爬虫
    location /robots.txt {
        return 200 "User-agent: *\nDisallow: /\n";
    }
}

# HTTP → HTTPS 重定向
server {
    listen 80;
    server_name pay.example.com;
    return 301 https://$host$request_uri;
}
```

### Cloudflare + Nginx 配置

如果域名经过 Cloudflare CDN：

```nginx
# 放在 http {} 块中
# Cloudflare IP ranges (定期更新: https://www.cloudflare.com/ips/)
set_real_ip_from 173.245.48.0/20;
set_real_ip_from 103.21.244.0/22;
set_real_ip_from 103.22.200.0/22;
set_real_ip_from 103.31.4.0/22;
set_real_ip_from 141.101.64.0/18;
set_real_ip_from 108.162.192.0/18;
set_real_ip_from 190.93.240.0/20;
set_real_ip_from 188.114.96.0/20;
set_real_ip_from 197.234.240.0/22;
set_real_ip_from 198.41.128.0/17;
set_real_ip_from 162.158.0.0/15;
set_real_ip_from 104.16.0.0/13;
set_real_ip_from 104.24.0.0/14;
set_real_ip_from 172.64.0.0/13;
set_real_ip_from 131.0.72.0/22;
# IPv6
set_real_ip_from 2400:cb00::/32;
set_real_ip_from 2606:4700::/32;
set_real_ip_from 2803:f800::/32;
set_real_ip_from 2405:b500::/32;
set_real_ip_from 2405:8100::/32;
set_real_ip_from 2a06:98c0::/29;
set_real_ip_from 2c0f:f248::/32;

real_ip_header CF-Connecting-IP;
```

**重要**：支付宝异步通知（notify_url）必须能被支付宝服务器直接访问。如果 Cloudflare 开启了"Under Attack"模式或 JS Challenge，需要为支付宝回调路径创建 Page Rule 或 WAF 规则放行：

- URL 匹配: `pay.example.com/notify`
- 安全级别: 关闭（Essentially Off）
- Browser Integrity Check: 关闭

### 启用配置

```bash
ln -s /etc/nginx/sites-available/pay.example.com /etc/nginx/sites-enabled/
nginx -t && systemctl reload nginx
```

---

## Dujiao-Next 对接配置

在 Dujiao-Next 后台 → 支付渠道 → 添加渠道：

| 配置项 | 值 | 说明 |
|--------|-----|------|
| 支付接口 | `epay` | 易支付协议 |
| 接口版本 | `v1` | epay_version |
| 网关地址 | `https://pay.example.com` | 中间件公网地址 |
| 商户ID | `1000` | 与 EPAY_MERCHANT_ID 一致 |
| 商户密钥 | `你设置的密钥` | 与 EPAY_MERCHANT_KEY 一致 |
| 支付方式 | `alipay` | channel_type |

---

## VMQ-Go 侧配置

中间件需要 VMQ 的两个密钥：

### 获取 VMQ_KEY（商户通讯密钥）

1. 登录 VMQ 后台
2. 进入系统设置
3. 找到"通讯密钥"（key），复制到 `.env` 的 `VMQ_KEY`

### 获取 VMQ_DEVICE_KEY（监控端密钥）

1. 登录 VMQ 后台
2. 进入系统设置
3. 找到"监控端密钥"（deviceKey），复制到 `.env` 的 `VMQ_DEVICE_KEY`

### VMQ 网络连通

确保 alipay-vmq 容器能访问 VMQ 服务。如果 VMQ 也在 `shared-db-net` 网络中：

```bash
# .env 中设置
VMQ_BASE_URL=http://vmq:8080
```

如果 VMQ 在不同网络，需要让两个容器加入同一网络，或使用宿主机 IP：

```bash
VMQ_BASE_URL=http://host.docker.internal:8080
```

---

## 验证部署

### 1. 检查服务状态

```bash
docker compose ps
docker compose logs alipay-vmq | tail -20
```

正常启动应看到：
```json
{"level":"INFO","msg":"server starting","addr":":8081"}
```

### 2. 测试 epay 接口

```bash
# 生成签名（替换为你的 EPAY_MERCHANT_KEY）
KEY="your_epay_merchant_key"
SIGN=$(echo -n "money=0.01&name=test&notify_url=http://example.com/notify&out_trade_no=TEST001&pid=1000&type=alipay${KEY}" | md5sum | cut -d' ' -f1)

curl -X POST https://pay.example.com/mapi.php \
  -d "pid=1000&type=alipay&out_trade_no=TEST001&notify_url=http://example.com/notify&name=test&money=0.01&sign=${SIGN}&sign_type=MD5"
```

成功应返回：
```json
{"code":1,"msg":"success","payurl":"https://pay.example.com/pay?order_id=...&token=..."}
```

### 3. 测试支付页面

用浏览器打开返回的 `payurl`，应看到支付页面（二维码或支付宝跳转按钮）。

---

## 常见问题

### Q: 支付宝回调收不到？

1. 确认 `PUBLIC_BASE_URL` 是公网可访问的 HTTPS 地址
2. 确认 Nginx 正确代理了 `/notify` 路径
3. 确认 Cloudflare 没有拦截支付宝服务器的请求
4. 检查日志: `docker compose logs alipay-vmq | grep notify`

### Q: VMQ createOrder 失败？

1. 确认 `VMQ_BASE_URL` 在容器内可达: `docker exec alipay-vmq wget -qO- http://vmq:8080/getState`
2. 确认 `VMQ_KEY` 与 VMQ 后台设置一致
3. 确认 VMQ 有可用的支付宝收款码（至少上传一个）

### Q: 数据库连接失败？

1. 确认 PostgreSQL 容器健康: `docker compose ps`
2. 确认 `DATABASE_URL` 中的用户名密码与 initdb 脚本一致
3. 手动测试连接: `docker exec shared-postgres psql -U alipay_vmq -d alipay_vmq -c "SELECT 1"`

### Q: 如何查看订单状态？

```bash
docker exec shared-postgres psql -U alipay_vmq -d alipay_vmq -c "SELECT id, status, amount, created_at FROM orders ORDER BY created_at DESC LIMIT 10"
```

### Q: 如何更新版本？

```bash
git pull
docker compose up -d --build
```

或使用 GHCR 镜像：

```bash
docker pull ghcr.io/cvinit/alipay-vmq:latest
docker compose -f docker-compose.standalone.yml up -d
```
