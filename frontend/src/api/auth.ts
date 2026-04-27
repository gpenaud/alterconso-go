import api from './client'

export interface User {
  id: number
  email: string
  firstname: string
  lastname: string
  isAdmin?: boolean
}

export interface LoginResponse {
  token: string
  user: User
}

export const login = (email: string, password: string, groupId?: number) =>
  api.post<LoginResponse>('/auth/login', { email, password, groupId }).then((r) => r.data)

export const logout = () => api.post('/auth/logout')
