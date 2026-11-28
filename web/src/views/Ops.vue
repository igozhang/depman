<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper, remainingSec, fmtBeijing, login, logout, useOpsAuth } from '../stores/opsAuth'
import { copyText } from '../utils/clipboard'

const { state } = useOpsAuth()
const key = ref('')
const msg = ref('')
const msgOk = ref(null)
const scoped = ref([])
const projects = ref([])
const envs = ref([])
const issue = ref({ label: '', ttlHours: 24, projectIds: [], envIds: [] })
const issuedPlain = ref('')

const expiryText = computed(() => {
  if (!canWrite.value || !state.expiresAt) return ''
  const mins = Math.max(0, Math.ceil(remainingSec() / 60))
  const role = state.session?.label || (isSuper.value ? '超管' : '可写')
  return `${role} · 有效期至 ${fmtBeijing(state.expiresAt)}（剩余 ${mins} 分钟）`
})

const expiryWarn = computed(() => canWrite.value && remainingSec() < 5 * 60)

/** 签发 key 时项目列表：根在前、子任务缩进标注；勾选父项目即覆盖其子任务访问 */
const projectOptions = computed(() => {
  const list = projects.value
  const byId = new Map(list.map((p) => [p.id, p]))
  const roots = list.filter((p) => !p.parentId)
  const out = []
  for (const root of roots) {
    out.push({ id: root.id, label: root.name })
    for (const ch of list.filter((p) => p.parentId === root.id)) {
      out.push({ id: ch.id, label: `${root.name} / ${ch.name}` })
    }
  }
  // 父级不在列表中的孤儿子任务
  for (const p of list) {
    if (p.parentId && !byId.has(p.parentId) && !out.some((o) => o.id === p.id)) {
      out.push({ id: p.id, label: p.name })
    }
  }
  return out
})

async function submit() {
  const k = String(key.value || '').trim()
  if (!k || state.busy) return
  msg.value = ''
  msgOk.value = null
  try {
    await login(k)
    key.value = ''
    await loadScoped()
  } catch {
    /* silent */
  }
}

function onLogout() {
  logout()
  scoped.value = []
  issuedPlain.value = ''
  msg.value = ''
  msgOk.value = null
}

async function loadScoped() {
  if (!isSuper.value) {
    scoped.value = []
    return
  }
  try {
    const [sk, p, e] = await Promise.all([readApi.scopedKeys(), readApi.projects(), readApi.environments()])
    scoped.value = Array.isArray(sk?.items) ? sk.items : []
    projects.value = Array.isArray(p?.items) ? p.items : []
    envs.value = Array.isArray(e?.items) ? e.items : []
  } catch {
    scoped.value = []
  }
}

function toggleId(list, id) {
  const i = list.indexOf(id)
  if (i >= 0) list.splice(i, 1)
  else list.push(id)
}

async function createScoped() {
  issuedPlain.value = ''
  try {
    const k = await writeApi.createScopedKey({
      label: issue.value.label,
      ttlHours: Number(issue.value.ttlHours) || 24,
      projectIds: [...issue.value.projectIds],
      envIds: [...issue.value.envIds]
    })
    issuedPlain.value = k?.plainKey || ''
    await loadScoped()
  } catch (e) {
    msg.value = e?.message || '签发失败'
    msgOk.value = false
  }
}

async function copyScoped(id) {
  try {
    const data = await writeApi.scopedKeyPlain(id)
    const plain = String(data?.plainKey || '').trim()
    if (!plain) return
    await copyText(plain)
  } catch {
    /* silent */
  }
}

async function removeScoped(id) {
  if (!confirm('撤销该项目管理 key？')) return
  try {
    await writeApi.deleteScopedKey(id)
    await loadScoped()
  } catch (e) {
    msg.value = e?.message || '撤销失败'
    msgOk.value = false
  }
}

watch(isSuper, (v) => {
  if (v) loadScoped()
})

onMounted(() => {
  if (isSuper.value) loadScoped()
})
</script>

<template>
  <div>
    <div v-if="!canWrite" class="ops-login-center">
      <div class="toolbar toolbar-start ops-login-row">
        <input
          v-model="key"
          type="text"
          class="ops-key-input"
          autocomplete="off"
          spellcheck="false"
          @keydown.enter.prevent="submit"
        />
        <button type="button" class="btn btn-primary" :disabled="state.busy" @click="submit">
          {{ state.busy ? '…' : '登录' }}
        </button>
      </div>
    </div>

    <template v-else>
      <div class="panel ops-panel">
        <p class="notice ops-ok">{{ state.session?.label || (isSuper ? '超管' : '可写') }}</p>
        <p class="muted ops-expiry" :class="{ 'is-warn': expiryWarn }">{{ expiryText }}</p>
        <div class="toolbar toolbar-start">
          <button type="button" class="btn" @click="onLogout">退出</button>
        </div>
        <p v-if="msg" class="ops-msg" :class="{ 'is-ok': msgOk === true, 'is-err': msgOk === false }">{{ msg }}</p>
      </div>

      <div v-if="isSuper" class="panel">
        <div class="toolbar">
          <div>
            <h2 style="margin:0;font-size:1.05rem">签发项目管理 key</h2>
          </div>
        </div>
        <div class="form-grid">
          <div class="form-row">
            <label>标签</label>
            <input v-model="issue.label" />
          </div>
          <div class="form-row">
            <label>有效小时</label>
            <input v-model.number="issue.ttlHours" type="number" min="1" max="168" />
          </div>
        </div>
        <div class="form-row">
          <label>项目（可多选；勾选父项目即含子任务）</label>
          <div class="kv-row">
            <label v-for="p in projectOptions" :key="p.id" class="status-pill" style="cursor:pointer">
              <input
                type="checkbox"
                :checked="issue.projectIds.includes(p.id)"
                @change="toggleId(issue.projectIds, p.id)"
              />
              {{ p.label }}
            </label>
          </div>
        </div>
        <div class="form-row">
          <label>环境（可多选）</label>
          <div class="kv-row">
            <label v-for="e in envs" :key="e.id" class="status-pill" style="cursor:pointer">
              <input
                type="checkbox"
                :checked="issue.envIds.includes(e.id)"
                @change="toggleId(issue.envIds, e.id)"
              />
              {{ e.name }}
            </label>
          </div>
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="createScoped">签发</button>
        </div>
        <p v-if="issuedPlain" class="notice ops-ok">
          <code>{{ issuedPlain }}</code>
        </p>

        <table class="data" style="margin-top:1rem">
          <thead>
            <tr>
              <th>标签</th>
              <th>hint</th>
              <th>过期</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="k in scoped" :key="k.id">
              <td>{{ k.label }}</td>
              <td><code>{{ k.keyHint }}</code></td>
              <td>{{ new Date(k.expiresAt).toLocaleString('zh-CN', { hour12: false }) }}</td>
              <td class="actions">
                <button type="button" class="btn btn-sm" @click="copyScoped(k.id)">复制</button>
                <button type="button" class="btn btn-sm btn-danger" @click="removeScoped(k.id)">撤销</button>
              </td>
            </tr>
            <tr v-if="!scoped.length">
              <td colspan="4" class="muted">暂无作用域 key</td>
            </tr>
          </tbody>
        </table>
      </div>
    </template>
  </div>
</template>
