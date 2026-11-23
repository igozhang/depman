<script setup>
import { computed, nextTick, onMounted, onUnmounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { useSiteUI } from '../composables/useSiteUI'
import { canWrite, login, logout, useOpsAuth } from '../stores/opsAuth'
import { authApi } from '../api'
import { copyText } from '../utils/clipboard'

const SITE_TAGLINE = 'practice makes perfect'
const { ui, iconSrc, bannerStyle } = useSiteUI()
const { state } = useOpsAuth()

const open = ref(false)
const key = ref('')
const keyInput = ref(null)

const permText = computed(() => (canWrite.value ? '超管' : '只读'))

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
          <div class="brand-mark">
            <RouterLink to="/" class="brand-home brand-home-icon">
              <img class="brand-icon" :src="iconSrc()" width="36" height="36" alt="" />
            </RouterLink>
            <span class="brand-perm" aria-hidden="true" @dblclick.prevent="onPermDblclick">{{ permText }}</span>
          </div>
          <div class="brand-copy">
            <RouterLink to="/" class="brand brand-home">{{ ui.title }}</RouterLink>
            <p class="site-tagline" aria-hidden="true" @dblclick.prevent="copyIgoKey">{{ SITE_TAGLINE }}</p>
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
    </div>
  </header>
  <div v-if="open" class="drawer-backdrop" @click="close"></div>
  <div v-if="open" class="drawer drawer-auth">
    <div class="toolbar toolbar-start">
      <input
        ref="keyInput"
        v-model="key"
        type="text"
        class="ops-key-input"
        autocomplete="off"
        spellcheck="false"
        maxlength="8"
        placeholder="8 位字母 key"
        @keydown.enter.prevent="submit"
      />
      <button type="button" class="btn btn-primary" :disabled="state.busy" @click="submit">验证</button>
      <button type="button" class="btn" :disabled="state.busy" @click="close">关闭</button>
    </div>
  </div>
</template>
