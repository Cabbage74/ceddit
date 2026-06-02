import api from './client';
import type { ApiResponse } from '../types/api';
import type { FollowResult, FollowListResponse, FollowerListResponse } from '../types/relation';

export async function followUser(toUserId: string, idempotencyKey: string): Promise<ApiResponse<FollowResult>> {
  const res = await api.post(`/users/${toUserId}/follow`, null, {
    headers: { 'Idempotency-Key': idempotencyKey },
  });
  return res.data;
}

export async function unfollowUser(toUserId: string, idempotencyKey: string): Promise<ApiResponse<null>> {
  const res = await api.post(`/users/${toUserId}/unfollow`, null, {
    headers: { 'Idempotency-Key': idempotencyKey },
  });
  return res.data;
}

export async function getFollowingList(userId: string, cursor?: string, limit = 20): Promise<ApiResponse<FollowListResponse>> {
  const res = await api.get(`/users/${userId}/following`, { params: { cursor, limit } });
  return res.data;
}

export async function getFollowerList(userId: string, cursor?: string, limit = 20): Promise<ApiResponse<FollowerListResponse>> {
  const res = await api.get(`/users/${userId}/followers`, { params: { cursor, limit } });
  return res.data;
}
