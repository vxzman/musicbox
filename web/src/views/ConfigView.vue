<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import '@material/web/button/filled-button.js'
import '@material/web/button/outlined-button.js'
import '@material/web/button/text-button.js'
import '@material/web/progress/linear-progress.js'
import '@material/web/textfield/outlined-text-field.js'
import Icon from '../components/Icon.vue'
import { configSync, fetchGeneral, fetchModeConfig, fetchSettings, saveGeneral } from '../api'

const target = ref('general')
const modes = ref<string[]>([])
const content = ref('')
const loading = ref(false)
const message = ref<{ ok: boolean; text: string } | null>(null)
const isGeneral = ref(true)

const fileName = ref('config_generic.json')

const files = computed(() => [
  { key: 'general', label: 'config_generic.json', sub: '通用配置 · 可编辑' },
  ...modes.value.map((m) => ({ key: m, label: `config_${m}.json`, sub: '自动生成 · 只读' })),
])

onMounted(async () => {
  try {
    const settings = await fetchSettings()
    modes.value = Object.keys(settings.modes)
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
    isGeneral.value = target.value === 'general'
    fileName.value = `config_${target.value}.json`
    content.value = isGeneral.value ? await fetchGeneral() : await fetchModeConfig(target.value)
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  } finally {
    loading.value = false
  }
}

async function save() {
  message.value = null
  try {
    await saveGeneral(content.value)
    message.value = { ok: true, text: '通用配置已保存，各模式配置已同步生成' }
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
}

async function sync() {
  message.value = null
  try {
    await configSync()
    message.value = { ok: true, text: '各模式配置已重新生成' }
    if (!isGeneral.value) await load()
  } catch (e) {
    message.value = { ok: false, text: (e as Error).message }
  }
}
</script>

<template>
  <div>
    <div class="page-head">
      <h1><Icon name="file" :size="26" /> 配置管理</h1>
      <p class="sub">
        通用配置是唯一编辑入口；各模式配置由「通用配置 + manager.yaml 预定义入站」合并生成（server 模式除外——其配置由用户自管）
        （校验通过 sing-box check 后原子写入）。修改预定义入站请到系统设置页。
      </p>
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
          <span v-if="!isGeneral" class="chip chip--tonal"><Icon name="eye" :size="14" /> 只读 · 自动生成</span>

          <span class="spacer"></span>

          <md-filled-button v-if="isGeneral" :disabled="loading" @click="save">
            <Icon slot="icon" name="save" :size="16" />
            保存并同步
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
        <div class="editor" :class="{ 'editor--readonly': !isGeneral }">
          <md-outlined-text-field
            :value="content"
            type="textarea"
            rows="26"
            class="code"
            :readonly="!isGeneral"
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
