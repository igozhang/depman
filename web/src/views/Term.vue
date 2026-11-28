<script setup>
import { onMounted, ref } from 'vue'
import { readApi, writeApi } from '../api'
import { canWrite } from '../stores/opsAuth'
import { useRouter } from 'vue-router'

const router = useRouter()
const creds = ref([])
const selected = ref('')
const msg = ref('')
const ok = ref(null)

async function load() {
  try {
    const c = await readApi.credentials()
    creds.value = (Array.isArray(c?.items) ? c.items : []).filter((x) => x.type === 'ssh')
  } catch (e) {
    msg.value = e?.message || '加载失败'
    ok.value = false
  }
}

async function test() {
  if (!canWrite.value) {
    router.push('/ops')
    return
  }
  if (!selected.value) {
    msg.value = '请选择 SSH 凭据'
    ok.value = false
    return
  }
  try {
    const r = await writeApi.testSSH(selected.value)
    ok.value = !!r?.ok
    msg.value = r?.ok ? 'SSH 连通成功' : r?.message || r?.error || '失败'
  } catch (e) {
    ok.value = false
    msg.value = e?.message || '测试失败'
  }
}

onMounted(load)
</script>

<template>
  <div>
    <div class="toolbar">
      <div>
        <h1>交互</h1>
        <p class="page-desc">与服务器 SSH 交互。当前提供连通测试；Web 终端（xterm）后续接入。</p>
      </div>
    </div>
    <div class="panel">
      <div class="form-row">
        <label>SSH 凭据</label>
        <select v-model="selected">
          <option value="">选择</option>
          <option v-for="c in creds" :key="c.id" :value="c.id">{{ c.name }} · {{ c.host }}</option>
        </select>
      </div>
      <div class="toolbar toolbar-start">
        <button type="button" class="btn btn-primary" @click="test">测试连通</button>
      </div>
      <p class="ops-msg" :class="{ 'is-ok': ok === true, 'is-err': ok === false }">{{ msg }}</p>
    </div>
  </div>
</template>
