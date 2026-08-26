# singbox-manager 项目说明

> 与 mihomo-manager 同架构的重建（参考 ~/project/mihomo-manager/mihomo-manager 与老 ~/project/singbox-web-panel）。
> 命名约定：二进制 /usr/local/bin/sing-box 与用户/组 sing-box 保留官方连字符，其余统一 singbox。

## 架构（与 mihomo-manager 一致）

- 单一 Go 二进制：`singbox-manager serve`（守护）+ CLI 子命令（模式启停 / status / config sync）
- 守护进程：dbus 管理 singbox@ 单元、exec ip/nft 套规则（intercept 包，核心无关直接复用）、
  同生共死监控 + 启动 reconcile + tun0 事件清理、REST + SSE + 内嵌 Vue 前端
- 单一事实源：/opt/singbox-manager/manager.yaml（SINGBOX_MANAGER_CONFIG 可覆盖）

## 模式（5 种，server 配置由用户自管）

| 模式 | 单元 | 配置 | 说明 |
|---|---|---|---|
| tun | singbox@tun | config_tun.json | tun0，auto_route/auto_redirect，rule 9000 / table 2022 派生清理 |
| tproxy | singbox@tproxy | config_tproxy.json | tproxy 22026，fwmark 1 / table 100 / nft sing-box_tproxy4 |
| redir-tproxy | singbox@redir-tproxy | config_redir-tproxy.json | redirect 22025 + tproxy 22026，nft sing-box_redir_tproxy4 |
| socks | singbox@socks | config_socks.json | mixed 20080 |
| server | singbox@server | config_server.json | **用户自管**：不生成/不覆盖/不校验，只启停与监控 |

- 配置格式 JSON：config_generic.json（用户配置）+ 各模式 preset 入站（manager.yaml）→ 合并生成；
  **server 模式无 preset，config sync 跳过**，启动前检查配置文件存在（缺失报明确错误）
- 校验：`sing-box check -D <dataDir> -c tmp`（root 时 runuser -u sing-box 降权，20s 硬超时）
- 回环避免：exclude_gid（meta skgid）优先 / routing_mark（meta mark）备选，二选一

## 部署

- install / copy / upgrade / remove 见 deploy/deploy.sh
- 目标机免编译：打包 `singbox-manager deploy/` 上传，`deploy.sh copy`
- upgrade 从老 sing-box-web-panel 布局迁移：.conf → manager.yaml，清理 sing-box-ctl、/etc/sing-box、/opt/sing-box-panel
- 面板 :8082；CLI socket /run/singbox-manager/singbox-manager.sock（root）

## 验证状态（2026-08-24）

- go vet / 构建通过；非 root 冒烟：页面、API、CLI status、config sync（sing-box check 实测）、
  server 模式跳过生成 + 缺失配置报错，全部通过
- 待真实机器端到端：5 模式启停、规则同生共死、tun 清理、gid/mark 双回环实测
