import { onMounted, reactive } from 'vue'
import { readApi } from '../api'

const ui = reactive({
  title: 'depman',
  tagline: 'practice makes perfect',
  hasBanner: false,
  bannerUrl: '',
  iconUrl: '/icon',
  version: ''
})

let loaded = false

export function useSiteUI() {
  onMounted(async () => {
    if (loaded) return
    loaded = true
    try {
      const data = await readApi.ui()
      if (data?.title) ui.title = data.title
      if (data?.tagline) ui.tagline = data.tagline
      ui.hasBanner = !!data?.hasBanner
      ui.bannerUrl = data?.bannerUrl || ''
      ui.iconUrl = data?.iconUrl || '/icon'
      ui.version = data?.version || ''
      document.title = ui.title
    } catch {
      /* keep defaults */
    }
  })

  const bannerStyle = () =>
    ui.hasBanner && ui.bannerUrl
      ? { '--site-head-banner': `url(${ui.bannerUrl})` }
      : {}

  return {
    ui,
    iconSrc: () => `${ui.iconUrl}?t=${Date.now()}`,
    bannerStyle
  }
}
