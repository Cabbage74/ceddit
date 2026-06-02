import api from './client';
import type { ApiResponse } from '../types/api';
import type { TokenPair } from '../types/user';

export async function signup(username: string, password: string, rePassword: string): Promise<ApiResponse<TokenPair>> {
  const res = await api.post('/signup', { username, password, re_password: rePassword });
  return res.data;
}

export async function login(username: string, password: string): Promise<ApiResponse<TokenPair>> {
  const res = await api.post('/login', { username, password });
  return res.data;
}

export async function refreshToken(): Promise<ApiResponse<TokenPair>> {
  const res = await api.post('/refresh');
  return res.data;
}

export async function logout(): Promise<ApiResponse<null>> {
  const res = await api.post('/logout');
  return res.data;
}
