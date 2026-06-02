import api from './client';
import type { ApiResponse } from '../types/api';
import type { FeedPageResponse } from '../types/post';

export async function getFeed(page = 1, size = 10): Promise<ApiResponse<FeedPageResponse>> {
  const res = await api.get('/feed', { params: { page, size } });
  return res.data;
}
