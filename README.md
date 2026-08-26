# singbox-manager

sing-box 运行模式管理器：**单一 Go 二进制**（守护进程 + CLI），Vue 3 前端内嵌，透明代理规则由守护进程编排并与 `singbox@` 实例同生共死。

> 命名约定：内核二进制 `/usr/local/bin/sing-box` 与用户/组 `sing-box` 保留官方连字符，其余（项目名、目录、单元、配置）统一 `singbox`。

## ✨ 界面

前端遵循 [Google MD3 + Glass 设计美学](<Google MD3 + Glass（玻璃氛围版）设计美学.md>)：紫罗兰 `#6750A4` 种子色、三颗光斑氛围层、三层玻璃体系（导航胶囊 / 玻璃卡片 / 涟漪按钮），"Material You — but make it breathe."

- **运行状态**：模式摘要 Hero、统计条、模式启停列表（单元状态 + 规则状态芯片，SSE 秒级刷新）
- **配置管理**：双栏编辑器，`config_generic.json` 唯一编辑入口，各模式配置只读预览
- **系统设置**：分段选项卡，管理 manager.yaml（规则数字 / 环境变量 / 预定义入站）

## 特性

- **5 种模式**：tun / tproxy / redir-tproxy / socks / server（server 配置文件由用户自管，管理器不生成不覆盖）
- **规则生命周期**：启动 `singbox@` 实例 → 延迟套用 nft/ip 规则 → 实例停止即清理（同生共死）；守护重启自动 reconcile 兜底；tun0 消失自动清理残留
- **回环避免双方式**：`meta skgid`（GID，优先）或 `meta mark`（路由 mark），二选一可配置
- **统一配置**：`/opt/singbox-manager/manager.yaml` 是规则数字 / env / 预定义入站的唯一事实源
- **实时状态**：dbus 订阅 + SSE 推送，面板状态秒级刷新
- **配置同步**：`config_generic.json` + 各模式预定义入站 → `sing-box check` 校验后原子写入

## 架构

```
singbox-manager                单一二进制
├── serve                      守护进程（systemd 服务入口）
│   ├── dbus 管理 singbox@ 单元
│   ├── exec ip/nft 套规则（同生共死编排）
│   ├── REST + SSE（/api、/api/events）
│   └── 内嵌 Vue 3 前端（web/dist，go:embed）
└── <mode> start|stop|status   模式启停与状态查询
```

| 模式 | 单元 | 配置 | 说明 |
|---|---|---|---|
| tun | singbox@tun | config_tun.json | tun0，auto_route/auto_redirect，rule 9000 / table 2022 派生清理 |
| tproxy | singbox@tproxy | config_tproxy.json | tproxy 22026，fwmark 1 / table 100 / nft sing-box_tproxy4 |
| redir-tproxy | singbox@redir-tproxy | config_redir-tproxy.json | redirect 22025 + tproxy 22026，nft sing-box_redir_tproxy4 |
| socks | singbox@socks | config_socks.json | mixed 20080 |
| server | singbox@server | config_server.json | **用户自管**：不生成/不覆盖/不校验，只启停与监控 |

## 项目结构

```
.
├── main.go                 CLI + 守护进程入口
├── internal/               Go 实现（dbus 编排 / nft、ip 规则 / API + SSE）
├── web/                    Vue 3 前端（@material/web 组件 + MD3 Glass 样式）
│   ├── src/                源码（App 外壳 + 三个视图）
│   └── dist/               构建产物（占位页入库，真实产物本地 npm run build）
├── deploy/                 部署物料：deploy.sh + systemd 单元 + 配置模板
├── dev/                    本地开发夹具（dev/run.sh 启动，不碰系统路径）
└── Google MD3 + Glass（玻璃氛围版）设计美学.md   前端设计规范
```

## 本地构建

```bash
# 前端（构建产物 dist/ 由后端 go:embed 内嵌）
cd web && npm install && npm run build
# 后端
cd .. && go build -ldflags="-s -w" -o singbox-manager .
```

> `web/dist/` 中保留了占位 `index.html`，未构建前端时 `go build` 也能通过（二进制将显示"前端尚未构建"提示页）。

