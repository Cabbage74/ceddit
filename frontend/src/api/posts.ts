import api from './client';
import type { ApiResponse } from '../types/api';
import type {
  PostDetail,
  RespCreateDraft,
  RespPresign,
  ParamPresign,
  ParamContentConfirm,
  ParamPatchPost,
  ParamPostList,
} from '../types/post';

// Create a draft
export async function createDraft(): Promise<ApiResponse<RespCreateDraft>> {
  const res = await api.post('/post/draft');
  return res.data;
}

// Get presigned COS upload URL
export async function getPresignedURL(params: ParamPresign): Promise<ApiResponse<RespPresign>> {
  const res = await api.post('/storage/presign', params);
  return res.data;
}

// Confirm uploaded content
export async function confirmContent(postId: string, params: ParamContentConfirm): Promise<ApiResponse<null>> {
  const res = await api.post(`/post/${postId}/content/confirm`, params);
  return res.data;
}

// Set title
export async function patchPost(postId: string, params: ParamPatchPost): Promise<ApiResponse<null>> {
  const res = await api.patch(`/post/${postId}`, params);
  return res.data;
}

// Publish
export async function publishPost(postId: string): Promise<ApiResponse<null>> {
  const res = await api.post(`/post/${postId}/publish`);
  return res.data;
}

// Get post detail
export async function getPostDetail(postId: string): Promise<ApiResponse<PostDetail>> {
  const res = await api.get(`/post/${postId}`);
  return res.data;
}

// Get post list (sorted by time/score)
export async function getPostList(params: ParamPostList): Promise<ApiResponse<PostDetail[]>> {
  const res = await api.get('/post', { params });
  return res.data;
}

// Get easy post list
export async function getEasyPost(page = 1, size = 10): Promise<ApiResponse<PostDetail[]>> {
  const res = await api.get('/easypost', { params: { page, size } });
  return res.data;
}
