<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper } from '../stores/opsAuth'

const TAG_MAX = 15

const envs = ref([])
const allTags = ref([])
const creds = ref([])
const resolves = ref([])
const err = ref('')
const filterKind = ref('')
const showEnv = ref(false)
const showCred = ref(false)
const showResolve = ref(false)
const reveal = ref(false)
const editingId = ref('')
const editingCredId = ref('')
const editingResolveId = ref('')
const tagDraft = ref('')
/** @type {import('vue').Ref<'shared'|'env'>} */
const credScope = ref('shared')
const envForm = ref({ name: '', kind: 'dev', description: '', tags: [] })
const credForm = ref(emptyCredForm())
const resolveForm = ref(emptyResolveForm())
/** @type {import('vue').Ref<Record<string, { ok: boolean|null, msg: string, busy?: boolean }>>} */
const testStatus = ref({})
let skipPortReset = false

function emptyCredForm() {
  return {
    envId: '',
    name: '',
    type: 'git',
    host: '',
    port: 0,
    username: '',
    secret: '',
    extra: '',
    authCredId: ''
  }
}

function emptyResolveForm() {
  return { hostname: '', ip: '', note: '' }
}

/** Port defaults by type; 0 = protocol default */
function defaultPortForType(type) {
  switch (type) {
    case 'ssh':
      return 22
    case 'harbor':
    case 'api':
      return 443
    case 'git':
      return 0
    default:
      return 0
  }
}

watch(
  () => credForm.value.type,
  (type) => {
    if (skipPortReset) return
    credForm.value.port = defaultPortForType(type)
    if (credForm.value.authCredId) {
      const ok = authOptions.value.some((c) => c.id === credForm.value.authCredId)
      if (!ok) credForm.value.authCredId = ''
    }
  }
)

const kinds = [
  { value: '', label: '全部' },
  { value: 'dev', label: 'dev' },
  { value: 'uat', label: 'uat' },
  { value: 'pro', label: 'pro' }
]

const filteredEnvs = computed(() => {
  if (!filterKind.value) return envs.value
  return envs.value.filter((e) => e.kind === filterKind.value)
})

const unusedTags = computed(() => {
  const selected = new Set((envForm.value.tags || []).map((t) => t.toLowerCase()))
  return allTags.value.filter((t) => !selected.has(String(t.name).toLowerCase()))
})

const sharedCreds = computed(() => creds.value.filter((c) => !c.envId))

const envDrawerTitle = computed(() => (editingId.value ? '编辑环境' : '新建环境'))
const credDrawerTitle = computed(() => {
  if (editingCredId.value) {
    return credScope.value === 'shared' ? '编辑公共凭据' : '编辑环境凭据'
  }
  return credScope.value === 'shared' ? '添加公共凭据' : '添加环境凭据'
})
const resolveDrawerTitle = computed(() => (editingResolveId.value ? '编辑解析' : '添加解析'))

const usesAuthRef = computed(() => credScope.value === 'env')

const authOptions = computed(() => {
  const typ = credForm.value.type
  const envId = credForm.value.envId
  return creds.value.filter((c) => {
    if (c.type !== typ) return false
    if (c.authCredId) return false
    if (editingCredId.value && c.id === editingCredId.value) return false
    return !c.envId || c.envId === envId
  })
})

const hideLocalSecret = computed(() => !!credForm.value.authCredId)

function credsOf(envId) {
  return creds.value.filter((c) => c.envId === envId)
}

function credName(id) {
  if (!id) return ''
  return creds.value.find((c) => c.id === id)?.name || id
}

/** Show effective dial target when port is protocol-default (esp. git → https/443). */
function formatCredEndpoint(c) {
  const host = c?.host || ''
  if (c?.port) return `${host}:${c.port}`
  if (
    c?.type === 'git' &&
    host &&
    !String(host).includes('://') &&
    !String(host).startsWith('git@')
  ) {
    return `${host}（默认 https/443）`
  }
  return host
}

function emptyEnvForm() {
  return { name: '', kind: 'dev', description: '', tags: [] }
}

async function load() {
  err.value = ''
  try {
    const e = await readApi.environments()
    envs.value = Array.isArray(e?.items) ? e.items : []
    const t = await readApi.tags()
    allTags.value = Array.isArray(t?.items) ? t.items : []
    const c = await readApi.credentials('', reveal.value && isSuper.value)
    creds.value = Array.isArray(c?.items) ? c.items : []
    const r = await readApi.hostResolves()
    resolves.value = Array.isArray(r?.items) ? r.items : []
  } catch (ex) {
    err.value = ex?.message || '加载失败'
  }
}

