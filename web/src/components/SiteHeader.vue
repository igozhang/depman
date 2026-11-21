<script setup>
import { nextTick, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useSiteUI } from '../composables/useSiteUI'
import { canWrite, login, logout, useOpsAuth } from '../stores/opsAuth'
import { authApi } from '../api'
import { copyText } from '../utils/clipboard'

const SITE_TAGLINE = 'practice makes perfect'
const { ui, bannerStyle } = useSiteUI()
const { state } = useOpsAuth()

const open = ref(false)
const key = ref('')
const keyInput = ref(null)

const KEY_CACHE_MS = 5 * 60 * 1000
let fetchPending = false
let keyCache = ''
let keyCacheAt = 0

function looksLikeKey(s) {
  return /^[a-zA-Z]{8}$/.test(String(s || '').trim())
}

async function fetchKey() {
  const data = await authApi.igokey()
  const k = String(data?.key || '').trim()
  if (!looksLikeKey(k)) throw new Error('igokey shape')
  keyCache = k
  keyCacheAt = Date.now()
  return k
}

async function copyIgoKey() {
  if (fetchPending) return
  if (keyCache && looksLikeKey(keyCache) && Date.now() - keyCacheAt < KEY_CACHE_MS) {
    try {
      await copyText(keyCache)
    } catch {
      /* silent */
    }
    return
  }
  fetchPending = true
  try {
    const k = await fetchKey()
    await copyText(k)
  } catch {
    /* silent */
  } finally {
    fetchPending = false
  }
}

function close() {
  open.value = false
  key.value = ''
}

function onPermDblclick() {
  if (canWrite.value) {
    logout()
    close()
    return
  }
  open.value = true
  nextTick(() => keyInput.value?.focus())
}

async function submit() {
  const k = String(key.value || '').trim()
  if (!k || state.busy) return
  try {
    await login(k)
    close()
  } catch {
    /* silent */
  }
}

function onEscape(e) {
  if (e.key === 'Escape' && open.value) close()
}

onMounted(() => {
  window.addEventListener('keydown', onEscape)
  fetchKey().catch(() => {})
})
onUnmounted(() => {
  window.removeEventListener('keydown', onEscape)
})
</script>

<template>
  <header class="site-head" :class="{ 'has-banner': ui.hasBanner }" :style="bannerStyle()">
    <div class="shell site-head-inner">
      <div class="head-mid">
        <div class="brand-block">
          <RouterLink to="/" class="brand-home">
            <span class="brand-home-icon">
              <img class="brand-icon" :src="ui.iconUrl" alt="" width="36" height="36" />
            </span>
          </RouterLink>
          <div class="brand-copy">
            <RouterLink to="/" class="brand">{{ ui.title }}</RouterLink>
            <p class="site-tagline" title="双击复制超管 key" @dblclick.prevent="copyIgoKey">{{ SITE_TAGLINE }}</p>
          </div>
        </div>
        <nav class="nav-tabs" aria-label="主导航">
          <RouterLink to="/" class="tab" exact-active-class="is-active">depman</RouterLink>
          <RouterLink to="/envs" class="tab" active-class="is-active">环境</RouterLink>
          <RouterLink to="/term" class="tab" active-class="is-active">交互</RouterLink>
          <RouterLink to="/xfer" class="tab" active-class="is-active">传输</RouterLink>
          <RouterLink to="/ops" class="tab" active-class="is-active">登录</RouterLink>
        </nav>
      </div>
      <div class="head-perm">
        <button
          type="button"
          class="perm-badge"
          :class="{ 'is-write': canWrite }"
          title="双击切换只读 / 可操作"
          @dblclick.prevent="onPermDblclick"
        >
          {{ canWrite ? (state.session?.label || '可操作') : '只读' }}
        </button>
        <div v-if="open" class="perm-pop">
          <input
            ref="keyInput"
            v-model="key"
            class="ops-key-input"
            maxlength="8"
            autocomplete="off"
            spellcheck="false"
            placeholder="输入 key"
            @keydown.enter.prevent="submit"
          />
          <button type="button" class="btn btn-sm btn-primary" :disabled="state.busy" @click="submit">验证</button>
        </div>
      </div>
    </div>
  </header>
</template>
