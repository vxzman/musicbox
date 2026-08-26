<script setup lang="ts">
import { onMounted, ref } from 'vue'
import '@material/web/button/filled-button.js'
import '@material/web/radio/radio.js'
import '@material/web/textfield/outlined-text-field.js'
import '@material/web/labs/segmentedbutton/outlined-segmented-button.js'
import '@material/web/labs/segmentedbuttonset/outlined-segmented-button-set.js'
import Icon from '../components/Icon.vue'
import { fetchSettings, saveSettings, type ManagerSettings } from '../api'

const settings = ref<ManagerSettings | null>(null)
const message = ref<{ ok: boolean; text: string } | null>(null)
const saving = ref(false)

// 回环避免方式：gid（meta skgid）优先 / mark（meta mark）备选
const bypassTproxy = ref<'gid' | 'mark'>('gid')
const bypassRedir = ref<'gid' | 'mark'>('gid')

const tabs = [
  { key: 'daemon', label: '守护进程' },
  { key: 'tun', label: 'TUN' },
  { key: 'tproxy', label: 'TPROXY' },
  { key: 'redir', label: 'REDIR' },
  { key: 'socks', label: 'SOCKS' },
  { key: 'server', label: 'SERVER' },
]
const tab = ref('daemon')

function onSegSelect(e: Event) {
  const d = (e as CustomEvent).detail as { selected: boolean; index: number }
  // 点击已选中段会触发 selected=false，忽略以避免整组变灰
  if (d.selected) tab.value = tabs[d.index]?.key ?? 'daemon'
}

function onInput(e: Event, set: (v: string) => void) {
  set((e.target as HTMLInputElement).value)
}

function onNum(e: Event, set: (v: number) => void) {
  const v = (e.target as HTMLInputElement).value
  set(v === '' ? 0 : Number(v))
}

