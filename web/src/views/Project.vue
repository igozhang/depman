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
const bindForm = ref({ name: '', envId: '', sshCredId: '', script: '', workDir: '' })
const showBind = ref(false)
const editingPeId = ref('')
const showHistory = ref(false)
const historyPe = ref(null)
const historyItems = ref([])
const historyTotal = ref(0)
const historyKept = ref(30)
const historyBusy = ref(false)
const historyExpanded = ref('')
const historyWidth = ref(560)
const historyDragging = ref(false)

const id = computed(() => String(route.params.id || ''))

/** 绑定抽屉只展示所选环境的 SSH 凭据，避免选错环境时报后端校验错误 */
const bindSshCreds = computed(() => {
  const envId = bindForm.value.envId
  const list = sshCreds.value
  if (!envId) return list
  return list.filter((c) => c.envId === envId)
})

function historyBtnClass(status) {
  if (status === 'ok') return 'btn-history-ok'
  if (status === 'warn') return 'btn-history-warn'
  if (status === 'failed') return 'btn-history-failed'
  return ''
}

function hasStatus(status) {
  return status === 'ok' || status === 'warn' || status === 'failed' || status === 'running' || status === 'pending'
}

function statusClass(status) {
  if (status === 'ok') return 'ok'
  if (status === 'warn') return 'warn'
  if (status === 'failed') return 'failed'
  if (status === 'running') return 'running'
  if (status === 'pending') return 'pending'
  return ''
}

function statusLabel(status) {
  if (status === 'ok') return '成功'
  if (status === 'warn') return '成功(有报错)'
  if (status === 'failed') return '失败'
  if (status === 'running') return '进行中'
  if (status === 'pending') return '排队'
  return status || '—'
}

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

function taskTitle(pe) {
  return pe?.name || envName(pe?.envId) || '发布任务'
}

