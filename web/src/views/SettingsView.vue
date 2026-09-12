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

const socksPort = ref(20080)

const tabs = [
  { key: 'daemon', label: '守护进程' },
  { key: 'tun', label: 'TUN' },
  { key: 'tproxy', label: 'TPROXY' },
  { key: 'redir', label: 'REDIR' },
  { key: 'socks', label: 'SOCKS' },
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

// 从 socks preset 里取出 mixed 入站端口（解析失败回退默认 20080）。
function readSocksPort(preset?: string): number {
  try {
    const arr = JSON.parse(preset ?? '')
    if (Array.isArray(arr) && arr[0] && typeof arr[0] === 'object' && arr[0].listen_port) {
      return Number(arr[0].listen_port)
    }
  } catch {
    /* 解析失败回退默认端口 */
  }
  return 20080
}

// 把端口写回 socks preset（preset 缺失/非法时按默认 mixed 结构生成）。
function buildSocksPreset(port: number, preset?: string): string {
  try {
    const arr = JSON.parse(preset ?? '')
    if (Array.isArray(arr) && arr[0] && typeof arr[0] === 'object') {
      arr[0].listen_port = port
      return JSON.stringify(arr, null, 2)
    }
  } catch {
    /* 重建默认 preset */
  }
  return JSON.stringify(
    [{ type: 'mixed', tag: 'mixed-in', listen: '0.0.0.0', listen_port: port, tcp_fast_open: true }],
    null,
    2,
  )
}

onMounted(async () => {
  try {
    settings.value = await fetchSettings()
    if ((settings.value.modes.tproxy?.env?.exclude_gid ?? 0) <= 0) bypassTproxy.value = 'mark'
    if ((settings.value.modes['redir-tproxy']?.env?.exclude_gid ?? 0) <= 0) bypassRedir.value = 'mark'
    socksPort.value = readSocksPort(settings.value.modes.socks?.preset)
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

    if (s.modes.socks) modes.socks = { preset: buildSocksPreset(socksPort.value, s.modes.socks.preset) }

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
      <h1><Icon name="sliders" :size="22" /> 系统设置</h1>
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
      <div class="section-title"><Icon name="activity" :size="15" /> TUN 路由索引</div>
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

      <div class="radio-row">
        <label class="radio-card" :class="{ selected: bypassTproxy === 'gid' }" @click="bypassTproxy = 'gid'" v-ripple>
          <md-radio name="bypass-tproxy" :checked="bypassTproxy === 'gid'"></md-radio>
          <span class="radio-text">
            <b>GID 放行</b>
            <span>meta skgid · 放行代理服务用户组流量</span>
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
            label="排除 GID（代理服务用户组）"
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
      <div class="section-title"><Icon name="git-merge" :size="15" /> REDIR-TPROXY 网络参数</div>
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

      <div class="radio-row">
        <label class="radio-card" :class="{ selected: bypassRedir === 'gid' }" @click="bypassRedir = 'gid'" v-ripple>
          <md-radio name="bypass-redir" :checked="bypassRedir === 'gid'"></md-radio>
          <span class="radio-text">
            <b>GID 放行</b>
            <span>meta skgid · 放行代理服务用户组流量</span>
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
            label="排除 GID（代理服务用户组）"
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
      <div class="section-title"><Icon name="zap" :size="15" /> SOCKS 入站端口</div>
      <div class="row">
        <div class="field">
          <md-outlined-text-field
            label="入站端口"
            type="number"
            :value="socksPort"
            @input="onNum($event, (v) => (socksPort = v))"
          ></md-outlined-text-field>
        </div>
      </div>
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
