import { api } from './client'
import { API_ENDPOINTS } from '../constants'

interface LoginResponse {
  access_token: string
  refresh_token: string
  driver: {
    id: string
    phone: string
    name: string
  }
}

export async function driverLogin(phone: string, pin: string): Promise<LoginResponse> {
  return api.post<LoginResponse>(API_ENDPOINTS.auth.driverLogin, { phone, pin })
}

export async function logout(): Promise<void> {
  return api.post<void>(API_ENDPOINTS.auth.logout, {})
}

export async function refreshToken(refreshToken: string): Promise<{ access_token: string; refresh_token: string }> {
  return api.post<{ access_token: string; refresh_token: string }>(API_ENDPOINTS.auth.refresh, {
    refresh_token: refreshToken,
  })
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  return api.post<void>(API_ENDPOINTS.auth.changePassword, {
    current_password: currentPassword,
    new_password: newPassword,
  })
}
