export interface PaginatedMeta {
  page: number
  per_page: number
  total: number
}

export interface PaginatedResponse<T> {
  data: T[]
  meta: PaginatedMeta
}

export interface ApiError {
  code: string
  message: string
  details?: any
}

export interface ApiResponse<T> {
  data?: T
  error?: ApiError
  meta?: {
    request_id?: string
  }
}
