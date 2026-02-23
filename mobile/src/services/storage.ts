import * as SecureStore from 'expo-secure-store'

export async function saveToken(key: string, token: string): Promise<void> {
  await SecureStore.setItemAsync(key, token)
}

export async function getToken(key: string): Promise<string | null> {
  return SecureStore.getItemAsync(key)
}

export async function removeToken(key: string): Promise<void> {
  await SecureStore.deleteItemAsync(key)
}

export async function clearAll(): Promise<void> {
  await SecureStore.deleteItemAsync('access_token')
  await SecureStore.deleteItemAsync('refresh_token')
  await SecureStore.deleteItemAsync('user_data')
}
