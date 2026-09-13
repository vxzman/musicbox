# MusicBox

**Linux 服务器上代理内核的透明代理模式编排与切换器。**

在 Linux 服务器直接运行 sing-box 等代理内核（无面板、无 GUI）时，用它一键切换 tun / tproxy / redir-tproxy / socks 等透明代理模式：配套的 nftables / ip 路由规则由守护进程自动编排，并与后台 systemd 实例**同生共死**（启动即套规则、停止即清理）。

实现为**单一 Go 二进制**（守护进程 + CLI），Vue 3 前端内嵌，部署只需分发一个文件。

## ✨ 界面

前端遵循 [Google MD3 + Glass 设计美学](google-md3-glass-design.md)：紫罗兰 `#6750A4` 种子色、三颗光斑氛围层、三层玻璃体系（导航胶囊 / 玻璃卡片 / 涟漪按钮），"Material You — but make it breathe."

- **运行状态**：模式摘要 Hero、统计条、模式启停列表（单元状态 + 规则状态芯片，SSE 秒级刷新）
- **配置管理**：双栏编辑器，`config_generic.json` 是编辑入口，各模式配置只读预览
- **系统设置**：分段选项卡，管理 manager.yaml（规则数字 / 环境变量 / 预定义入站）

## 特性

- **4 种模式**：tun / tproxy / redir-tproxy / socks；原生环境下若 systemd 上已有 `sing-box@<mode>` 在跑，首页直接显示该模式为运行中
- **规则生命周期**：启动服务实例 → 延迟套用 nft/ip 规则 → 实例停止即清理（同生共死）；守护重启自动 reconcile 兜底；tun0 消失自动清理残留
- **回环避免双方式**：`meta skgid`（GID，优先）或 `meta mark`（路由 mark），二选一可配置
- **统一配置**：`/opt/musicbox/manager.yaml` 是规则数字 / env / 预定义入站的唯一事实源
- **实时状态**：dbus 订阅 + SSE 推送，面板状态秒级刷新
- **配置同步**：`config_generic.json` + 各模式预定义入站 → 配置校验后原子写入

## 架构

```
musicbox                       单一二进制
├── serve                      守护进程（systemd 或容器进程后端）
│   ├── 管理 sing-box@<mode> 实例
│   ├── exec ip/nft 套规则（同生共死编排）
│   ├── REST + SSE（/api、/api/events）
│   └── 内嵌 Vue 3 前端（web/dist，go:embed）
└── <mode> start|stop|status   模式启停与状态查询
```

| 模式 | 单元 | 配置 | 说明 |
|---|---|---|---|
| tun | sing-box@tun | config_tun.json | tun0，auto_route/auto_redirect，rule 9000 / table 2022 派生清理 |
| tproxy | sing-box@tproxy | config_tproxy.json | tproxy 22026，fwmark 1 / table 100 / nft sing-box_tproxy4 |
| redir-tproxy | sing-box@redir-tproxy | config_redir-tproxy.json | redirect 22025 + tproxy 22026，nft sing-box_redir_tproxy4 |
| socks | sing-box@socks | config_socks.json | mixed 20080 |

## 项目结构

```
.
├── main.go                 CLI + 守护进程入口
├── internal/               Go 实现（systemd 或容器进程后端 / nft、ip 规则 / API + SSE）
├── web/                    Vue 3 前端（@material/web 组件 + MD3 Glass 样式）
│   ├── src/                源码（App 外壳 + 三个视图）
│   └── dist/               构建产物（占位页入库，真实产物本地 npm run build）
├── deploy.sh               一键打包、部署与卸载管理脚本
├── deploy/                 Linux 目录映射物料（etc, opt, usr, var）
├── container/              容器入口脚本与默认配置
├── Dockerfile              Rocky Linux 9 容器镜像
├── docker-compose.yml      本地容器运行配置
├── build.sh                原生二进制 / 容器镜像构建
└── google-md3-glass-design.md 前端设计规范
```

## 从源码构建（git clone 之后）

前置：**Go ≥ 1.22**（必须）、**Node.js ≥ 18 + npm**（仅构建前端需要；仓库自带占位页，不构建前端也能编译出可用的后端二进制）。

