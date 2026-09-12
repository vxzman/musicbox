<script setup lang="ts">
import { computed } from 'vue'
import Icon from '../components/Icon.vue'
import type { Status, ModeStatus } from '../api'

const props = defineProps<{
  status: Status | null
  loading: boolean
}>()

defineEmits<{ (e: 'action', mode: string, action: 'start' | 'stop'): void }>()

const modeOrder = ['tun', 'tproxy', 'redir-tproxy', 'socks']

const modeIcons: Record<string, string> = {
  tun: 'activity',
  tproxy: 'shuffle',
  'redir-tproxy': 'git-merge',
  socks: 'zap',
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
      <h1><Icon name="activity" :size="22" /> 运行状态</h1>
    </div>

    <!-- 摘要 Hero：模式色氛围渗入 -->
    <div class="hero" :class="status?.active_mode ? `c-${status.active_mode}` : ''">
      <div class="hero-icon">
        <Icon
          :name="status?.active_mode ? (modeIcons[status.active_mode] ?? 'zap') : 'activity'"
          :size="24"
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
          <!-- 已停止不显示状态芯片：开关位置本身已表达启停状态 -->
          <span
            v-if="m.unit_state !== 'inactive'"
            class="chip chip--small status-chip"
            :class="{ 'chip--ok': unitClass(m.unit_state) === 'active', 'chip--err': unitClass(m.unit_state) === 'failed' }"
          >
            <span class="dot"></span>
            {{ unitText[m.unit_state] ?? m.unit_state }}
          </span>
          <span class="chip chip--small rules-chip" :class="{ 'chip--ok': rulesClass(m) === 'active', 'chip--warn': rulesClass(m) === 'partial' }">
            {{ rulesText[m.rules] ?? m.rules }}
          </span>
        </div>

        <div class="mode-actions">
          <!-- 胶囊开关：圆点在左=停止，滑到右侧点亮=启动 -->
          <button
            type="button"
            class="mode-toggle"
            :class="{ on: m.active }"
            :disabled="loading"
            :aria-label="m.active ? `停止 ${m.label}` : `启动 ${m.label}`"
            :title="m.active ? `停止 ${m.label}` : `启动 ${m.label}`"
            @click="$emit('action', m.name, m.active ? 'stop' : 'start')"
          >
            <span class="knob"></span>
          </button>
        </div>
      </div>
    </div>
  </div>
</template>
