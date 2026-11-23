<script setup>
import { useSiteUI } from '../composables/useSiteUI'

const { ui } = useSiteUI()
</script>

<template>
  <footer class="footer">
    <div class="shell footer-inner">
      depman. 简单发布：按环境凭据，SSH 脚本一键部署到目标机<template v-if="ui.version"> · v{{ ui.version }}</template>
    </div>
  </footer>
</template>
