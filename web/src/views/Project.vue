<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper } from '../stores/opsAuth'

const route = useRoute()
const router = useRouter()
const project = ref(null)
const parentProject = ref(null)
const childProjects = ref([])
const bindings = ref([])
const envs = ref([])
const sshCreds = ref([])
const err = ref('')
const busy = ref(false)
const jobLog = ref(null)
const bindForm = ref({ envId: '', sshCredId: '', script: '', workDir: '' })
const showBind = ref(false)
const editingPeId = ref('')

const id = computed(() => String(route.params.id || ''))

/** 绑定抽屉只展示所选环境的 SSH 凭据，避免选错环境时报后端校验错误 */
const bindSshCreds = computed(() => {
  const envId = bindForm.value.envId
  const list = sshCreds.value
  if (!envId) return list
  return list.filter((c) => c.envId === envId)
})

async function load() {
  err.value = ''
  parentProject.value = null
  childProjects.value = []
  try {
    const data = await readApi.project(id.value)
    project.value = data?.project || null
    bindings.value = Array.isArray(data?.envs) ? data.envs : []
    const e = await readApi.environments()
    envs.value = Array.isArray(e?.items) ? e.items : []
    const c = await readApi.credentials()
    sshCreds.value = (Array.isArray(c?.items) ? c.items : []).filter((x) => x.type === 'ssh')
    if (project.value?.parentId) {
      try {
        const parentData = await readApi.project(project.value.parentId)
        parentProject.value = parentData?.project || null
      } catch {
        parentProject.value = null
      }
    } else if (project.value?.id) {
      try {
        const all = await readApi.projects()
        const list = Array.isArray(all?.items) ? all.items : []
        childProjects.value = list.filter((it) => it.parentId === project.value.id)
      } catch {
        childProjects.value = []
      }
    }
  } catch (e) {
    err.value = e?.message || '加载失败'
  }
}

function envName(envId) {
  return envs.value.find((e) => e.id === envId)?.name || envId
}

function openBind(pe) {
  if (pe) {
    editingPeId.value = pe.id
    bindForm.value = {
      envId: pe.envId,
      sshCredId: pe.sshCredId || '',
      script: pe.script || '',
      workDir: pe.workDir || ''
    }
  } else {
    editingPeId.value = ''
    bindForm.value = { envId: '', sshCredId: '', script: 'echo deploy && hostname', workDir: '' }
  }
  showBind.value = true
}

function onBindEnvChange() {
  // 换环境后清掉不属于新环境的凭据
  if (!bindSshCreds.value.some((c) => c.id === bindForm.value.sshCredId)) {
    bindForm.value.sshCredId = ''
  }
}

async function saveBind() {
  busy.value = true
  err.value = ''
  try {
    await writeApi.upsertProjectEnv({
      projectId: id.value,
      envId: bindForm.value.envId,
      sshCredId: bindForm.value.sshCredId,
      script: bindForm.value.script,
      workDir: bindForm.value.workDir
    })
    showBind.value = false
    editingPeId.value = ''
    await load()
  } catch (e) {
    err.value = e?.message || '保存失败'
  } finally {
    busy.value = false
  }
}

async function deploy(pe) {
  if (!canWrite.value) {
    router.push('/ops')
    return
  }
  busy.value = true
  err.value = ''
  try {
    const job = await writeApi.deploy(pe.id)
    pollJob(job.id)
  } catch (e) {
    err.value = e?.message || '发布失败'
  } finally {
    busy.value = false
  }
}

async function removeBind(pe) {
  if (!isSuper.value) return
  const name = envName(pe.envId)
  if (!confirm(`删除「${name}」的发布绑定？`)) return
  busy.value = true
  err.value = ''
  try {
    await writeApi.deleteProjectEnv(pe.id)
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  } finally {
    busy.value = false
  }
}

let pollTimer = 0
function pollJob(jobId) {
  window.clearInterval(pollTimer)
  const tick = async () => {
    try {
      const j = await readApi.job(jobId)
      jobLog.value = j
      if (j.status === 'ok' || j.status === 'failed') {
        window.clearInterval(pollTimer)
        await load()
      }
    } catch {
      window.clearInterval(pollTimer)
    }
  }
  tick()
  pollTimer = window.setInterval(tick, 1500)
}