```bash
git clone git@github.com:vxzman/musicbox.git
cd musicbox

# 1. 构建前端（产物 web/dist 由后端 go:embed 内嵌）
cd web && npm install && npm run build

# 2. 编译后端（注入版本/编译时间/Git 提交，供 info 子命令展示）
cd .. && go build -ldflags="-s -w \
  -X main.version=$(git describe --tags --exact-match 2>/dev/null || echo dev) \
  -X main.buildTime=$(date -u '+%Y-%m-%dT%H:%M:%SZ') \
  -X main.gitCommit=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)" -o musicbox .
```

产物为根目录下单一二进制 `musicbox`，可用 `./musicbox info` 查看版本与内嵌前端状态。

> `web/dist/` 中保留了占位 `index.html`，未构建前端时 `go build` 也能通过（二进制将显示"前端尚未构建"提示页）。
> 目标机装有 Go 时也可直接 `sudo ./deploy.sh --install`，它会自动完成上述构建与部署。

## 部署到服务器（免编译，上传哪些文件）

前置：目标机已安装内核 `sudo cp sing-box /usr/local/bin/sing-box`。

**首次部署上传 4 个文件**：

| 文件（deploy/ 映射目录） | 目标位置 |
|---|---|
| `deploy/usr/local/bin/musicbox`（已编译二进制，前端已内嵌） | `/usr/local/bin/musicbox` |
| `deploy/etc/systemd/system/sing-box@.service` | `/etc/systemd/system/sing-box@.service` |
| `deploy/etc/systemd/system/musicbox.service` | `/etc/systemd/system/musicbox.service` |
| `deploy/etc/sing-box/config_generic.json`（模板配置） | `/etc/sing-box/config_generic.json` |
| `deploy/opt/musicbox/manager.yaml`（系统设置） | `/opt/musicbox/manager.yaml` |

```bash
# 1. 本机一键打包（自动附加时间戳，产物为 musicbox-deploy-<version>-<timestamp>.tar.gz）
./deploy.sh --pack

# 2. 上传安装包至服务器
scp musicbox-deploy-*.tar.gz 服务器:/tmp/

# 3. 在服务器解压并一键部署（含创建用户、目录权限、二进制与服务安装、开机自启）
ssh 服务器
cd /tmp && tar xzf musicbox-deploy-*.tar.gz
sudo ./deploy.sh --install
# 或直接指定 --file 部署: sudo ./deploy.sh --install --file /tmp/musicbox-deploy-*.tar.gz

# 卸载清理（如需完全移除服务器文件与服务）:
# sudo ./deploy.sh --remove
```

手动安装等效命令：

```bash
sudo install -m 0755 /tmp/musicbox /usr/local/bin/musicbox
sudo cp /tmp/deploy/sing-box@.service /tmp/deploy/musicbox.service /etc/systemd/system/
sudo mkdir -p /etc/sing-box && sudo cp /tmp/deploy/etc-sing-box/config_generic.json /etc/sing-box/
sudo useradd --system --shell /usr/sbin/nologin --home-dir /var/lib/sing-box --no-create-home sing-box
sudo mkdir -p /var/lib/sing-box /var/log/sing-box && sudo chown sing-box:sing-box /var/lib/sing-box /var/log/sing-box
sudo systemctl daemon-reload && sudo systemctl enable --now musicbox
```

**日常更新只传 1 个文件**：

```bash
scp musicbox 服务器:/tmp/
ssh 服务器 'sudo install -m 0755 /tmp/musicbox /usr/local/bin/musicbox && sudo systemctl restart musicbox'
```

安装后检查：`/opt/musicbox/manager.yaml` 里 tproxy/redir-tproxy 的 `exclude_gid` 与 `id -g sing-box` 一致（不一致会环路），可在面板「系统设置」修改。

### 模式配置说明（面板「系统设置」→ 各模式标签页）

- **socks**：只填「入站端口」，保存后写入 `config_socks.json` 的 mixed 入站（mixed-in）`listen_port`，其余字段由预定义入站模板维护。

