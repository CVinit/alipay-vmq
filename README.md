# alipay-vmq

支付宝官方 API 中间件，替代 VMQ-Go 的手机监控端。通过支付宝官方接口（RSA2 签名）实现自动到账确认，对外兼容易支付（epay）协议，可直接对接 Dujiao-Next。

## 特性

- **支付宝官方 API** — 使用 `alipay.trade.page.pay`（PC）、`alipay.trade.wap.pay`（H5）、`alipay.trade.precreate`（当面付扫码）三种支付方式
- **自适应支付页** — PC 端展示动态二维码，手机端自动唤起支付宝 H5 支付
- **易支付协议兼容** — 实现 `/mapi.php` 和 `/submit.php`，Dujiao-Next 可直接对接
- **双重到账确认** — 支付宝异步通知为主，后台轮询查询为兜底，不丢单
- **双存储后端** — 支持 PostgreSQL 和 SQLite，按需切换
- **单文件部署** — Go embed 打包模板，编译为单个二进制
- **Docker 多架构** — 自动构建 `linux/amd64` + `linux/arm64` 镜像

## 工作流程

```
Dujiao-Next ──epay协议──▶ 中间件 /mapi.php
                              │
                    ┌─────────┴─────────┐
                    ▼                   ▼
            VMQ /createOrder      支付宝 API
            (获取尾数金额)        (创建交易)
                    │                   │
                    │         ┌─────────┘
                    │         ▼
                    │    用户完成支付
                    │         │
                    │         ▼
                    │    支付宝异步通知
                    │         │
                    ▼         ▼
            VMQ /appPush ◀── 中间件验签确认
                    │
                    ▼
            VMQ 通知 Dujiao-Next
```

## 快速开始

### 1. 准备配置

```bash
cp .env.example .env
```

必须修改的配置项：

```bash
ALIPAY_APP_ID=你的支付宝应用ID
ALIPAY_PRIVATE_KEY=你的RSA2应用私钥
ALIPAY_PUBLIC_KEY=支付宝公钥

VMQ_BASE_URL=http://vmq:8080
VMQ_KEY=VMQ商户密钥
VMQ_DEVICE_KEY=VMQ监控端密钥

EPAY_MERCHANT_ID=1000
EPAY_MERCHANT_KEY=至少32位随机字符串

PUBLIC_BASE_URL=https://pay.example.com
```

### 2. Docker Compose 部署（推荐）

完整部署（含共享 PostgreSQL）：

```bash
docker compose up -d
```

如果 PostgreSQL 已由其他项目管理（`shared-db-net` 网络已存在）：

```bash
docker compose -f docker-compose.standalone.yml up -d
```

详细部署教程见 [docs/DEPLOYMENT.md](docs/DEPLOYMENT.md)。

### 3. 本地开发

```bash
# 准备环境变量
export $(grep -v '^#' .env | xargs)

# 启动
go run ./cmd/alipay-vmq
```

### 4. 编译二进制

```bash
CGO_ENABLED=0 go build -o alipay-vmq ./cmd/alipay-vmq
```

## Nginx 反向代理配置

```nginx
server {
    listen 443 ssl http2;
    server_name pay.example.com;

    ssl_certificate     /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}

# HTTP redirect
server {
    listen 80;
    server_name pay.example.com;
    return 301 https://$host$request_uri;
}
```

如果前面还有 Cloudflare：

```nginx
# 在 http 块中恢复 Cloudflare 真实 IP
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
real_ip_header CF-Connecting-IP;
```

## Dujiao-Next 对接

在 Dujiao-Next 支付渠道配置中：

| 配置项 | 值 |
|--------|-----|
| `provider_type` | `epay` |
| `gateway_url` | `https://pay.example.com` |
| `epay_version` | `v1` |
| `merchant_id` | 与 `EPAY_MERCHANT_ID` 一致 |
| `merchant_key` | 与 `EPAY_MERCHANT_KEY` 一致 |
| `channel_type` | `alipay` |

## 配置项说明

| 环境变量 | 说明 | 默认值 |
|----------|------|--------|
| `ALIPAY_APP_ID` | 支付宝应用 ID | (必填) |
| `ALIPAY_PRIVATE_KEY` | RSA2 应用私钥内容 | (必填) |
| `ALIPAY_PRIVATE_KEY_PATH` | 私钥文件路径（优先于内容） | - |
| `ALIPAY_PUBLIC_KEY` | 支付宝公钥 | (必填) |
| `VMQ_BASE_URL` | VMQ 服务地址 | (必填) |
| `VMQ_KEY` | VMQ 商户密钥 | (必填) |
| `VMQ_DEVICE_KEY` | VMQ 监控端密钥 | (必填) |
| `EPAY_MERCHANT_ID` | 易支付商户 ID | (必填) |
| `EPAY_MERCHANT_KEY` | 易支付商户密钥 | (必填) |
| `PUBLIC_BASE_URL` | 公网访问地址 | (必填) |
| `DATABASE_DRIVER` | 数据库类型 | `sqlite` |
| `DATABASE_URL` | 数据库连接串/文件路径 | `alipay_vmq.db` |
| `LISTEN_ADDR` | 监听地址 | `:8081` |
| `VMQ_ORDER_TIMEOUT` | 订单超时（分钟） | `5` |
| `POLL_INTERVAL` | 轮询间隔（秒） | `30` |
| `CONFIG_FILE` | YAML 配置文件路径 | `config.yaml` |

## GitHub Actions

- **CI** — push/PR 时自动 build + vet + test
- **Docker Publish** — push 到 master 或打 tag 时构建多架构镜像到 `ghcr.io/cvinit/alipay-vmq`
- **Release** — 打 `v*` tag 时编译 4 平台二进制并创建 GitHub Release

发布新版本：

```bash
git tag v1.0.0
git push origin v1.0.0
```

## 许可证

MIT
