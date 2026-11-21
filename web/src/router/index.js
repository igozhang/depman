import { createRouter, createWebHistory } from 'vue-router'
import SiteLayout from '../layouts/SiteLayout.vue'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    {
      path: '/',
      component: SiteLayout,
      children: [
        { path: '', name: 'home', component: () => import('../views/Home.vue') },
        { path: 'projects/:id', name: 'project', component: () => import('../views/Project.vue') },
        { path: 'envs', name: 'envs', component: () => import('../views/Envs.vue') },
        { path: 'term', name: 'term', component: () => import('../views/Term.vue') },
        { path: 'xfer', name: 'xfer', component: () => import('../views/Xfer.vue') },
        { path: 'ops', name: 'ops', component: () => import('../views/Ops.vue') }
      ]
    }
  ],
  scrollBehavior() {
    return { top: 0 }
  }
})

export default router
