<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { readApi, writeApi } from '../api'
import { isSuper } from '../stores/opsAuth'

const router = useRouter()
const items = ref([])
const loading = ref(false)
const err = ref('')
const showCreate = ref(false)
const form = ref({ name: '', description: '' })

async function load() {
  loading.value = true
  err.value = ''
  try {
    const data = await readApi.projects()
    items.value = Array.isArray(data?.items) ? data.items : []
  } catch (e) {
    err.value = e?.message || '加载失败'
  } finally {
    loading.value = false
  }
}

function open(id) {
  router.push(`/projects/${id}`)
}

function openCreate() {
  form.value = { name: '', description: '' }
  showCreate.value = true
}

async function create() {
  try {
    await writeApi.createProject(form.value)
    showCreate.value = false
    form.value = { name: '', description: '' }
    await load()
  } catch (e) {
    err.value = e?.message || '创建失败'
  }
}

function hasStatus(s) {
  return s === 'ok' || s === 'warn' || s === 'failed' || s === 'running' || s === 'pending'
}

function statusClass(s) {
  if (s === 'ok') return 'ok'
  if (s === 'warn') return 'warn'
  if (s === 'failed') return 'failed'
  if (s === 'running') return 'running'
  if (s === 'pending') return 'pending'
  return ''
}

function statusLabel(s) {
  if (s === 'ok') return '成功'
  if (s === 'warn') return '成功(有报错)'
  if (s === 'failed') return '失败'
  if (s === 'running') return '进行中'
  if (s === 'pending') return '排队'
  return s || ''
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>depman</h1>
        <p class="page-desc">按项目添加发布任务（目标环境 + 凭据 + 脚本），SSH 一键发布；同一环境可有多条任务。</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" :disabled="loading" @click="load">刷新</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openCreate">新建项目</button>
      </div>
    </div>

    <p v-if="err" class="ops-msg is-err">{{ err }}</p>
    <div v-if="loading && !items.length" class="empty">加载中…</div>
    <div v-else-if="!items.length" class="empty">暂无项目</div>
    <div v-else class="capsule-grid capsule-grid-home">
      <article
        v-for="it in items"
        :key="it.id"
        class="project-capsule project-capsule-home"
        @click="open(it.id)"
      >
        <h2 class="capsule-title">{{ it.name }}</h2>
        <p class="capsule-desc">{{ it.description || '（无描述）' }}</p>
        <div class="capsule-meta">
          <span>{{ it.envCount || 0 }} 发布任务</span>
          <span v-if="it.lastEnvName">最近：{{ it.lastEnvName }}</span>
        </div>
        <div class="capsule-foot">
          <time v-if="it.lastDeployAt" class="capsule-time">{{ new Date(it.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</time>
          <span v-else class="capsule-time capsule-time-empty">尚未发布</span>
          <span
            v-if="hasStatus(it.lastStatus)"
            class="status-pill"
            :class="statusClass(it.lastStatus)"
          >{{ statusLabel(it.lastStatus) }}</span>
        </div>
      </article>
    </div>

    <div v-if="showCreate" class="drawer-mask" @click.self="showCreate = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">新建项目</h2>
          <button type="button" class="btn btn-sm" @click="showCreate = false">关闭</button>
        </div>
        <div class="form-row">
          <label>名称</label>
          <input v-model="form.name" placeholder="项目名" />
        </div>
        <div class="form-row">
          <label>描述</label>
          <textarea v-model="form.description" rows="3" placeholder="可选" />
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="create">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
