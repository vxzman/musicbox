<script setup lang="ts">
import { computed } from 'vue'
import '@material/web/button/filled-button.js'
import '@material/web/button/outlined-button.js'
import Icon from '../components/Icon.vue'
import type { Status, ModeStatus } from '../api'

const props = defineProps<{
  status: Status | null
  loading: boolean
}>()

defineEmits<{ (e: 'action', mode: string, action: 'start' | 'stop'): void }>()

const modeOrder = ['tun', 'tproxy', 'redir-tproxy', 'socks', 'server']

const modeIcons: Record<string, string> = {
  tun: 'activity',
  tproxy: 'shuffle',
  'redir-tproxy': 'git-merge',
  socks: 'zap',
  server: 'server',
}

const entries = computed(() => {
  if (!props.status) return []
  const names = [...modeOrder, ...Object.keys(props.status.modes).filter((n) => !modeOrder.includes(n))]
  return names
    .filter((n) => props.status!.modes[n])
    .map((n, i) => ({ name: n, index: i, icon: modeIcons[n] ?? 'zap', ...props.status!.modes[n] }))
})

const activeEntry = computed(() => {
  if (!props.status?.active_mode) return null
  return props.status.modes[props.status.active_mode] ?? null
})

const stats = computed(() => {
  const es = entries.value
  return {
    total: es.length,
    running: es.filter((m) => m.active).length,
    rulesOk: es.filter((m) => rulesClass(m) === 'active').length,
    failed: es.filter((m) => m.unit_state === 'failed').length,
  }
})

const rulesText: Record<string, string> = {
  present: '规则就位',
  missing: '规则缺失',
  clean: '无残留',
  leftover: '有残留',
  na: '不适用',
}

function rulesClass(m: ModeStatus): string {
  if (m.rules === 'present' || m.rules === 'clean') return 'active'
  if (m.rules === 'missing' || m.rules === 'leftover') return 'partial'
  return ''
}

const unitText: Record<string, string> = {
  active: '运行中',
  activating: '启动中',
  deactivating: '停止中',
  inactive: '已停止',
  failed: '失败',
  unknown: '未知',
}

function unitClass(s: string): string {
  if (s === 'active' || s === 'activating' || s === 'deactivating') return 'active'
  if (s === 'failed') return 'failed'
  return ''
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1><Icon name="activity" :size="26" /> 运行状态</h1>
      <p class="sub">
        切换模式后由守护进程自动编排：先停其他互斥模式，启动 sing-box 实例后延迟套用透明代理规则，
        规则与实例同生共死；tun 模式仅在 tun 接口消失后做残留清理。
      </p>
    </div>

    <!-- 摘要 Hero：模式色氛围渗入 -->
    <div class="hero" :class="status?.active_mode ? `c-${status.active_mode}` : ''">
      <div class="hero-icon">
        <Icon
          :name="status?.active_mode ? (modeIcons[status.active_mode] ?? 'zap') : 'activity'"
          :size="30"
        />
      </div>
      <div class="hero-body">
        <div class="hero-title">
          {{ !status ? '正在连接守护进程…' : activeEntry ? activeEntry.label : '无活跃模式' }}
        </div>
        <div class="hero-sub">
          {{
            !status
              ? '状态同步通过 SSE 自动恢复'
              : activeEntry
                ? `${entries.length} 种模式 · ${activeEntry.unit} 运行中`
                : `${entries.length} 种模式 · 全部已停止`
          }}
        </div>
      </div>
      <span class="chip" :class="!status ? 'chip--warn' : activeEntry ? 'chip--ok' : 'chip--tonal'">
        <span class="dot"></span>{{ !status ? '连接中' : activeEntry ? '运行中' : '待机' }}
      </span>
    </div>

    <!-- 统计条 -->
    <div v-if="status" class="stats">
      <div class="stat-tile" :style="{ animationDelay: '0s' }">
        <span class="stat-label"><span class="stat-ic"><Icon name="grid" :size="14" /></span> 模式总数</span>
        <span class="stat-value">{{ stats.total }}</span>
      </div>
      <div class="stat-tile" :class="{ ok: stats.running > 0 }" :style="{ animationDelay: '0.05s' }">
        <span class="stat-label"><span class="stat-ic"><Icon name="activity" :size="14" /></span> 运行中</span>
        <span class="stat-value">{{ stats.running }}</span>
      </div>
      <div class="stat-tile" :class="{ ok: stats.rulesOk > 0 }" :style="{ animationDelay: '0.1s' }">
        <span class="stat-label"><span class="stat-ic"><Icon name="check-circle" :size="14" /></span> 规则就位</span>
        <span class="stat-value">{{ stats.rulesOk }}</span>
      </div>
      <div class="stat-tile" :class="{ err: stats.failed > 0 }" :style="{ animationDelay: '0.15s' }">
        <span class="stat-label"><span class="stat-ic"><Icon name="alert-triangle" :size="14" /></span> 失败</span>
        <span class="stat-value">{{ stats.failed }}</span>
      </div>
    </div>

    <!-- 行式模式列表 -->
    <div class="mode-list">
      <div
        v-for="m in entries"
        :key="m.name"
        class="mode-row"
        :class="[`c-${m.name}`, { active: m.active }]"
        :style="{ animationDelay: `${0.15 + m.index * 0.05}s` }"
      >
        <div class="mode-icon"><Icon :name="m.icon" :size="20" /></div>

        <div class="mode-body">
          <div class="name">
            {{ m.label }} <small>{{ m.name }}</small>
          </div>
          <div class="unit" :title="m.unit_state === 'unknown' ? '未查询到（单元未安装？）' : m.unit">
            {{ m.unit_state === 'unknown' ? '未查询到（单元未安装？）' : m.unit }}
          </div>
        </div>

        <div class="mode-chips">
          <span
            class="chip chip--small"
            :class="{ 'chip--ok': unitClass(m.unit_state) === 'active', 'chip--err': unitClass(m.unit_state) === 'failed' }"
          >
            <span class="dot"></span>
            {{ unitText[m.unit_state] ?? m.unit_state }}
          </span>
          <span class="chip chip--small" :class="{ 'chip--ok': rulesClass(m) === 'active', 'chip--warn': rulesClass(m) === 'partial' }">
            {{ rulesText[m.rules] ?? m.rules }}
          </span>
        </div>

        <div class="mode-actions">
          <md-filled-button :disabled="loading || m.active" @click="$emit('action', m.name, 'start')">
            <Icon slot="icon" name="power" :size="16" />
            启动
          </md-filled-button>
          <md-outlined-button
            class="danger-btn"
            :disabled="loading || !m.active"
            @click="$emit('action', m.name, 'stop')"
          >
            <Icon slot="icon" name="stop" :size="16" />
            停止
          </md-outlined-button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.danger-btn {
  --md-outlined-button-outline-color: var(--md-sys-color-error);
  --md-outlined-button-label-text-color: var(--md-sys-color-error);
  --md-outlined-button-hover-label-text-color: var(--md-sys-color-error);
  --md-outlined-button-focus-label-text-color: var(--md-sys-color-error);
  --md-outlined-button-pressed-label-text-color: var(--md-sys-color-error);
  --md-outlined-button-hover-state-layer-color: var(--md-sys-color-error);
  --md-outlined-button-focus-state-layer-color: var(--md-sys-color-error);
  --md-outlined-button-pressed-state-layer-color: var(--md-sys-color-error);
}
</style>
