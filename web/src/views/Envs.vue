<script setup>
import { computed, onMounted, ref } from 'vue'
import { readApi, writeApi } from '../api'
import { canWrite, isSuper } from '../stores/opsAuth'

const envs = ref([])
const creds = ref([])
const err = ref('')
const filterKind = ref('')
const showEnv = ref(false)
const showCred = ref(false)
const reveal = ref(false)
const envForm = ref({ name: '', kind: 'dev', description: '' })
const credForm = ref({
  envId: '',
  name: '',
  type: 'ssh',
  host: '',
  port: 22,
  username: '',
  secret: '',
  extra: ''
})

const kinds = [
  { value: '', label: '全部' },
  { value: 'dev', label: 'dev' },
  { value: 'test', label: 'test' },
  { value: 'staging', label: 'staging' },
  { value: 'prod', label: 'prod' },
  { value: 'other', label: 'other' }
]

const filteredEnvs = computed(() => {
  if (!filterKind.value) return envs.value
  return envs.value.filter((e) => e.kind === filterKind.value)
})

function credsOf(envId) {
  return creds.value.filter((c) => c.envId === envId)
}

async function load() {
  err.value = ''
  try {
    const e = await readApi.environments()
    envs.value = Array.isArray(e?.items) ? e.items : []
    const c = await readApi.credentials('', reveal.value && isSuper.value)
    creds.value = Array.isArray(c?.items) ? c.items : []
  } catch (ex) {
    err.value = ex?.message || '加载失败'
  }
}

async function toggleReveal() {
  if (!isSuper.value) return
  reveal.value = !reveal.value
  await load()
}

async function saveEnv() {
  try {
    await writeApi.createEnvironment(envForm.value)
    showEnv.value = false
    envForm.value = { name: '', kind: 'dev', description: '' }
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
    const body = { ...credForm.value, port: Number(credForm.value.port) || 0 }
    await writeApi.createCredential(body)
    showCred.value = false
    credForm.value = { envId: '', name: '', type: 'ssh', host: '', port: 22, username: '', secret: '', extra: '' }
    await load()
  } catch (e) {
    err.value = e?.message || '保存失败'
  }
}

async function removeCred(id) {
  if (!confirm('删除凭据？')) return
  try {
    await writeApi.deleteCredential(id)
    await load()
  } catch (e) {
    err.value = e?.message || '删除失败'
  }
}

async function testSSH(id) {
  try {
    const r = await writeApi.testSSH(id)
    alert(r?.ok ? '连通成功' : `失败：${r?.error || ''}`)
  } catch (e) {
    err.value = e?.message || '测试失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>环境</h1>
        <p class="page-desc">按环境管理 Git / Harbor / SSH / API 授权信息；密码默认掩码。</p>
      </div>
      <div class="actions">
        <button type="button" class="btn" @click="load">刷新</button>
        <button v-if="isSuper" type="button" class="btn" @click="toggleReveal">
          {{ reveal ? '隐藏密钥' : '显示密钥' }}
        </button>
        <button v-if="isSuper" type="button" class="btn" @click="showCred = true">添加凭据</button>
        <button v-if="isSuper" type="button" class="btn btn-primary" @click="showEnv = true">新建环境</button>
      </div>
    </div>

    <div class="toolbar toolbar-start" style="margin-bottom:0.75rem">
      <label class="muted">分类</label>
      <select v-model="filterKind">
        <option v-for="k in kinds" :key="k.value" :value="k.value">{{ k.label }}</option>
      </select>
    </div>

    <p v-if="err" class="ops-msg is-err">{{ err }}</p>

    <div v-if="!filteredEnvs.length" class="empty">暂无环境</div>
    <div v-for="env in filteredEnvs" :key="env.id" class="panel">
      <div class="toolbar">
        <div>
          <strong>{{ env.name }}</strong>
          <span class="status-pill" style="margin-left:8px">{{ env.kind }}</span>
          <p class="muted" style="margin:0.25rem 0 0">{{ env.description || '—' }}</p>
        </div>
        <div v-if="isSuper" class="actions">
          <button type="button" class="btn btn-sm btn-danger" @click="removeEnv(env.id)">删除</button>
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
            <th></th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="c in credsOf(env.id)" :key="c.id">
            <td>{{ c.name }}</td>
            <td>{{ c.type }}</td>
            <td>{{ c.host }}{{ c.port ? ':' + c.port : '' }}</td>
            <td>{{ c.username || '—' }}</td>
            <td>
              <code v-if="reveal && c.secret">{{ c.secret }}</code>
              <span v-else>{{ c.secretSet ? '••••••••' : '（无）' }}</span>
            </td>
            <td class="actions">
              <button v-if="canWrite && c.type === 'ssh'" type="button" class="btn btn-sm" @click="testSSH(c.id)">测试</button>
              <button v-if="isSuper" type="button" class="btn btn-sm btn-danger" @click="removeCred(c.id)">删</button>
            </td>
          </tr>
          <tr v-if="!credsOf(env.id).length">
            <td colspan="6" class="muted">无凭据</td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="showEnv" class="drawer-mask" @click.self="showEnv = false">
      <div class="drawer">
        <div class="drawer-head">
          <h2 style="margin:0;font-size:1.1rem">新建环境</h2>
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
            <option value="test">test</option>
            <option value="staging">staging</option>
            <option value="prod">prod</option>
            <option value="other">other</option>
          </select>
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
          <h2 style="margin:0;font-size:1.1rem">添加凭据</h2>
          <button type="button" class="btn btn-sm" @click="showCred = false">关闭</button>
        </div>
        <div class="form-row">
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
              <option value="ssh">ssh</option>
              <option value="git">git</option>
              <option value="harbor">harbor</option>
              <option value="api">api</option>
            </select>
          </div>
        </div>
        <div class="form-grid">
          <div class="form-row">
            <label>主机</label>
            <input v-model="credForm.host" />
          </div>
          <div class="form-row">
            <label>端口</label>
            <input v-model="credForm.port" type="number" />
          </div>
        </div>
        <div class="form-row">
          <label>用户名</label>
          <input v-model="credForm.username" />
        </div>
        <div class="form-row">
          <label>密码 / 私钥 / Token</label>
          <textarea v-model="credForm.secret" rows="4" placeholder="不明文列表展示；含 PRIVATE KEY 则按私钥认证" />
        </div>
        <div class="actions">
          <button type="button" class="btn btn-primary" @click="saveCred">保存</button>
        </div>
      </div>
    </div>
  </div>
</template>