onMounted(async () => {
  try {
    settings.value = await fetchSettings()
    if ((settings.value.modes.tproxy?.env?.exclude_gid ?? 0) <= 0) bypassTproxy.value = 'mark'
    if ((settings.value.modes['redir-tproxy']?.env?.exclude_gid ?? 0) <= 0) bypassRedir.value = 'mark'
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
})

async function save() {
  if (!settings.value) return
  saving.value = true
  message.value = null
  try {
    const s = settings.value
    const update: Record<string, unknown> = { daemon: s.daemon, modes: {} }
    const modes = update.modes as Record<string, unknown>

    modes.tun = {
      routing: s.modes.tun?.routing ?? { rule_index: 0, table_index: 0 },
      preset: s.modes.tun?.preset ?? '',
    }

    for (const [name, sel] of [
      ['tproxy', bypassTproxy],
      ['redir-tproxy', bypassRedir],
    ] as const) {
      const env = { ...(s.modes[name]?.env ?? {}) }
      // 二选一：未被选中的方式清零（守护侧校验至少其一）
      if (sel.value === 'gid') env.routing_mark = 0
      else env.exclude_gid = 0
      modes[name] = { env, preset: s.modes[name]?.preset ?? '' }
    }

    for (const name of ['socks']) {
      if (s.modes[name]) modes[name] = { preset: s.modes[name]!.preset ?? '' }
    }

    await saveSettings(update)
    message.value = { ok: true, text: '设置已保存' }
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div v-if="settings">
    <div class="page-head">
      <h1><Icon name="sliders" :size="26" /> 系统设置</h1>
      <p class="sub">
        以下字段写入 /opt/singbox-manager/manager.yaml——规则数字、环境变量与预定义入站的唯一事实源，
        由守护进程与各模式编排共同读取。
      </p>
    </div>

    <!-- 分段选项卡（玻璃胶囊） -->
    <div class="seg-wrap">
      <md-outlined-segmented-button-set @segmented-button-set-selection="onSegSelect">
        <md-outlined-segmented-button
          v-for="t in tabs"
          :key="t.key"
          :label="t.label"
          :selected="tab === t.key"
          no-checkmark
        ></md-outlined-segmented-button>
      </md-outlined-segmented-button-set>
    </div>

    <!-- 守护进程 -->
    <div v-show="tab === 'daemon'" class="card">
      <div class="section-title"><Icon name="server" :size="15" /> 守护进程</div>
      <div class="row">
        <div class="field">
          <md-outlined-text-field
            label="Web 监听地址"
            :value="settings.daemon.web_addr"
            @input="onInput($event, (v) => (settings.daemon.web_addr = v))"
          ></md-outlined-text-field>
          <small class="hint">面板无鉴权：默认仅监听 IPv4。需 IPv6 时设为 [::]:8082（仅 IPv6）或 :8082（双栈）</small>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="套规则延迟（毫秒）"
            type="number"
            :value="settings.daemon.apply_delay_ms"
            @input="onNum($event, (v) => (settings.daemon.apply_delay_ms = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="对账间隔（如 5s）"
            :value="settings.daemon.reconcile_interval"
            @input="onInput($event, (v) => (settings.daemon.reconcile_interval = v))"
          ></md-outlined-text-field>
        </div>
      </div>
    </div>

    <!-- TUN -->
    <div v-show="tab === 'tun'" class="card">
      <div class="section-title"><Icon name="activity" :size="15" /> TUN 路由索引（tun0 消失后按此清理）</div>
      <div class="row">
        <div class="field">
          <md-outlined-text-field
            label="ip rule 起始索引"
            type="number"
            :value="settings.modes.tun!.routing!.rule_index"
            @input="onNum($event, (v) => (settings.modes.tun!.routing!.rule_index = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="路由表索引"
            type="number"
            :value="settings.modes.tun!.routing!.table_index"
            @input="onNum($event, (v) => (settings.modes.tun!.routing!.table_index = v))"
          ></md-outlined-text-field>
        </div>
      </div>
      <md-outlined-text-field
        label="预定义入站 preset（JSON）"
        type="textarea"
        rows="12"
        class="code"
        :value="settings.modes.tun!.preset"
        @input="onInput($event, (v) => (settings.modes.tun!.preset = v))"
      ></md-outlined-text-field>
    </div>

    <!-- TPROXY -->
    <div v-show="tab === 'tproxy'" class="card">
      <div class="section-title"><Icon name="shuffle" :size="15" /> TPROXY 网络参数</div>
      <div class="row">
        <div class="field">
          <md-outlined-text-field
            label="TPROXY 端口（UDP）"
            type="number"
            :value="settings.modes.tproxy!.env!.tproxy_port"
            @input="onNum($event, (v) => (settings.modes.tproxy!.env!.tproxy_port = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="fwmark"
            type="number"
            :value="settings.modes.tproxy!.env!.fwmark"
            @input="onNum($event, (v) => (settings.modes.tproxy!.env!.fwmark = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="路由表 ID"
            type="number"
            :value="settings.modes.tproxy!.env!.table_id"
            @input="onNum($event, (v) => (settings.modes.tproxy!.env!.table_id = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="nftables 表名"
            :value="settings.modes.tproxy!.env!.nftables_table"
            @input="onInput($event, (v) => (settings.modes.tproxy!.env!.nftables_table = v))"
          ></md-outlined-text-field>
        </div>
      </div>

      <p class="section-hint">回环避免方式（二选一，gid 优先）</p>
      <div class="radio-row">
        <label class="radio-card" :class="{ selected: bypassTproxy === 'gid' }" @click="bypassTproxy = 'gid'" v-ripple>
          <md-radio name="bypass-tproxy" :checked="bypassTproxy === 'gid'"></md-radio>
          <span class="radio-text">
            <b>GID 放行</b>
            <span>meta skgid · 放行 sing-box 用户组流量</span>
          </span>
        </label>
        <label class="radio-card" :class="{ selected: bypassTproxy === 'mark' }" @click="bypassTproxy = 'mark'" v-ripple>
          <md-radio name="bypass-tproxy" :checked="bypassTproxy === 'mark'"></md-radio>
          <span class="radio-text">
            <b>路由 mark 放行</b>
            <span>meta mark · 放行已打标流量</span>
          </span>
        </label>
      </div>
      <div class="row" style="margin-top: 14px">
        <div class="field">
          <md-outlined-text-field
            label="排除 GID（sing-box 用户组）"
            type="number"
            :value="settings.modes.tproxy!.env!.exclude_gid"
            @input="onNum($event, (v) => (settings.modes.tproxy!.env!.exclude_gid = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="路由 mark"
            type="number"
            :value="settings.modes.tproxy!.env!.routing_mark"
            @input="onNum($event, (v) => (settings.modes.tproxy!.env!.routing_mark = v))"
          ></md-outlined-text-field>
        </div>
      </div>
    </div>

    <!-- REDIR-TPROXY -->
    <div v-show="tab === 'redir'" class="card">
      <div class="section-title"><Icon name="git-merge" :size="15" /> REDIR-TPROXY 网络参数（TCP REDIRECT + UDP TPROXY）</div>
      <div class="row">
        <div class="field">
          <md-outlined-text-field
            label="REDIRECT 端口（TCP）"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.redirect_port"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.redirect_port = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="TPROXY 端口（UDP）"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.tproxy_port"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.tproxy_port = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="fwmark"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.fwmark"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.fwmark = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="路由表 ID"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.table_id"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.table_id = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="nftables 表名"
            :value="settings.modes['redir-tproxy']!.env!.nftables_table"
            @input="onInput($event, (v) => (settings.modes['redir-tproxy']!.env!.nftables_table = v))"
          ></md-outlined-text-field>
        </div>
      </div>

      <p class="section-hint">回环避免方式（二选一，gid 优先）</p>
      <div class="radio-row">
        <label class="radio-card" :class="{ selected: bypassRedir === 'gid' }" @click="bypassRedir = 'gid'" v-ripple>
          <md-radio name="bypass-redir" :checked="bypassRedir === 'gid'"></md-radio>
          <span class="radio-text">
            <b>GID 放行</b>
            <span>meta skgid · 放行 sing-box 用户组流量</span>
          </span>
        </label>
        <label class="radio-card" :class="{ selected: bypassRedir === 'mark' }" @click="bypassRedir = 'mark'" v-ripple>
          <md-radio name="bypass-redir" :checked="bypassRedir === 'mark'"></md-radio>
          <span class="radio-text">
            <b>路由 mark 放行</b>
            <span>meta mark · 放行已打标流量</span>
          </span>
        </label>
      </div>
      <div class="row" style="margin-top: 14px">
        <div class="field">
          <md-outlined-text-field
            label="排除 GID（sing-box 用户组）"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.exclude_gid"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.exclude_gid = v))"
          ></md-outlined-text-field>
        </div>
        <div class="field">
          <md-outlined-text-field
            label="路由 mark"
            type="number"
            :value="settings.modes['redir-tproxy']!.env!.routing_mark"
            @input="onNum($event, (v) => (settings.modes['redir-tproxy']!.env!.routing_mark = v))"
          ></md-outlined-text-field>
        </div>
      </div>
    </div>

    <!-- SOCKS -->
    <div v-show="tab === 'socks'" class="card">
      <div class="section-title"><Icon name="zap" :size="15" /> SOCKS 预定义入站</div>
      <p class="section-hint">默认 mixed 20080</p>
      <md-outlined-text-field
        label="预定义入站 preset（JSON）"
        type="textarea"
        rows="10"
        class="code"
        :value="settings.modes.socks!.preset"
        @input="onInput($event, (v) => (settings.modes.socks!.preset = v))"
      ></md-outlined-text-field>
    </div>

    <!-- SERVER -->
    <div v-show="tab === 'server'" class="card">
      <div class="section-title"><Icon name="server" :size="15" /> SERVER 模式（用户自管）</div>
      <p class="sub">
        该模式的配置文件 <b style="font-family: var(--font-mono)">config_server.json</b> 由用户自行管理：
        管理器不生成、不覆盖、不校验其内容，只负责启动/停止与状态监控。
        请自行将配置文件放入 <b style="font-family: var(--font-mono)">/etc/singbox/config_server.json</b>，
        之后即可在状态页正常启停该模式。
      </p>
    </div>

    <!-- 保存栏（玻璃浮层） -->
    <div class="save-bar">
      <div v-if="message" class="alert" :class="message.ok ? 'ok' : 'err'">
        <Icon :name="message.ok ? 'check-circle' : 'alert-triangle'" :size="16" />
        <span>{{ message.text }}</span>
      </div>
      <md-filled-button :disabled="saving" @click="save">
        <Icon slot="icon" name="save" :size="16" />
        保存设置
      </md-filled-button>
    </div>
  </div>
  <div v-else class="card"><p class="sub">加载设置中…</p></div>
</template>
