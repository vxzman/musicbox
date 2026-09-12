# 开发日志

> 记录本次（2026-09-02）变更内容、开发注意点与经验总结，供后续开发参考。

## 一、本次更改了什么

### 1. 品牌视觉：sing-box logo + favicon

- 新增 `web/public/favicon.svg` 与 `favicon-dark.svg`：按参考图（`~/Documents/clash和mihomo的logo.html`）用 SVG 代码重绘 sing-box 立体开口六边形盒子（外轮廓 + 顶面/左右侧面 + 高光 + 开口暗面），配色对齐 MD3 紫色系（seed `#6750A4`）。
- `web/index.html` 用 `media="(prefers-color-scheme: ...)"` 双 `<link rel="icon">` 随浏览器深浅色主题切换。
- 导航栏 logo-mark 由 32px 单色 cube 图标换成 **42px 彩色重绘版**（内联 SVG，与 favicon 同款几何）。
- 图标进 vite `public/`：构建时自动拷入 `dist/` 根，Go 端 `//go:embed all:web/dist` 直接内嵌，无需改服务端。

### 2. 页面整体缩放 + 四周留白

- `App.vue` 新增 `.app-scale` 缩放外壳包裹「导航 + 主内容」；氛围背景、FAB、snackbar、底部导航留在外壳外。
- 用 **`zoom: 0.94` + `width: calc(100% * var(--app-zoom))` + `margin: 0 auto`** 实现缩小居中；缩放因子集中在 `.app` 的 `--app-zoom`（改一处即可）。
- FAB/snackbar 单独同步 zoom 保持比例；`@supports (zoom: 1)` 兜底（不支持时回退 100% 布局）。

### 3. 新增 EP / EBPF 两种模式

- **EP**（`sing-box@ep` / `config_ep.json`）：在 `config_generic.json` 基础上插入**顶层 `endpoints` 模块数组**（与 `dns`/`inbounds` 平行，按 tag 合并），例如 wireguard 端点。
- **EBPF**（`sing-box@ebpf` / `config_ebpf.json`）：插入 **ebpf 入站模块**（复用 preset → inbounds 合并机制），配置内容由用户填写（仅测试版 sing-box 支持）。
- `Mode` 新增 `Endpoints` 字段 + `SelfManaged()` 判断（preset 与 endpoints 均为空即用户自管）。
- `fillDefaults` 自动补齐老 `manager.yaml` 缺失的内置模式（升级后面板直接出现 ep/ebpf）。
- 生成合并逻辑泛化为 `mergeByTag`（inbounds / endpoints 共用）。

### 4. 表单与配置编辑改造

- **socks**：系统设置页从 preset JSON 文本域改为**单个端口输入框**，保存时前端改写 mixed-in 的 `listen_port` 后提交 preset（后端零改动）。
- **ep / ebpf**：系统设置页各一个模块 JSON 文本域（带占位示例），保存后端校验并自动重新生成对应 `config_*.json`。
- **server**：配置管理页 `config_server.json` 由只读改为**可直接编辑保存**（新增 `PUT /api/config/mode/{mode}`，sing-box check 校验后写入）；生成型模式拒绝直接写入。

### 5. 状态页交互

- 启停双按钮改为**胶囊开关**：圆点在左=停止（灰色），滑到右侧=启动（点亮，取该模式主题色 + 光晕）。
- 「已停止」状态芯片不再显示（开关位置已表达启停状态）；运行中/启动中/失败/未知仍显示。
- 状态/规则芯片加固定最小宽度居中，字数不同（不适用/规则缺失/无残留…）时上下居中对齐。
- 三个页面头部说明文案删除；模式配置细节移入 README 部署教程「模式配置说明」。

### 6. UI 精简（删除冗余控件）

按「每个信息只出现一次」原则清理：

- **FAB 刷新按钮**：顶栏导航胶囊里已有常驻刷新按钮，FAB 是重复入口——删除按钮及其全部 CSS（含 `.view-status .snackbar` 的避让偏移、移动端规则、死掉的 `.btn-*` ripple 配色类）。
- **Hero 右侧状态芯片**（运行中/待机/连接中）：顶栏芯片与 Hero 大标题已表达同一状态，三处重复——删除。
- **统计条（4 个 tiles）**：模式总数/运行中/规则就位/失败全部能从下方模式列表直接读出（行数、绿色芯片、规则芯片、红色芯片），且互斥模式下「运行中」恒为 0/1——整行删除。

