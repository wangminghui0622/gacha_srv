import { createRouter, createWebHistory } from 'vue-router'
import { installRouterGuard } from './guard'

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('@/views/login/LoginView.vue'),
      meta: { title: 'nav.login', public: true },
    },
  ],
})

installRouterGuard(router)

export default router
