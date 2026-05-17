import { createRouter, createWebHistory } from 'vue-router'
import { getAuthToken, statusApi } from '../api'

const router = createRouter({
  history: createWebHistory('/ui/'),
  routes: [
    {
      path: '/login',
      name: 'login',
      component: () => import('../views/LoginView.vue'),
    },
    {
      path: '/',
      name: 'dashboard',
      component: () => import('../views/DashboardView.vue'),
    },
  ],
})

async function canAccessWithoutToken(): Promise<boolean> {
  try {
    await statusApi.get()
    return true
  } catch {
    return false
  }
}

router.beforeEach(async (to, _from) => {
  const token = getAuthToken()
  if (to.name !== 'login' && !token) {
    if (await canAccessWithoutToken()) return true
    return { name: 'login' }
  }
  if (to.name === 'login' && token) {
    return { name: 'dashboard' }
  }
  if (to.name === 'login' && !token && await canAccessWithoutToken()) {
    return { name: 'dashboard' }
  }
})

export default router
