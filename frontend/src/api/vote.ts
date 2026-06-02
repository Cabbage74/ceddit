import api from './client';
import type { ApiResponse } from '../types/api';
import type { ParamVote, VoteResult } from '../types/post';

export async function vote(params: ParamVote): Promise<ApiResponse<VoteResult>> {
  const res = await api.post('/vote', params);
  return res.data;
}
