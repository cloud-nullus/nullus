import axios from 'axios'
import { useAuthStore } from '../stores/auth-store'
import { applyTourFixture } from '../features/tour/tour-mock-adapter'

interface StandardizedApiError {
  status: number
  message: string
  details?: unknown
}

function standardizeApiError(error: unknown): StandardizedApiError {
  if (axios.isAxiosError(error)) {
    const status = error.response?.status ?? 500
    const responseData = error.response?.data
    const topLevelMessage =
      typeof responseData === 'object' &&
      responseData !== null &&
      'message' in responseData &&
      typeof responseData.message === 'string'
        ? responseData.message
        : null
    const nestedMessage =
      typeof responseData === 'object' &&
      responseData !== null &&
      'error' in responseData &&
      typeof responseData.error === 'object' &&
      responseData.error !== null &&
      'message' in responseData.error &&
      typeof responseData.error.message === 'string'
        ? responseData.error.message
        : null
    const message = (topLevelMessage ?? nestedMessage ?? error.message) || 'Request failed'

    return {
      status,
      message,
      details: responseData,
    }
  }

  if (error instanceof Error) {
    return {
      status: 500,
      message: error.message,
      details: error,
    }
  }

  return {
    status: 500,
    message: 'Unexpected error occurred',
    details: error,
  }
}

export const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
})

api.interceptors.request.use((config) => {
  const { token, user } = useAuthStore.getState()

  config.headers = axios.AxiosHeaders.from(config.headers)

  if (token) {
    config.headers.set('Authorization', `Bearer ${token}`)
  }

  if (user?.orgId) {
    config.headers.set('X-Org-ID', user.orgId)
  }

  // Alpha/session auth: the backend AuthMiddleware identifies the caller via
  // X-User-* headers (see internal/auth/adapter/middleware/auth_middleware.go).
  // Without these the API returns 401 → the response interceptor logs the user
  // out, bouncing them back to /login right after a successful client-side login.
  if (user) {
    config.headers.set('X-User-ID', user.id)
    config.headers.set('X-User-Email', user.email)
    config.headers.set('X-User-Name', user.name)
    config.headers.set('X-User-Role', user.role)
    config.headers.set('X-User-OrgID', user.orgId)
  }

  // 둘러보기 중에는 목록 응답을 화면 재료로 갈아 끼운다. 투어를 처음 도는
  // 사람의 계정은 대개 비어 있어, 빈 표를 강조하며 흐름을 설명할 수 없다.
  return applyTourFixture(config)
})

api.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      useAuthStore.getState().logout()
    }

    return Promise.reject(standardizeApiError(error))
  }
)
