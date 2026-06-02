import axios from 'axios';
import { ErrorCode } from '../types/api';

const api = axios.create({
  baseURL: '/api/v1',
  withCredentials: true,
  headers: { 'Content-Type': 'application/json' },
});

// Module-level token storage — auth store updates this directly.
// Avoids circular dependency between client ↔ store.
let accessToken: string | null = null;

export function setAccessToken(token: string | null) {
  accessToken = token;
  if (token) {
    api.defaults.headers.common['Authorization'] = `Bearer ${token}`;
  } else {
    delete api.defaults.headers.common['Authorization'];
  }
}

export function getAccessToken(): string | null {
  return accessToken;
}

// Token refresh state
let isRefreshing = false;
let pendingRequests: Array<{
  resolve: (token: string) => void;
  reject: (err: unknown) => void;
}> = [];

function flushPendingRequests(token: string | null, error: unknown | null) {
  pendingRequests.forEach(({ resolve, reject }) => {
    if (token) resolve(token);
    else reject(error);
  });
  pendingRequests = [];
}

async function handleAuthExpired(failedConfig: any): Promise<any> {
  if (failedConfig._retry) {
    return Promise.reject(new Error('Token refresh loop'));
  }

  if (isRefreshing) {
    return new Promise((resolve, reject) => {
      pendingRequests.push({
        resolve: (token: string) => {
          failedConfig.headers.Authorization = `Bearer ${token}`;
          resolve(api(failedConfig));
        },
        reject,
      });
    });
  }

  failedConfig._retry = true;
  isRefreshing = true;

  try {
    const res = await axios.post('/api/v1/refresh', {}, { withCredentials: true });
    if (res.data.code !== ErrorCode.Success) {
      throw new Error(res.data.msg || 'Refresh failed');
    }
    const newToken: string = res.data.data.access_token;
    setAccessToken(newToken);
    flushPendingRequests(newToken, null);
    failedConfig.headers.Authorization = `Bearer ${newToken}`;
    return api(failedConfig);
  } catch (err) {
    flushPendingRequests(null, err);
    setAccessToken(null);
    window.location.href = '/login';
    return Promise.reject(err);
  } finally {
    isRefreshing = false;
  }
}

// Response interceptor
api.interceptors.response.use(
  (response) => {
    const { code } = response.data;
    if (code === ErrorCode.Success) return response;
    if (code === ErrorCode.AuthExpired) return handleAuthExpired(response.config);
    return response;
  },
  (error) => Promise.reject(error),
);

// Request interceptor — attach token
api.interceptors.request.use((config) => {
  if (accessToken) {
    config.headers.Authorization = `Bearer ${accessToken}`;
  }
  return config;
});

export default api;