async function toggleReveal() {
  if (!isSuper.value) return
  reveal.value = !reveal.value
  await load()
}

function openCreateSharedCred() {
  editingCredId.value = ''
  credScope.value = 'shared'
  skipPortReset = true
  credForm.value = { ...emptyCredForm(), type: 'git', port: 0 }
  showCred.value = true
  nextTick(() => {
    skipPortReset = false
  })
}

function openCreateEnvCred(env) {
  editingCredId.value = ''
  credScope.value = 'env'
  skipPortReset = true
  credForm.value = {
    ...emptyCredForm(),
    envId: env.id,
    type: 'ssh',
    port: 22
  }
  showCred.value = true
  nextTick(() => {
    skipPortReset = false
  })
}

function openEditCred(c) {
  editingCredId.value = c.id
  credScope.value = c.envId ? 'env' : 'shared'
  skipPortReset = true
  credForm.value = {
    envId: c.envId || '',
    name: c.name || '',
    type: c.type || 'ssh',
    host: c.host || '',
    port: c.port ?? defaultPortForType(c.type || 'ssh'),
    username: c.username || '',
    secret: '',
    extra: c.extra || '',
    authCredId: c.authCredId || ''
  }
  showCred.value = true
  nextTick(() => {
    skipPortReset = false
  })
}

function openCreateEnv() {
  editingId.value = ''
  envForm.value = emptyEnvForm()
  tagDraft.value = ''
  showEnv.value = true
}

function openEditEnv(env) {
  editingId.value = env.id
  envForm.value = {
    name: env.name || '',
    kind: env.kind || 'dev',
    description: env.description || '',
    tags: Array.isArray(env.tags) ? [...env.tags] : []
  }
  tagDraft.value = ''
  showEnv.value = true
}

function normalizeTag(raw) {
  return String(raw || '').trim()
}

function addTag(raw) {
  const name = normalizeTag(raw)
  if (!name) return
  if ([...name].length > TAG_MAX) {
    err.value = '标签须小于 16 个字符'
    return
  }
  const tags = envForm.value.tags || []
  if (tags.some((t) => t.toLowerCase() === name.toLowerCase())) {
    tagDraft.value = ''
    return
  }
  envForm.value.tags = [...tags, name]
  tagDraft.value = ''
  err.value = ''
}

function removeTag(name) {
  envForm.value.tags = (envForm.value.tags || []).filter((t) => t !== name)
}

function onTagKeydown(e) {
  if (e.key === 'Enter' || e.key === ',') {
    e.preventDefault()
    addTag(tagDraft.value.replace(/,/g, ''))
  } else if (e.key === 'Backspace' && !tagDraft.value && envForm.value.tags?.length) {
    removeTag(envForm.value.tags[envForm.value.tags.length - 1])
  }
}

async function saveEnv() {
  try {
    const draft = normalizeTag(tagDraft.value)
    if (draft) addTag(draft)
    const body = {
      name: envForm.value.name,
      kind: envForm.value.kind,
      description: envForm.value.description,
      tags: [...(envForm.value.tags || [])]
    }
    if (editingId.value) {
      await writeApi.updateEnvironment(editingId.value, body)
    } else {
      await writeApi.createEnvironment(body)
    }
    showEnv.value = false
    editingId.value = ''
    envForm.value = emptyEnvForm()
    tagDraft.value = ''
    await load()
  } catch (e) {
    err.value = e?.message || '保存失败'
  }
}

async function removeEnv(id) {
  if (!confirm('删除环境及其凭据？')) return
  try {
    await writeApi.deleteEnvironment(id)
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  }
}

async function saveCred() {
  try {
    const body = {
      ...credForm.value,
      envId: credScope.value === 'shared' ? '' : credForm.value.envId,
      port: Number(credForm.value.port) || 0,
      authCredId: credForm.value.authCredId || ''
    }
    if (credScope.value === 'env' && !body.envId) {
      err.value = '请选择环境'
      return
    }
    if (body.authCredId) {
      body.username = ''
      body.secret = ''
    }
    if (editingCredId.value) {
      if (!body.secret) delete body.secret
      await writeApi.updateCredential(editingCredId.value, body)
    } else {
      await writeApi.createCredential(body)
    }
    showCred.value = false
    editingCredId.value = ''
    credForm.value = emptyCredForm()
    await load()
  } catch (e) {
    err.value = e?.message || '保存失败'
  }
}

async function removeCred(id) {
  if (!confirm('删除凭据？')) return
  try {
    await writeApi.deleteCredential(id)
    const next = { ...testStatus.value }
    delete next[id]
    testStatus.value = next
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  }
}

