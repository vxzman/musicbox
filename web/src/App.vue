<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import '@material/web/iconbutton/icon-button.js'
import '@material/web/progress/linear-progress.js'
import '@material/web/labs/navigationbar/navigation-bar.js'
import '@material/web/labs/navigationtab/navigation-tab.js'
import StatusView from './views/StatusView.vue'
import ConfigView from './views/ConfigView.vue'
import SettingsView from './views/SettingsView.vue'
import Icon from './components/Icon.vue'
import { fetchStatus, modeAction, subscribeStatus, type Status } from './api'

type View = 'status' | 'config' | 'settings'
const view = ref<View>('status')
const status = ref<Status | null>(null)
const loading = ref(false)
const message = ref<{ ok: boolean; text: string } | null>(null)
let messageTimer: ReturnType<typeof setTimeout> | null = null

let unsubscribe: (() => void) | null = null

onMounted(() => {
  refresh()
  unsubscribe = subscribeStatus((s) => (status.value = s))
})

onUnmounted(() => {
  unsubscribe?.()
  if (messageTimer) clearTimeout(messageTimer)
})

async function refresh() {
  try {
    status.value = await fetchStatus()
  } catch {
    /* 守护进程暂不可达，SSE 重连后恢复 */
  }
}

async function onModeAction(mode: string, action: 'start' | 'stop') {
  loading.value = true
  try {
    await modeAction(mode, action)
    showMessage(true, `模式 ${mode} ${action === 'start' ? '启动' : '停止'}指令已执行`)
    await refresh()
  } catch (e) {
    showMessage(false, (e as Error).message)
  } finally {
    loading.value = false
  }
}

function showMessage(ok: boolean, text: string) {
  message.value = { ok, text }
  if (messageTimer) clearTimeout(messageTimer)
  messageTimer = setTimeout(() => (message.value = null), 5000)
}

const nav: { key: View; label: string; icon: string }[] = [
  { key: 'status', label: '运行状态', icon: 'activity' },
  { key: 'config', label: '配置管理', icon: 'file' },
  { key: 'settings', label: '系统设置', icon: 'sliders' },
]

const navIndex = computed(() => nav.findIndex((i) => i.key === view.value))

function onNavActivated(e: { detail: { activeIndex: number } }) {
  view.value = nav[e.detail.activeIndex]?.key ?? 'status'
}

const activeLabel = () => {
  const s = status.value
  if (!s || !s.active_mode) return '无活跃模式'
  return s.modes[s.active_mode]?.label ?? s.active_mode
}

// 顶栏状态芯片：连接中 → 警告；活跃 → 绿色呼吸点；待机 → 紫色调容器
const appChipClass = computed(() => {
  if (!status.value) return 'chip--warn'
  return status.value.active_mode ? 'chip--ok' : 'chip--tonal'
})

const appChipText = computed(() => (!status.value ? '连接中…' : activeLabel()))
</script>