onMounted(load)
watch(id, load)
onUnmounted(() => window.clearInterval(pollTimer))
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <p v-if="parentProject" class="muted" style="margin:0 0 0.25rem;font-size:13px">
          <a href="#" @click.prevent="router.push(`/projects/${parentProject.id}`)">{{ parentProject.name }}</a>
          <span> / 子任务</span>
        </p>
        <h1>{{ project?.name || '项目' }}</h1>
        <p class="page-desc">{{ project?.description || '简单发布：绑定环境与脚本，SSH 到目标机执行。' }}</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" @click="router.push(parentProject ? `/projects/${parentProject.id}` : '/')">返回</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openBind(null)">绑定环境</button>
      </div>
    </div>
    <p v-if="err" class="ops-msg is-err">{{ err }}</p>

    <div v-if="childProjects.length" class="panel" style="margin-bottom:1rem">
      <div class="toolbar toolbar-start">
        <strong>子任务</strong>
        <span class="muted">{{ childProjects.length }} 个</span>
      </div>
      <div class="capsule-grid">
        <article
          v-for="ch in childProjects"
          :key="ch.id"
          class="project-capsule project-capsule-child"
          @click="router.push(`/projects/${ch.id}`)"
        >
          <h2 class="capsule-title">{{ ch.name }}</h2>
          <p class="muted" style="margin:0">{{ ch.description || '（无描述）' }}</p>
          <div class="capsule-meta">
            <span>{{ ch.envCount || 0 }} 环境</span>
          </div>
        </article>
      </div>
    </div>

    <div v-if="!bindings.length" class="empty">尚未绑定环境</div>
    <div v-else class="capsule-grid capsule-grid-bind">
      <article v-for="pe in bindings" :key="pe.id" class="project-capsule project-capsule-bind">
        <h2 class="capsule-title">{{ envName(pe.envId) }}</h2>
        <div class="capsule-body">
          <div class="capsule-meta">
            <span v-if="pe.lastStatus" class="status-pill" :class="pe.lastStatus">{{ pe.lastStatus }}</span>
            <span v-if="pe.lastDeployAt">{{ new Date(pe.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</span>
            <span v-if="pe.workDir">dir: {{ pe.workDir }}</span>
          </div>
          <pre class="capsule-script muted">{{ pe.script || '（无脚本）' }}</pre>
        </div>
        <div class="actions capsule-actions">
          <button type="button" class="btn btn-sm btn-primary" :disabled="busy" @click="deploy(pe)">发布</button>
          <button v-if="isSuper" type="button" class="btn btn-sm" :disabled="busy" @click="openBind(pe)">编辑</button>
          <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" :disabled="busy" @click="removeBind(pe)">删</button>
        </div>
      </article>
    </div>

    <div v-if="jobLog" class="panel" style="margin-top:1rem">
      <div class="toolbar toolbar-start">
        <strong>任务 {{ jobLog.status }}</strong>
        <span class="muted">{{ jobLog.id }}</span>
      </div>
      <div class="log-box">{{ jobLog.logTail || jobLog.error || '（等待输出…）' }}</div>
    </div>

    <div v-if="showBind" class="drawer-mask" @click.self="showBind = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">环境绑定</h2>
          <button type="button" class="btn btn-sm" @click="showBind = false">关闭</button>
        </div>
        <div class="form-row">
          <label>环境</label>
          <select v-model="bindForm.envId" :disabled="!!editingPeId" @change="onBindEnvChange">
            <option value="">选择环境</option>
            <option v-for="e in envs" :key="e.id" :value="e.id">{{ e.name }} ({{ e.kind }})</option>
          </select>
          <p v-if="editingPeId" class="muted" style="margin:0.35rem 0 0;font-size:12px">编辑时不可更换环境；若要换环境请删除后重新绑定</p>
        </div>
        <div class="form-row">
          <label>SSH 凭据</label>
          <select v-model="bindForm.sshCredId">
            <option value="">选择凭据</option>
            <option v-for="c in bindSshCreds" :key="c.id" :value="c.id">{{ c.name }} · {{ c.host }}</option>
          </select>
        </div>
        <div class="form-row">
          <label>工作目录</label>
          <input v-model="bindForm.workDir" placeholder="/opt/app" />
        </div>
        <div class="form-row">
          <label>发布脚本</label>
          <textarea v-model="bindForm.script" rows="6" placeholder="bash 命令" />
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" :disabled="busy" @click="saveBind">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