async function testCred(c) {
  if (!canWrite.value) return
  testStatus.value = {
    ...testStatus.value,
    [c.id]: { ok: null, msg: '测试中…', busy: true }
  }
  try {
    const r = await writeApi.pingCredential(c.id)
    const ok = !!r?.ok
    const msg = ok ? '连通成功' : r?.message || r?.error || '失败'
    testStatus.value = {
      ...testStatus.value,
      [c.id]: { ok, msg, busy: false }
    }
  } catch (e) {
    const raw = e?.message || '测试失败'
    const timedOut = /timeout|deadline|context canceled|超时/i.test(raw)
    testStatus.value = {
      ...testStatus.value,
      [c.id]: { ok: false, msg: timedOut ? '超时（5s）' : raw, busy: false }
    }
  }
}

function openCreateResolve() {
  editingResolveId.value = ''
  resolveForm.value = emptyResolveForm()
  showResolve.value = true
}

function openEditResolve(r) {
  editingResolveId.value = r.id
  resolveForm.value = {
    hostname: r.hostname || '',
    ip: r.ip || '',
    note: r.note || ''
  }
  showResolve.value = true
}

async function saveResolve() {
  try {
    const body = {
      hostname: resolveForm.value.hostname,
      ip: resolveForm.value.ip,
      note: resolveForm.value.note
    }
    if (editingResolveId.value) {
      await writeApi.updateHostResolve(editingResolveId.value, body)
    } else {
      await writeApi.createHostResolve(body)
    }
    showResolve.value = false
    editingResolveId.value = ''
    resolveForm.value = emptyResolveForm()
    await load()
  } catch (e) {
    err.value = e?.message || '保存失败'
  }
}