## 部署到服务器（免编译，上传哪些文件）

前置：目标机已安装 sing-box 内核 `sudo cp sing-box /usr/local/bin/sing-box`。

**首次部署上传 4 个文件**：

| 文件 | 目标位置 |
|---|---|
| `singbox-manager`（已编译二进制，前端已内嵌） | `/usr/local/bin/singbox-manager` |
| `deploy/singbox@.service` | `/etc/systemd/system/` |
| `deploy/singbox-manager.service` | `/etc/systemd/system/` |
| `deploy/etc-singbox/config_generic.json`（模板配置） | `/etc/singbox/` |

```bash
# 本机打包
tar czf /tmp/sb-dist.tar.gz singbox-manager deploy/

# 上传并安装
scp /tmp/sb-dist.tar.gz 服务器:/tmp/
ssh 服务器
cd /tmp && tar xzf sb-dist.tar.gz        # 解出 /tmp/singbox-manager + /tmp/deploy/
sudo /tmp/deploy/deploy.sh copy          # 装二进制+单元+建用户+启动守护
# 老 sing-box-web-panel 机器改用: sudo /tmp/deploy/deploy.sh upgrade（自动迁移 .conf）
```

手动安装等效命令：

```bash
sudo install -m 0755 /tmp/singbox-manager /usr/local/bin/singbox-manager
sudo cp /tmp/deploy/singbox@.service /tmp/deploy/singbox-manager.service /etc/systemd/system/
sudo mkdir -p /etc/singbox && sudo cp /tmp/deploy/etc-singbox/config_generic.json /etc/singbox/
sudo useradd --system --shell /usr/sbin/nologin --home-dir /var/lib/singbox --no-create-home sing-box
sudo mkdir -p /var/lib/singbox /var/log/singbox && sudo chown sing-box:sing-box /var/lib/singbox /var/log/singbox
sudo systemctl daemon-reload && sudo systemctl enable --now singbox-manager
```

**日常更新只传 1 个文件**：

```bash
scp singbox-manager 服务器:/tmp/
ssh 服务器 'sudo install -m 0755 /tmp/singbox-manager /usr/local/bin/singbox-manager && sudo systemctl restart singbox-manager'
```

安装后检查：`/opt/singbox-manager/manager.yaml` 里 tproxy/redir-tproxy 的 `exclude_gid` 与 `id -g sing-box` 一致（不一致会环路），可在面板「系统设置」修改。

## 监听地址与 IPv6（安全说明）

面板无鉴权，**默认仅监听 IPv4**（`0.0.0.0:8082`，不暴露 IPv6）。在 manager.yaml 中调整：

- 仅本机访问：`web_addr: "127.0.0.1:8082"`
- 显式启用 IPv6：`web_addr: "[::]:8082"`（仅 IPv6）或 `":8082"`（双栈，含 IPv4）

修改后 `sudo systemctl restart singbox-manager` 生效（也可在面板「系统设置」修改）。

## nginx 反向代理配置示例

面板（`http://<host>:8082`）无内置鉴权，公网暴露建议经 nginx 反代并加 Basic Auth：

```nginx
server {
    listen 80;
    server_name singbox.example.com;

    # 可选：面板无鉴权，建议开启 Basic Auth（先 htpasswd -c /etc/nginx/.htpasswd 用户名）
    # auth_basic "Singbox Manager";
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
sudo certbot --nginx -d singbox.example.com
```

## 常用命令

```bash
sudo singbox-manager tun start            # 启动模式（tun|tproxy|redir-tproxy|socks|server）
sudo singbox-manager tproxy stop          # 停止（规则自动清理）
sudo singbox-manager status               # 各模式/单元/规则状态
sudo singbox-manager config sync          # 重新生成各模式配置（sing-box check 校验）
# server 模式：自行放置 /etc/singbox/config_server.json 后即可启停
```

## 本地开发

```bash
./dev/run.sh                # 守护进程（dev fixture，不碰系统路径），面板 :8082
cd web && npm run dev       # 前端热更新（vite 代理到 :8082）
```

## License

[MIT](LICENSE) © 2026 vxzman