<template>
  <div class="app" :class="`view-${view}`">
    <!-- 氛围层：光斑 + bokeh 色泡 + 噪点（Level 1 Ambient） -->
    <div class="ambient" aria-hidden="true">
      <div class="glow glow-purple"></div>
      <div class="glow glow-blue"></div>
      <div class="glow glow-pink"></div>
      <div class="bokeh bokeh-b1"></div>
      <div class="bokeh bokeh-b2"></div>
      <div class="bokeh bokeh-b3"></div>
      <div class="bokeh bokeh-b4"></div>
      <div class="bokeh bokeh-b5"></div>
      <div class="noise"></div>
    </div>

    <!-- 居中面板：导航 + 主内容收进固定宽度窗口，不铺满视口；
         snackbar / 底部导航留在面板外，仍相对视口定位 -->
    <div class="app-frame">
      <header class="nav-wrap">
        <nav class="nav-inner" aria-label="主导航">
          <span class="logo">
            <!-- MusicBox 原创八音盒图标（与标签页 favicon 同款，代码重绘） -->
            <span class="logo-mark">
              <svg width="22" height="22" viewBox="0 0 100 100" aria-hidden="true">
                <defs>
                  <linearGradient id="nav-mb-top" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" stop-color="#d0bcff"/>
                    <stop offset="100%" stop-color="#b69ce6"/>
                  </linearGradient>
                  <linearGradient id="nav-mb-left" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" stop-color="#8a6fd0"/>
                    <stop offset="100%" stop-color="#7354c0"/>
                  </linearGradient>
                  <linearGradient id="nav-mb-right" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" stop-color="#6750a4"/>
                    <stop offset="100%" stop-color="#4f378b"/>
                  </linearGradient>
                </defs>
                <!-- 3D 音乐盒立方体 -->
                <polygon points="50,18 82,34 50,50 18,34" fill="url(#nav-mb-top)"/>
                <polygon points="18,34 50,50 50,82 18,66" fill="url(#nav-mb-left)"/>
                <polygon points="50,50 82,34 82,66 50,82" fill="url(#nav-mb-right)"/>
                <polyline points="18,34 50,50 82,34" fill="none" stroke="#ffffff" stroke-width="2" opacity="0.6"/>
                <line x1="50" y1="50" x2="50" y2="82" stroke="#ffffff" stroke-width="1.5" opacity="0.4"/>
                <!-- 纯白双连音符 ♫ -->
                <g fill="#ffffff" filter="drop-shadow(0 2px 3px rgba(33, 0, 93, 0.5))">
                  <ellipse cx="38" cy="62" rx="7.5" ry="5.5" transform="rotate(-20 38 62)"/>
                  <ellipse cx="58" cy="54" rx="7.5" ry="5.5" transform="rotate(-20 58 54)"/>
                  <rect x="43" y="30" width="3.5" height="32" rx="1.5"/>
                  <rect x="63" y="22" width="3.5" height="32" rx="1.5"/>
                  <polygon points="43,30 66.5,22 66.5,28 43,36"/>
                </g>
                <!-- 旋律弧光声波 -->
                <path d="M 72,16 A 12 12 0 0 1 80,28" stroke="#6750a4" stroke-width="2.5" stroke-linecap="round" fill="none" opacity="0.9"/>
                <path d="M 78,11 A 19 19 0 0 1 90,29" stroke="#6750a4" stroke-width="2.5" stroke-linecap="round" fill="none" opacity="0.6"/>
              </svg>
            </span>
            <span class="logo-text">MusicBox</span>
          </span>

          <div class="nav-links">
            <button
              v-for="item in nav"
              :key="item.key"
              type="button"
              class="nav-item"
              :class="{ active: view === item.key }"
              :aria-current="view === item.key ? 'page' : undefined"
              @click="view = item.key"
              v-ripple
            >
              <Icon :name="item.icon" :size="16" />
              <span>{{ item.label }}</span>
            </button>
          </div>

          <div class="nav-actions">
            <span class="chip" :class="appChipClass"><span class="dot"></span>{{ appChipText }}</span>
            <md-icon-button aria-label="刷新状态" title="刷新状态" @click="refresh">
              <Icon name="refresh-cw" :size="18" />
            </md-icon-button>
          </div>

          <md-linear-progress v-if="loading" indeterminate class="app-bar-progress"></md-linear-progress>
        </nav>
      </header>

      <main class="main">
        <Transition name="view" mode="out-in">
          <StatusView
            v-if="view === 'status'"
            key="status"
            :status="status"
            :loading="loading"
            @action="onModeAction"
          />
          <ConfigView v-else-if="view === 'config'" key="config" />
          <SettingsView v-else key="settings" />
        </Transition>
      </main>
    </div>

    <!-- 移动端：MD3 底部导航 -->
    <nav class="bottom-nav">
      <md-navigation-bar :active-index="navIndex" @navigation-bar-activated="onNavActivated">
        <md-navigation-tab v-for="item in nav" :key="item.key" :label="item.label">
          <Icon slot="icon" :name="item.icon" :size="24" />
          <Icon slot="activeIcon" :name="item.icon" :size="24" />
        </md-navigation-tab>
      </md-navigation-bar>
    </nav>

    <!-- Snackbar：玻璃逆表面 -->
    <transition name="fade">
      <div v-if="message" class="snackbar" :class="{ error: !message.ok }" role="status" aria-live="polite">
        <Icon :name="message.ok ? 'check-circle' : 'alert-triangle'" :size="18" />
        {{ message.text }}
      </div>
    </transition>
  </div>
</template>