function openBind(pe) {
  if (pe) {
    editingPeId.value = pe.id
    bindForm.value = {
      name: pe.name || '',
      envId: pe.envId,
      sshCredId: pe.sshCredId || '',
      script: pe.script || '',
      workDir: pe.workDir || ''
    }
  } else {
    editingPeId.value = ''
    bindForm.value = { name: '', envId: '', sshCredId: '', script: 'echo deploy && hostname', workDir: '' }
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
  err.value = ''
  if (!String(bindForm.value.name || '').trim()) {
    err.value = '请填写任务名称'
    return
  }
  if (!bindForm.value.envId) {
    err.value = '请选择目标环境'
    return
  }
  if (!bindForm.value.sshCredId) {
    err.value = '请选择 SSH 凭据'
    return
  }
  if (!String(bindForm.value.script || '').trim()) {
    err.value = '请填写发布脚本'
    return
  }
  busy.value = true
  try {
    const body = {
      projectId: id.value,
      name: bindForm.value.name,
      envId: bindForm.value.envId,
      sshCredId: bindForm.value.sshCredId,
      script: bindForm.value.script,
      workDir: bindForm.value.workDir
    }
    if (editingPeId.value) body.id = editingPeId.value
    await writeApi.upsertProjectEnv(body)
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
  if (!confirm(`删除发布任务「${taskTitle(pe)}」？`)) return
  busy.value = true
  err.value = ''
  try {
    await writeApi.deleteProjectEnv(pe.id)
    if (historyPe.value?.id === pe.id) {
      showHistory.value = false
      historyPe.value = null
    }
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  } finally {
    busy.value = false
  }
}

async function openHistory(pe) {
  historyPe.value = pe
  showHistory.value = true
  historyExpanded.value = ''
  historyBusy.value = true
  err.value = ''
  try {
    const data = await readApi.projectEnvJobs(pe.id)
    historyItems.value = Array.isArray(data?.items) ? data.items : []
    historyTotal.value = Number(data?.totalCount) || historyItems.value.length
    historyKept.value = Number(data?.kept) || 30
    if (historyItems.value.length) {
      historyExpanded.value = historyItems.value[0].id
    }
  } catch (e) {
    err.value = e?.message || '加载历史失败'
    historyItems.value = []
  } finally {
    historyBusy.value = false
  }
}

function toggleHistoryItem(id) {
  historyExpanded.value = historyExpanded.value === id ? '' : id
}

function onHistoryResizeStart(e) {
  e.preventDefault()
  historyDragging.value = true
  const startX = e.clientX
  const startW = historyWidth.value
  const onMove = (ev) => {
    const next = Math.min(900, Math.max(360, startW + (startX - ev.clientX)))
    historyWidth.value = next
  }
  const onUp = () => {
    historyDragging.value = false
    window.removeEventListener('mousemove', onMove)
    window.removeEventListener('mouseup', onUp)
  }
  window.addEventListener('mousemove', onMove)
  window.addEventListener('mouseup', onUp)
}

let pollTimer = 0
function pollJob(jobId) {
  window.clearInterval(pollTimer)
  const tick = async () => {
    try {
      const j = await readApi.job(jobId)
      jobLog.value = j
      if (j.status === 'ok' || j.status === 'warn' || j.status === 'failed') {
        window.clearInterval(pollTimer)
        await load()
        if (showHistory.value && historyPe.value) {
          await openHistory(historyPe.value)
        }
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
        <p class="page-desc">{{ project?.description || '为本项目添加发布任务：名称、目标环境、凭据与脚本，SSH 一键执行。同一环境可配置多条任务。' }}</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" @click="router.push('/')">返回</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openBind(null)">添加发布任务</button>
      </div>
    </div>
    <p v-if="err" class="ops-msg is-err">{{ err }}</p>

    <div class="toolbar toolbar-start" style="margin-bottom:0.65rem">
      <strong>发布任务</strong>
      <span class="muted">每条任务独立配置；同一环境可有多条</span>
    </div>
    <div v-if="!bindings.length" class="empty">尚未添加发布任务</div>
    <div v-else class="capsule-grid capsule-grid-bind">
      <article v-for="pe in bindings" :key="pe.id" class="project-capsule project-capsule-bind">
        <h2 class="capsule-title">{{ taskTitle(pe) }}</h2>
        <div class="capsule-body">
          <div class="capsule-meta">
            <span v-if="envName(pe.envId) !== taskTitle(pe)" class="capsule-env">{{ envName(pe.envId) }}</span>
            <time v-if="pe.lastDeployAt" class="capsule-time">{{ new Date(pe.lastDeployAt).toLocaleString('zh-CN', { hour12: false }) }}</time>
            <span v-else class="capsule-time capsule-time-empty">尚未发布</span>
          </div>
          <pre class="capsule-script">{{ pe.workDir ? `dir: ${pe.workDir}\n` : '' }}{{ pe.script || '（无脚本）' }}</pre>
        </div>
        <div class="capsule-foot capsule-actions">
          <div class="capsule-btns">
            <button type="button" class="btn btn-sm btn-primary" :disabled="busy" @click="deploy(pe)">发布</button>
            <button v-if="isSuper" type="button" class="btn btn-sm" :disabled="busy" @click="openBind(pe)">编辑</button>
            <button
              type="button"
              class="btn btn-sm"
              :class="historyBtnClass(pe.lastStatus)"
              :disabled="historyBusy"
              :title="pe.lastStatus ? `上次：${statusLabel(pe.lastStatus)}` : '暂无发布记录'"
              @click="openHistory(pe)"
            >历史</button>
            <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" :disabled="busy" @click="removeBind(pe)">删</button>
          </div>
          <span
            v-if="hasStatus(pe.lastStatus)"
            class="status-pill"
            :class="statusClass(pe.lastStatus)"
          >{{ statusLabel(pe.lastStatus) }}</span>
        </div>
      </article>
    </div>

    <div v-if="jobLog" class="panel" style="margin-top:1rem">
      <div class="toolbar toolbar-start">
        <strong>发布记录 · {{ statusLabel(jobLog.status) }}</strong>
        <span class="muted">{{ jobLog.id }}</span>
      </div>
      <div class="log-box">{{ jobLog.logTail || jobLog.error || '（等待输出…）' }}</div>
    </div>

    <div v-if="showHistory" class="drawer-mask" @click.self="showHistory = false">
      <div
        class="drawer drawer-history"
        :style="{ width: `min(${historyWidth}px, 100%)` }"
      >
        <div
          class="drawer-resize"
          :class="{ 'is-dragging': historyDragging }"
          title="拖动调整宽度"
          @mousedown="onHistoryResizeStart"
        />
        <div class="drawer-head">
          <div>
            <h2 style="margin:0;font-size:1.1rem">发布历史</h2>
            <p class="muted" style="margin:0.25rem 0 0;font-size:12px">
              {{ historyPe ? taskTitle(historyPe) : '' }}
              <template v-if="historyPe"> · {{ envName(historyPe.envId) }}</template>
              · 累计 {{ historyTotal }} 次
              · 保留最近 {{ historyKept }} 次输出
            </p>
          </div>
          <button type="button" class="btn btn-sm" @click="showHistory = false">关闭</button>
        </div>
        <p v-if="historyBusy" class="muted">加载中…</p>
        <div v-else-if="!historyItems.length" class="empty">暂无发布记录</div>
        <div v-else class="history-list">
          <div v-for="j in historyItems" :key="j.id" class="history-item">
            <div class="toolbar history-item-head" @click="toggleHistoryItem(j.id)">
              <span class="status-pill" :class="j.status">{{ statusLabel(j.status) }}</span>
              <span class="muted">{{ j.finishedAt || j.createdAt ? new Date(j.finishedAt || j.createdAt).toLocaleString('zh-CN', { hour12: false }) : '' }}</span>
              <span class="muted" style="margin-left:auto;font-size:12px">{{ historyExpanded === j.id ? '收起' : '展开' }}</span>
            </div>
            <div v-if="historyExpanded === j.id" class="log-box">{{ j.logTail || j.error || '（无输出）' }}</div>
          </div>
        </div>
      </div>
    </div>

    <div v-if="showBind" class="drawer-mask" @click.self="showBind = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">{{ editingPeId ? '编辑发布任务' : '添加发布任务' }}</h2>
          <button type="button" class="btn btn-sm" @click="showBind = false">关闭</button>
        </div>
        <div class="form-row">
          <label>名称</label>
          <input v-model="bindForm.name" placeholder="如：API 发布 / Web 回滚" />
        </div>
        <div class="form-row">
          <label>目标环境</label>
          <select v-model="bindForm.envId" @change="onBindEnvChange">
            <option value="">选择环境</option>
            <option v-for="e in envs" :key="e.id" :value="e.id">{{ e.name }} ({{ e.kind }})</option>
          </select>
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
