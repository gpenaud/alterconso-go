import axios from 'axios'
import { useAuthStore } from '../store/auth'

const api = axios.create({ baseURL: '/api' })

api.interceptors.request.use((config) => {
  const token = useAuthStore.getState().token
  if (token) config.headers.Authorization = `Bearer ${token}`
  return config
})

api.interceptors.response.use(
  (r) => r,
  (err) => {
    if (err.response?.status === 401) {
      // L'auth peut être portée par le cookie JWT (cas de la SPA shop accédée
      // directement via /shop2/...) ou par le Bearer token dans le store. Dans
      // les deux cas on renvoie vers le login Go avec un __redirect, qui ré-émet
      // un cookie et rebascule l'utilisateur exactement là où il était.
      useAuthStore.getState().logout()
      const here = window.location.pathname + window.location.search
      const url = `/user/login?__redirect=${encodeURIComponent(here)}`
      // Évite les boucles si on est déjà sur la page de login (très improbable
      // depuis la SPA mais pas coûteux à garder).
      if (!window.location.pathname.startsWith('/user/login')) {
        window.location.href = url
      }
    }
    return Promise.reject(err)
  }
)

export default api
