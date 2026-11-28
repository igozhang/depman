<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { readApi, writeApi } from '../api'
import { isSuper } from '../stores/opsAuth'

const router = useRouter()
const items = ref([])
const loading = ref(false)
const err = ref('')
const showCreate = ref(false)
const form = ref({ name: '', description: '', parentId: '' })

/** 根项目；若父级因权限过滤不可见，子任务升为顶栏展示，避免“孤儿消失” */
const roots = computed(() => {
  const ids = new Set(items.value.map((it) => it.id))
  return items.value.filter((it) => !it.parentId || !ids.has(it.parentId))
})

function childrenOf(parentId) {
  return items.value.filter((it) => it.parentId === parentId)
}

function isRealRoot(it) {
  return !it.parentId
}

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

function openCreate(parentId = '') {
  form.value = { name: '', description: '', parentId: parentId || '' }
  showCreate.value = true
}

async function create() {
  try {
    const body = {
      name: form.value.name,
      description: form.value.description
    }
    if (form.value.parentId) body.parentId = form.value.parentId
    await writeApi.createProject(body)
    showCreate.value = false
    form.value = { name: '', description: '', parentId: '' }
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

const drawerTitle = computed(() => (form.value.parentId ? '新建子任务' : '新建项目'))

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>depman</h1>
        <p class="page-desc">简单发布：按项目绑定环境，SSH 执行脚本一键部署。可为同一项目添加多个子任务。</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" :disabled="loading" @click="load">刷新</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openCreate('')">新建项目</button>
      </div>
    </div>

    <p v-if="err" class="ops-msg is-err">{{ err }}</p>
    <div v-if="loading && !items.length" class="empty">加载中…</div>
    <div v-else-if="!roots.length" class="empty">
      暂无项目
    </div>
    <div v-else class="project-groups">
      <section v-for="root in roots" :key="root.id" class="project-group">
        <div class="toolbar project-group-head">
          <article class="project-capsule project-capsule-root" @click="open(root.id)">
            <h2 class="capsule-title">{{ root.name }}</h2>
            <p class="muted" style="margin:0;min-height:1.3em">{{ root.description || '（无描述）' }}</p>
            <div class="capsule-meta">
              <span v-if="root.parentId" class="status-pill">子任务</span>
              <span>{{ root.envCount || 0 }} 环境</span>
              <span v-if="isRealRoot(root) && childrenOf(root.id).length">{{ childrenOf(root.id).length }} 子任务</span>
              <span v-if="root.lastEnvName">最近：{{ root.lastEnvName }}</span>
              <span v-if="root.lastStatus" class="status-pill" :class="statusClass(root.lastStatus)">{{ root.lastStatus }}</span>
              <span v-if="root.lastDeployAt">{{ new Date(root.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</span>
            </div>
          </article>
          <div v-if="isSuper && isRealRoot(root)" class="actions" @click.stop>
            <button type="button" class="btn btn-sm" @click="openCreate(root.id)">添加子任务</button>
          </div>
        </div>

        <div v-if="isRealRoot(root) && childrenOf(root.id).length" class="capsule-grid project-children">
          <article
            v-for="child in childrenOf(root.id)"
            :key="child.id"
            class="project-capsule project-capsule-child"
            @click="open(child.id)"
          >
            <h2 class="capsule-title">{{ child.name }}</h2>
            <p class="muted" style="margin:0;min-height:1.3em">{{ child.description || '（无描述）' }}</p>
            <div class="capsule-meta">
              <span>{{ child.envCount || 0 }} 环境</span>
              <span v-if="child.lastEnvName">最近：{{ child.lastEnvName }}</span>
              <span v-if="child.lastStatus" class="status-pill" :class="statusClass(child.lastStatus)">{{ child.lastStatus }}</span>
              <span v-if="child.lastDeployAt">{{ new Date(child.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</span>
            </div>
          </article>
        </div>
        <p v-else-if="isSuper && isRealRoot(root)" class="muted project-children-empty">暂无子任务，可添加同项目下的不同发布任务</p>
      </section>
    </div>

    <div v-if="showCreate" class="drawer-mask" @click.self="showCreate = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">{{ drawerTitle }}</h2>
          <button type="button" class="btn btn-sm" @click="showCreate = false">关闭</button>
        </div>
        <div v-if="form.parentId" class="form-row">
          <label>所属项目</label>
          <input :value="roots.find((r) => r.id === form.parentId)?.name || form.parentId" disabled />
        </div>
        <div class="form-row">
          <label>名称</label>
          <input v-model="form.name" :placeholder="form.parentId ? '子任务名' : '项目名'" />
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