经验：删控件时要连带删掉只为它存在的 CSS（避让偏移、响应式规则、ripple 配色），否则样式表里会积累「为已删除控件服务」的死代码。

## 二、开发要注意的点

### 页面缩放：为什么用 zoom 而不是 transform（重点）

实测（headless Chrome）结论，以后不要再踩：

| 方案 | sticky 导航 | fixed 浮层 | 结论 |
|---|---|---|---|
| `transform: scale` 在外壳上 | **随滚动漂移出屏幕**（变换原点锚在元素盒上，视觉位置 = 0.94·layout − 0.06·scrollY） | 变成相对外壳定位，钉不到视口 | ✗ 不可用 |
| `zoom: z` + `width: calc(100% * z)` | 正常吸附（16px → 15px） | 保持视口定位 | ✓ 采用 |

- `zoom` 是布局生效属性：文档高度真实缩小（无底部死白）、sticky bottom 的保存栏不会随滚动上漂。
- fixed 浮层若在缩放外壳**之外**，需单独 `zoom` 否则与页面比例不一致。
- transform 方案里 sticky 会「吸住但视觉漂移」，首次目测可能误以为正常——一定要滚动后验证。

### 后端

- `modeUpdate.Endpoints` 用 `*string` 而非 `string`：指针才能表达「清空」意图（`""` 合法，表示回到用户自管），`omitempty` 的 string 无法区分未提交与清空。
- `Validate` 只校验非空值；EP/EBPF 留空必须合法（自管模式）。
- `SyncAll` 跳过条件用 `md.SelfManaged()`；`StartMode` 对自管模式先查配置文件存在，报错文案要提示「该模式配置由用户自管」。
- **sing-box check 会对所有生成配置全量校验**：ebpf 入站在非测试版 sing-box 上会校验失败导致整次同步失败——部署环境必须用支持 ebpf 的测试版；开发机无 sing-box 时自动跳过（`LookPath` 失败即返回 nil）。
- 设置保存（`Save`）会把 `manager.yaml` 整文件 `yaml.Marshal` 重排为 **4 空格缩进**并丢弃注释——冒烟测试后要还原 fixture，避免 git 大块缩进 churn。
- 前端产物内嵌于二进制：改前端必须 `npm run build` 后再 `go build`，否则面板还是旧界面；`./musicbox info` 可核对「内嵌前端：已构建」。

### 前端

- vite `public/` 目录的文件原样进 `dist/` 根，dev 与 build 都能以 `/favicon.svg` 访问。
- favicon 深浅色切换靠 `media` 属性（不支持的老浏览器回退用第一个 light 图标）。
- 胶囊开关的 knob 位移 = 宽度 − 2×padding − knob 直径，改尺寸时记得同步 `translateX`。
- 删除 UI 说明文案前先确认内容是否要落入 README（用户偏好：界面干净，细节进文档）。

## 三、经验和总结

1. **CSS 行为用 headless Chrome 实测再定方案**：本次「transform vs zoom」之争靠一段带 fixed/sticky 探针的测试页 + `--dump-dom` 输出 getBoundingClientRect 数据定案，比查文档、猜行为可靠得多。截图在无头环境读不回时，注入测量脚本输出数值是有效替代。
2. **通用合并器一步到位**：inbounds/endpoints 的「按 tag 覆盖、缺则追加」抽象成 `mergeByTag` 后，新模块类型零成本接入，避免复制粘贴两套几乎相同的合并代码。
3. **空值语义要显式**：`SelfManaged()` 把「未配置 = 用户自管」的隐式规则收敛成一个方法，lifecycle / api / 前端三处共用同一判断，改规则只动一处。
4. **冒烟测试要善后**：用 dev fixture 起守护进程 + curl 打 API 是最快的前后端联调方式，但守护进程会落盘（重排 manager.yaml、生成 config_*.json）——测试后 `git show HEAD:dev/manager.yaml` 还原并删除新生成的配置文件。
5. 小坑记录：`pkill -f` 的 pattern 会匹配到包含该字符串的**自身命令行**导致 shell 自杀（exit 144）；用 `pkill -f '[h]ttp.server'` 括号技巧规避。
6. **交互直觉优先**：双按钮 → 胶囊开关、JSON 文本域 → 端口输入框，都是「状态可视化 + 单一操作点」的方向，和本项目玻璃 MD3 的「一个控件只做一件事」一致。