async function removeResolve(id) {
  if (!confirm('删除该主机解析？')) return
  try {
    await writeApi.deleteHostResolve(id)
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>环境</h1>
        <p class="page-desc">维护可发布的目标环境与凭据（与项目里的「发布任务」配合使用）。密码默认掩码；</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" @click="load">刷新</button>
        <button v-if="isSuper" type="button" class="btn" @click="toggleReveal">
          {{ reveal ? '隐藏密钥' : '显示密钥' }}
        </button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="openCreateEnv">新建环境</button>
      </div>
    </div>

    <div class="toolbar toolbar-start" style="margin-bottom:0.75rem">
      <label class="muted">分类</label>
      <select v-model="filterKind">
        <option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      </select>
    </div>

    <p v-if="err" class="ops-msg is-err">{{ err }}</p>

    <div class="panel">
      <div class="toolbar">
        <div>
          <strong>主机解析</strong>
          <p class="muted" style="margin:0.25rem 0 0">写入本服务所有 Pod 的 hosts，供凭据连通与发布使用</p>
        </div>
        <div v-if="isSuper" class="actions">
          <button type="button" class="btn btn-sm" @click="openCreateResolve">添加解析</button>
        </div>
      </div>
      <table class="data">
        <thead>
          <tr>
            <th>主机名</th>
            <th>IP</th>
            <th>备注</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="r in resolves" :key="r.id">
            <td><code>{{ r.hostname }}</code></td>
            <td>{{ r.ip }}</td>
            <td>{{ r.note || '—' }}</td>
            <td class="actions">
              <button v-if="isSuper" type="button" class="btn btn-sm" @click="openEditResolve(r)">编辑</button>
              <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" @click="removeResolve(r.id)">删</button>
            </td>
          </tr>
          <tr v-if="!resolves.length">
            <td colspan="4" class="muted">暂无主机解析</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div class="panel">
      <div class="toolbar">
        <div>
          <strong>公共凭据</strong>
          <p class="muted" style="margin:0.25rem 0 0">全局认证池（Git / Harbor / Rancher / SSH），可被各环境引用</p>
        </div>
        <div v-if="isSuper" class="actions">
          <button type="button" class="btn btn-sm" @click="openCreateSharedCred">添加公共凭据</button>
        </div>
      </div>
      <table class="data">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>主机</th>
            <th>用户</th>
            <th>密钥</th>
            <th>连通</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in sharedCreds" :key="c.id">
            <td>{{ c.name }}</td>
            <td>{{ c.type }}</td>
            <td>{{ formatCredEndpoint(c) }}</td>
            <td>{{ c.username || '—' }}</td>
            <td>
              <code v-if="reveal && c.secret">{{ c.secret }}</code>
              <span v-else>{{ c.secretSet ? '••••••••' : '（无）' }}</span>
            </td>
            <td>
              <span
                v-if="testStatus[c.id]"
                class="ops-msg"
                style="margin:0;display:inline"
                :class="{
                  'is-ok': testStatus[c.id].ok === true,
                  'is-err': testStatus[c.id].ok === false
                }"
              >{{ testStatus[c.id].msg }}</span>
              <span v-else class="muted">—</span>
            </td>
            <td class="actions">
              <button
                v-if="canWrite"
                type="button"
                class="btn btn-sm"
                :disabled="testStatus[c.id]?.busy"
                @click="testCred(c)"
              >测试</button>
              <button v-if="isSuper" type="button" class="btn btn-sm" @click="openEditCred(c)">编辑</button>
              <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" @click="removeCred(c.id)">删</button>
            </td>
          </tr>
          <tr v-if="!sharedCreds.length">
            <td colspan="7" class="muted">暂无公共凭据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="!filteredEnvs.length" class="empty">暂无环境</div>
    <div v-for="env in filteredEnvs" :key="env.id" class="panel">
      <div class="toolbar">
        <div>
          <strong>{{ env.name }}</strong>
          <span class="status-pill" style="margin-left:8px">{{ env.kind }}</span>
          <span v-if="env.tags?.length" class="tag-list" style="margin-left:8px">
            <span v-for="t in env.tags" :key="t" class="tag-chip">{{ t }}</span>
          </span>
          <p class="muted" style="margin:0.25rem 0 0">{{ env.description || '—' }}</p>
        </div>
        <div v-if="isSuper" class="actions">
          <button type="button" class="btn btn-sm" @click="openCreateEnvCred(env)">添加环境凭据</button>
          <button type="button" class="btn btn-sm" @click="openEditEnv(env)">编辑</button>
          <button type="button" class="btn btn-sm btn-danger" @click="removeEnv(env.id)">删除</button>
        </div>
      </div>
      <table class="data">
        <thead>
          <tr>
            <th>名称</th>
            <th>类型</th>
            <th>主机 / 目标</th>
            <th>认证</th>
            <th>密钥</th>
            <th>连通</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in credsOf(env.id)" :key="c.id">
            <td>{{ c.name }}</td>
            <td>{{ c.type }}</td>
            <td>{{ formatCredEndpoint(c) }}</td>
            <td>
              <template v-if="c.authCredId">引用 · {{ credName(c.authCredId) }}</template>
              <template v-else>{{ c.username || '—' }}</template>
            </td>
            <td>
              <template v-if="c.authCredId"><span class="muted">随认证凭据</span></template>
              <template v-else>
                <code v-if="reveal && c.secret">{{ c.secret }}</code>
                <span v-else>{{ c.secretSet ? '••••••••' : '（无）' }}</span>
              </template>
            </td>
            <td>
              <span
                v-if="testStatus[c.id]"
                class="ops-msg"
                style="margin:0;display:inline"
                :class="{
                  'is-ok': testStatus[c.id].ok === true,
                  'is-err': testStatus[c.id].ok === false
                }"
              >{{ testStatus[c.id].msg }}</span>
              <span v-else class="muted">—</span>
            </td>
            <td class="actions">
              <button
                v-if="canWrite"
                type="button"
                class="btn btn-sm"
                :disabled="testStatus[c.id]?.busy"
                @click="testCred(c)"
              >测试</button>
              <button v-if="isSuper" type="button" class="btn btn-sm" @click="openEditCred(c)">编辑</button>
              <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" @click="removeCred(c.id)">删</button>
            </td>
          </tr>
          <tr v-if="!credsOf(env.id).length">
            <td colspan="7" class="muted">无环境凭据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showEnv" class="drawer-mask" @click.self="showEnv = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">{{ envDrawerTitle }}</h2>
          <button type="button" class="btn btn-sm" @click="showEnv = false">关闭</button>
        </div>
        <div class="form-row">
          <label>名称</label>
          <input v-model="envForm.name" />
        </div>
        <div class="form-row">
          <label>分类</label>
          <select v-model="envForm.kind">
            <option value="dev">dev</option>
            <option value="uat">uat</option>
            <option value="pro">pro</option>
          </select>
        </div>
        <div class="form-row">
          <label>标签</label>
          <div class="tag-list" style="margin-bottom:6px" v-if="envForm.tags?.length">
            <span v-for="t in envForm.tags" :key="t" class="tag-chip">
              {{ t }}
              <button type="button" title="移除" @click="removeTag(t)">×</button>
            </span>
          </div>
          <div class="tag-input-row">
            <input
              v-model="tagDraft"
              maxlength="15"
              placeholder="选择或输入，回车添加（少于16字，全局最多30）"
              @keydown="onTagKeydown"
            />
            <button type="button" class="btn btn-sm" @click="addTag(tagDraft)">添加</button>
          </div>
          <div v-if="unusedTags.length" class="tag-suggest">
            <button
              v-for="t in unusedTags"
              :key="t.id"
              type="button"
              class="btn btn-sm"
              @click="addTag(t.name)"
            >
              {{ t.name }}
            </button>
          </div>
        </div>
        <div class="form-row">
          <label>描述</label>
          <textarea v-model="envForm.description" rows="2" />
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="saveEnv">保存</button>
        </div>
      </div>
    </div>

    <div v-if="showCred" class="drawer-mask" @click.self="showCred = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">{{ credDrawerTitle }}</h2>
          <button type="button" class="btn btn-sm" @click="showCred = false">关闭</button>
        </div>
        <p v-if="credScope === 'shared'" class="muted" style="margin:0 0 0.75rem">
          公共凭据只存服务主机与账号；具体仓库/集群目标在环境凭据里填写并引用此处认证。
        </p>
        <div v-if="credScope === 'env'" class="form-row">
          <label>环境</label>
          <select v-model="credForm.envId">
            <option value="">选择</option>
            <option v-for="e in envs" :key="e.id" :value="e.id">{{ e.name }}</option>
          </select>
        </div>
        <div class="form-grid">
          <div class="form-row">
            <label>名称</label>
            <input v-model="credForm.name" />
          </div>
          <div class="form-row">
            <label>类型</label>
            <select v-model="credForm.type">
              <option value="git">git</option>
              <option value="harbor">harbor</option>
              <option value="api">api</option>
              <option value="ssh">ssh</option>
            </select>
          </div>
        </div>
        <div class="form-grid">
          <div class="form-row">
            <label>{{ credScope === 'shared' ? '主机' : '主机 / 仓库地址' }}</label>
            <input
              v-model="credForm.host"
              :placeholder="
                credForm.type === 'git'
                  ? credScope === 'shared'
                    ? 'gitlab.example.com 或 http://… / https://…'
                    : 'https://gitlab.example.com/group/repo.git'
                  : ''
              "
            />
          </div>
          <div class="form-row">
            <label>端口</label>
            <input
              v-model="credForm.port"
              type="number"
              min="0"
              :placeholder="credForm.type === 'git' ? '0=https/443；HTTP 填 80' : ''"
            />
          </div>
        </div>
        <div v-if="usesAuthRef" class="form-row">
          <label>认证凭据</label>
          <select v-model="credForm.authCredId">
            <option value="">本条自带账号（不引用）</option>
            <option v-for="a in authOptions" :key="a.id" :value="a.id">
              {{ a.envId ? '[环境] ' : '[公共] ' }}{{ a.name }} · {{ a.host }}
            </option>
          </select>
          <p v-if="credForm.type === 'git'" class="muted" style="margin:0.35rem 0 0;font-size:0.85rem">
            主机填完整仓库 URL（HTTP 服务用 http://…，勿写 https 除非 443 可达）；认证选公共 Git 账号即可。
          </p>
        </div>
        <template v-if="!hideLocalSecret">
          <div class="form-row">
            <label>用户名</label>
            <input v-model="credForm.username" />
          </div>
          <div class="form-row">
            <label>密码 / 私钥 / Token</label>
            <textarea
              v-model="credForm.secret"
              rows="4"
              :placeholder="editingCredId ? '留空则保留原密钥；填写则覆盖' : '不明文列表展示；含 PRIVATE KEY 则按私钥认证'"
            />
          </div>
        </template>
        <p v-else class="muted" style="margin:0 0 0.75rem">已引用认证凭据，用户名与密钥由其提供。</p>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="saveCred">保存</button>
        </div>
      </div>
    </div>

    <div v-if="showResolve" class="drawer-mask" @click.self="showResolve = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">{{ resolveDrawerTitle }}</h2>
          <button type="button" class="btn btn-sm" @click="showResolve = false">关闭</button>
        </div>
        <p class="muted" style="margin:0 0 0.75rem">保存后写入本服务 Pod 的 /etc/hosts，重启后也会自动同步。</p>
        <div class="form-row">
          <label>主机名</label>
          <input v-model="resolveForm.hostname" placeholder="gitlab.example.com" />
        </div>
        <div class="form-row">
          <label>IP</label>
          <input v-model="resolveForm.ip" placeholder="10.0.0.1" />
        </div>
        <div class="form-row">
          <label>备注</label>
          <input v-model="resolveForm.note" placeholder="可选" />
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="saveResolve">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