自管模式（preset 与 endpoints 均为空）不参与配置生成：管理器不生成、不覆盖，启停前需先放置好配置文件。

## 监听地址与 IPv6（安全说明）

面板无鉴权，**默认仅监听 IPv4**（`0.0.0.0:8082`，不暴露 IPv6）。在 manager.yaml 中调整：

- 仅本机访问：`web_addr: "127.0.0.1:8082"`
- 显式启用 IPv6：`web_addr: "[::]:8082"`（仅 IPv6）或 `":8082"`（双栈，含 IPv4）

修改后 `sudo systemctl restart musicbox` 生效（也可在面板「系统设置」修改）。

## nginx 反向代理配置示例

面板（`http://<host>:8082`）无内置鉴权，公网暴露建议经 nginx 反代并加 Basic Auth：

```nginx
server {
    listen 80;
    server_name musicbox.example.com;

    # 可选：面板无鉴权，建议开启 Basic Auth（先 htpasswd -c /etc/nginx/.htpasswd 用户名）
    # auth_basic "MusicBox";
    # auth_basic_user_file /etc/nginx/.htpasswd;

    location / {
        proxy_pass http://127.0.0.1:8082;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # SSE 实时状态推送必须：关闭缓冲、放宽读超时
        proxy_buffering off;
        proxy_cache off;
        proxy_read_timeout 1h;
        proxy_send_timeout 1h;
        proxy_set_header Connection '';
    }
}
```

HTTPS 用 certbot 免费证书：

```bash
sudo apt install certbot python3-certbot-nginx
sudo certbot --nginx -d musicbox.example.com
```

## 常用命令

```bash
sudo musicbox info                 # 版本/编译时间/Git 提交/平台/内嵌前端（鉴别是否最新构建）
sudo musicbox tun start            # 启动模式（tun|tproxy|redir-tproxy|socks）
sudo musicbox tproxy stop          # 停止（规则自动清理）
sudo musicbox status               # 各模式/单元/规则状态
sudo musicbox config sync          # 重新生成各模式配置（配置校验）
```

## 容器运行与部署

容器版用 `go build -tags container` 编译独立进程后端，镜像基于 **Rocky Linux 9**，无需 systemd / D-Bus，由 MusicBox 直接管理 `sing-box` 子进程和 nft/ip 规则。

构建前把目标架构的 `sing-box` 二进制放到 `build/sing-box`（或设 `SINGBOX_BIN`）：

```bash
mkdir -p build
cp /path/to/sing-box build/sing-box
chmod 0755 build/sing-box

./build.sh binary             # build/musicbox + build/musicbox-container
./build.sh container          # 两个二进制 + Rocky 镜像 + tar.gz
./build.sh all                # 同 container
```

> **容器回环避免必须用 routing_mark。** 容器内以 root 跑 sing-box，没有专有 GID，不能用 `meta skgid`。默认镜像已在 `config_generic.json` 写入 `route.default_mark: 6666`，并在 `manager.yaml` 把 TPROXY / REDIR-TPROXY 设为 `routing_mark: 6666`。若改回 GID 方式会导致流量回环。

### Docker Compose（host 网络）

```bash
docker compose up -d
docker compose logs -f
```

透明代理需要操作宿主机 nftables / 策略路由，因此使用 `network_mode: host` 和 `privileged: true`。

### docker run

```bash
docker run -d \
  --name musicbox \
  --restart unless-stopped \
  --network host \
  --privileged \
  -v /opt/musicbox/config:/etc/sing-box \
  -v /opt/musicbox/data:/var/lib/sing-box \
  -v /opt/musicbox/manager:/opt/musicbox \
  localhost/musicbox:latest
```

面板默认 `http://<host>:8082`。

## 本地开发与测试
 
```bash
# 启动守护进程（指定 deploy 映射配置，面板 :8082）
MUSICBOX_CONFIG=$PWD/deploy/opt/musicbox/manager.yaml go run . serve

# 前端开发热更新（另开终端，vite 代理到 :8082）
cd web && npm run dev
```

## License

[MIT](LICENSE) © 2026 vxzman
