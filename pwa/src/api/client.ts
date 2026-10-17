import axios from 'axios'
import { router } from '../router'

export const api = axios.create({ baseURL: '/api' })

// Sessão expirada volta pro login; usuário inativado cai na tela de bloqueio.
api.interceptors.response.use(
  (response) => response,
  (error) => {
    const status = error.response?.status
    const code = error.response?.data?.code
    const route = router.currentRoute.value
    if (status === 401 && !route.meta.public) {
      router.push({ name: 'login' })
    } else if (status === 403 && code === 'inactive' && route.name !== 'blocked') {
      router.push({ name: 'blocked' })
    }
    return Promise.reject(error)
  },
)

export function errorMessage(error: unknown): string {
  if (axios.isAxiosError(error) && error.response?.data?.error) {
    return error.response.data.error as string
  }
  return 'Algo deu errado. Tente novamente.'
}
