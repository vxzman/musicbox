<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import '@material/web/button/filled-button.js'
import '@material/web/button/outlined-button.js'
import '@material/web/button/text-button.js'
import '@material/web/progress/linear-progress.js'
import '@material/web/textfield/outlined-text-field.js'
import Icon from '../components/Icon.vue'
import { configSync, fetchGeneral, fetchModeConfig, fetchSettings, saveGeneral, saveModeConfig } from '../api'

const target = ref('general')
const modes = ref<string[]>([])
// 用户自管模式（server 等）：无 preset 且无 endpoints，配置文件可直接编辑保存
const selfManaged = ref<Set<string>>(new Set())
const content = ref('')
const loading = ref(false)
const message = ref<{ ok: boolean; text: string } | null>(null)

const fileName = ref('config_generic.json')

const editable = computed(() => target.value === 'general' || selfManaged.value.has(target.value))

const files = computed(() => [
  { key: 'general', label: 'config_generic.json', sub: '通用配置 · 可编辑' },
  ...modes.value.map((m) => ({
    key: m,
    label: `config_${m}.json`,
    sub: selfManaged.value.has(m) ? '用户自管 · 可编辑' : '自动生成 · 只读',
  })),
])

onMounted(async () => {
  try {
    const settings = await fetchSettings()
    modes.value = Object.keys(settings.modes)
    selfManaged.value = new Set(
      Object.entries(settings.modes)
        .filter(([, m]) => !m.preset && !m.endpoints)
        .map(([name]) => name),
    )
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
  await load()
})

watch(target, () => load())

async function load() {
  loading.value = true
  message.value = null
  try {
    fileName.value = `config_${target.value}.json`
    content.value = target.value === 'general' ? await fetchGeneral() : await fetchModeConfig(target.value)
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  } finally {
    loading.value = false
  }
}

async function save() {
  message.value = null
  try {
    if (target.value === 'general') {
      await saveGeneral(content.value)
      message.value = { ok: true, text: '通用配置已保存，各模式配置已同步生成' }
    } else {
      await saveModeConfig(target.value, content.value)
      message.value = { ok: true, text: `${fileName.value} 已保存` }
    }
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
}

async function sync() {
  message.value = null
  try {
    await configSync()
    message.value = { ok: true, text: '各模式配置已重新生成' }
    if (target.value !== 'general') await load()
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1><Icon name="file" :size="26" /> 配置管理</h1>
    </div>

    <div class="config-layout">
      <!-- 左：文件导航列表（玻璃卡） -->
      <nav class="file-nav" aria-label="配置文件列表">
        <div class="file-nav-title">配置文件</div>
        <button
          v-for="f in files"
          :key="f.key"
          type="button"
          class="file-item"
          :class="{ active: target === f.key }"
          :aria-current="target === f.key ? 'page' : undefined"
          @click="target = f.key"
          v-ripple
        >
          <Icon name="file" :size="16" class="fi-icon" />
          <span class="fi-text">
            <span class="fi-name">{{ f.label }}</span>
            <span class="fi-sub">{{ f.sub }}</span>
          </span>
        </button>
      </nav>

      <!-- 右：编辑器面板 -->
      <div class="editor-pane">
        <div class="toolbar">
          <span class="chip chip--mono"><Icon name="file" :size="14" /> {{ fileName }}</span>
          <span v-if="!editable" class="chip chip--tonal"><Icon name="eye" :size="14" /> 只读 · 自动生成</span>
          <span v-else-if="target !== 'general'" class="chip chip--tonal"><Icon name="save" :size="14" /> 用户自管 · 可编辑</span>

          <span class="spacer"></span>

          <md-filled-button v-if="editable" :disabled="loading" @click="save">
            <Icon slot="icon" name="save" :size="16" />
            {{ target === 'general' ? '保存并同步' : '保存' }}
          </md-filled-button>
          <md-outlined-button v-else :disabled="loading" @click="sync">
            <Icon slot="icon" name="refresh-cw" :size="16" />
            重新生成
          </md-outlined-button>
          <md-text-button :disabled="loading" @click="load">
            <Icon slot="icon" name="refresh-cw" :size="16" />
            刷新
          </md-text-button>
        </div>

        <md-linear-progress v-if="loading" indeterminate class="editor-progress"></md-linear-progress>

        <!-- 代码编辑区：高不透明度玻璃，保证文字锐利 -->
        <div class="editor" :class="{ 'editor--readonly': !editable }">
          <md-outlined-text-field
            :value="content"
            type="textarea"
            rows="26"
            class="code"
            :readonly="!editable"
            spellcheck="false"
            @input="content = ($event.target as HTMLInputElement).value"
          ></md-outlined-text-field>
        </div>

        <div v-if="message" class="alert" :class="message.ok ? 'ok' : 'err'">
          <Icon :name="message.ok ? 'check-circle' : 'alert-triangle'" :size="16" />
          <span>{{ message.text }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
