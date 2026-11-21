<script setup>
import { RouterView } from 'vue-router'
import SiteHeader from '../components/SiteHeader.vue'
</script>

<template>
  <div class="layout">
    <SiteHeader />
    <main class="body shell">
      <RouterView />
    </main>
  </div>
</template>
