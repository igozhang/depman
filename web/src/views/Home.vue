<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper } from '../stores/opsAuth'

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

function statusClass(s) {
  if (s === 'ok') return 'ok'
  if (s === 'failed') return 'failed'
  if (s === 'running') return 'running'
  return ''
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>depman</h1>
        <p class="page-desc">简单发布：按项目绑定环境，SSH 执行脚本一键部署。</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" :disabled="loading" @click="load">刷新</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="showCreate = true">新建项目</button>
      </div>
    </div>

    <p v-if="err" class="ops-msg is-err">{{ err }}</p>
    <div v-if="loading && !items.length" class="empty">加载中…</div>
    <div v-else-if="!items.length" class="empty">
      暂无项目
    </div>
    <div v-else class="capsule-grid">
      <article
        v-for="it in items"
        :key="it.id"
        class="project-capsule"
        @click="open(it.id)"
      >
        <h2 class="capsule-title">{{ it.name }}</h2>
        <p class="muted" style="margin:0;min-height:1.3em">{{ it.description || '（无描述）' }}</p>
        <div class="capsule-meta">
          <span>{{ it.envCount || 0 }} 环境</span>
          <span v-if="it.lastEnvName">最近：{{ it.lastEnvName }}</span>
          <span v-if="it.lastStatus" class="status-pill" :class="statusClass(it.lastStatus)">{{ it.lastStatus }}</span>
          <span v-if="it.lastDeployAt">{{ new Date(it.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</span>
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
