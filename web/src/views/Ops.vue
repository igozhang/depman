<script setup>
import { computed, onMounted, ref, watch } from 'vue'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper, remainingSec, fmtBeijing, logout, useOpsAuth } from '../stores/opsAuth'

const { state } = useOpsAuth()
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

function onLogout() {
  logout()
  scoped.value = []
  issuedPlain.value = ''
  msg.value = '已退出，当前为只读'
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
    <div class="toolbar">
      <div>
        <h1>登录</h1>
        <p class="page-desc">默认只读。双击页眉图标下方「只读」在右侧抽屉输入 key；已登录可双击「超管」退出。超管可在此签发项目管理 key。</p>
      </div>
    </div>

    <div class="panel ops-panel">
      <template v-if="!canWrite">
        <p class="muted">当前只读。请双击页眉图标下「只读」打开抽屉验证。</p>
      </template>
      <template v-else>
        <p class="notice ops-ok">已验证（{{ state.session?.label || (isSuper ? '超管' : '可写') }}）。</p>
        <p class="muted ops-expiry" :class="{ 'is-warn': expiryWarn }">{{ expiryText }}</p>
        <div class="toolbar toolbar-start">
          <button type="button" class="btn" @click="onLogout">退出</button>
        </div>
      </template>
      <p v-if="msg" class="ops-msg" :class="{ 'is-ok': msgOk === true, 'is-err': msgOk === false }">{{ msg }}</p>
    </div>

    <div v-if="isSuper" class="panel">
      <div class="toolbar">
        <div>
          <h2 style="margin:0;font-size:1.05rem">签发项目管理 key</h2>
          <p class="page-desc">绑定项目与环境；留空表示该维度不限。明文仅创建时显示一次。</p>
        </div>
      </div>
      <div class="form-grid">
        <div class="form-row">
          <label>标签</label>
          <input v-model="issue.label" placeholder="如 前端组" />
        </div>
        <div class="form-row">
          <label>有效小时</label>
          <input v-model.number="issue.ttlHours" type="number" min="1" max="168" />
        </div>
      </div>
      <div class="form-row">
        <label>项目（可多选）</label>
        <div class="kv-row">
          <label v-for="p in projects" :key="p.id" class="status-pill" style="cursor:pointer">
            <input
              type="checkbox"
              :checked="issue.projectIds.includes(p.id)"
              @change="toggleId(issue.projectIds, p.id)"
            />
            {{ p.name }}
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
        新 key（请立即复制）：<code>{{ issuedPlain }}</code>
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
              <button type="button" class="btn btn-sm btn-danger" @click="removeScoped(k.id)">撤销</button>
            </td>
          </tr>
          <tr v-if="!scoped.length">
            <td colspan="4" class="muted">暂无作用域 key</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>
