<script setup>
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper } from '../stores/opsAuth'

const route = useRoute()
const router = useRouter()
const project = ref(null)
const bindings = ref([])
const envs = ref([])
const sshCreds = ref([])
const err = ref('')
const busy = ref(false)
const jobLog = ref(null)
const bindForm = ref({ envId: '', sshCredId: '', script: '', workDir: '' })
const showBind = ref(false)

const id = computed(() => String(route.params.id || ''))

async function load() {
  err.value = ''
  try {
    const data = await readApi.project(id.value)
    project.value = data?.project || null
    bindings.value = Array.isArray(data?.envs) ? data.envs : []
    const e = await readApi.environments()
    envs.value = Array.isArray(e?.items) ? e.items : []
    const c = await readApi.credentials()
    sshCreds.value = (Array.isArray(c?.items) ? c.items : []).filter((x) => x.type === 'ssh')
  } catch (e) {
    err.value = e?.message || '加载失败'
  }
}

function envName(envId) {
  return envs.value.find((e) => e.id === envId)?.name || envId
}

function openBind(pe) {
  if (pe) {
    bindForm.value = {
      envId: pe.envId,
      sshCredId: pe.sshCredId || '',
      script: pe.script || '',
      workDir: pe.workDir || ''
    }
  } else {
    bindForm.value = { envId: '', sshCredId: '', script: 'echo deploy && hostname', workDir: '' }
  }
  showBind.value = true
}

async function saveBind() {
  busy.value = true
  try {
    await writeApi.upsertProjectEnv({
      projectId: id.value,
      envId: bindForm.value.envId,
      sshCredId: bindForm.value.sshCredId,
      script: bindForm.value.script,
      workDir: bindForm.value.workDir
    })
    showBind.value = false
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
        <h1>{{ project?.name || '项目' }}</h1>
        <p class="page-desc">{{ project?.description || '简单发布：绑定环境与脚本，SSH 到目标机执行。' }}</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" @click="router.push('/')">返回</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openBind(null)">绑定环境</button>
      </div>
    </div>
    <p v-if="err" class="ops-msg is-err">{{ err }}</p>

    <div v-if="!bindings.length" class="empty">尚未绑定环境</div>
    <div v-else class="capsule-grid">
      <article v-for="pe in bindings" :key="pe.id" class="project-capsule" style="cursor:default">
        <h2 class="capsule-title">{{ envName(pe.envId) }}</h2>
        <div class="capsule-meta">
          <span v-if="pe.lastStatus" class="status-pill" :class="pe.lastStatus">{{ pe.lastStatus }}</span>
          <span v-if="pe.lastDeployAt">{{ new Date(pe.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</span>
          <span v-if="pe.workDir">dir: {{ pe.workDir }}</span>
        </div>
        <pre class="muted" style="margin:0;font-size:12px;white-space:pre-wrap;max-height:4.5em;overflow:hidden">{{ pe.script || '（无脚本）' }}</pre>
        <div class="actions" style="margin-top:0">
          <button type="button" class="btn btn-sm btn-primary" :disabled="busy" @click="deploy(pe)">发布</button>
          <button v-if="isSuper" type="button" class="btn btn-sm" @click="openBind(pe)">编辑</button>
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
          <select v-model="bindForm.envId">
            <option value="">选择环境</option>
            <option v-for="e in envs" :key="e.id" :value="e.id">{{ e.name }} ({{ e.kind }})</option>
          </select>
        </div>
        <div class="form-row">
          <label>SSH 凭据</label>
          <select v-model="bindForm.sshCredId">
            <option value="">选择凭据</option>
            <option v-for="c in sshCreds" :key="c.id" :value="c.id">{{ c.name }} · {{ c.host }}</option>
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
