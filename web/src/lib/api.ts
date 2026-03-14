import axios from 'axios'

export const api = axios.create({
  baseURL: '/api/v1',
  headers: {
    'Content-Type': 'application/json',
  },
})

api.interceptors.response.use(
  (response) => response,
  (error: unknown) => {
    if (axios.isAxiosError(error) && error.response?.status === 401) {
      // Clear auth state and redirect to login
      localStorage.removeItem('nullus-token')
      window.location.href = '/login'
    }
    return Promise.reject(error)
  }
)
