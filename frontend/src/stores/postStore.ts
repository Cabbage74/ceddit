import { create } from 'zustand';
import * as postsApi from '../api/posts';
import { vote as voteApi } from '../api/vote';
import { ErrorCode } from '../types/api';
import type { PostDetail, FeedItem } from '../types/post';

interface PostState {
  currentPost: PostDetail | null;
  postList: PostDetail[];
  isLoading: boolean;
  error: string | null;

  fetchPostDetail: (postId: string) => Promise<void>;
  fetchPostList: (page?: number, size?: number, order?: 'time' | 'score') => Promise<void>;
  fetchEasyPost: (page?: number, size?: number) => Promise<void>;
  toggleLike: (postId: string, currentLiked: boolean) => Promise<boolean>;
  clearCurrentPost: () => void;
  // Update a feed item's like state locally (for optimistic UI)
  updateFeedItemLike: (postId: string, liked: boolean) => void;
  createDraft: () => Promise<string | null>;
  getPresignedURL: (postId: string, scene: 'post_content' | 'post_image', contentType: string, ext: string) => Promise<any>;
  confirmContent: (postId: string, params: any) => Promise<string | null>;
  patchTitle: (postId: string, title: string) => Promise<string | null>;
  publish: (postId: string) => Promise<string | null>;
}

export const usePostStore = create<PostState>((set) => ({
  currentPost: null,
  postList: [],
  isLoading: false,
  error: null,

  fetchPostDetail: async (postId: string) => {
    set({ isLoading: true, error: null });
    try {
      const res = await postsApi.getPostDetail(postId);
      if (res.code === ErrorCode.Success) {
        set({ currentPost: res.data, isLoading: false });
      } else {
        set({ error: res.msg, isLoading: false });
      }
    } catch (err: any) {
      set({ error: err.message || '加载失败', isLoading: false });
    }
  },

  fetchPostList: async (page = 1, size = 10, order = 'time') => {
    set({ isLoading: true, error: null });
    try {
      const res = await postsApi.getPostList({ page, size, order });
      if (res.code === ErrorCode.Success) {
        set({ postList: res.data, isLoading: false });
      } else {
        set({ error: res.msg, isLoading: false });
      }
    } catch (err: any) {
      set({ error: err.message || '加载失败', isLoading: false });
    }
  },

  fetchEasyPost: async (page = 1, size = 10) => {
    set({ isLoading: true, error: null });
    try {
      const res = await postsApi.getEasyPost(page, size);
      if (res.code === ErrorCode.Success) {
        set({ postList: res.data, isLoading: false });
      } else {
        set({ error: res.msg, isLoading: false });
      }
    } catch (err: any) {
      set({ error: err.message || '加载失败', isLoading: false });
    }
  },

  toggleLike: async (postId: string, currentLiked: boolean) => {
    const direction = currentLiked ? 0 : 1;
    try {
      const res = await voteApi({ post_id: postId, direction });
      if (res.code === ErrorCode.Success) {
        return res.data.liked;
      }
      return currentLiked;
    } catch {
      return currentLiked;
    }
  },

  updateFeedItemLike: (postId: string, liked: boolean) => {
    set((state) => {
      if (!state.currentPost || state.currentPost.post_id !== postId) return state;
      return {
        currentPost: {
          ...state.currentPost,
          vote_num: state.currentPost.vote_num + (liked ? 1 : -1),
        },
      };
    });
  },

  clearCurrentPost: () => set({ currentPost: null }),

  createDraft: async () => {
    const res = await postsApi.createDraft();
    if (res.code === ErrorCode.Success) return res.data.post_id;
    return null;
  },

  getPresignedURL: async (postId, scene, contentType, ext) => {
    const res = await postsApi.getPresignedURL({ scene, post_id: postId, content_type: contentType, ext });
    if (res.code === ErrorCode.Success) return res.data;
    return null;
  },

  confirmContent: async (postId, params) => {
    const res = await postsApi.confirmContent(postId, params);
    if (res.code === ErrorCode.Success) return null;
    return res.msg;
  },

  patchTitle: async (postId, title) => {
    const res = await postsApi.patchPost(postId, { title });
    if (res.code === ErrorCode.Success) return null;
    return res.msg;
  },

  publish: async (postId) => {
    const res = await postsApi.publishPost(postId);
    if (res.code === ErrorCode.Success) return null;
    return res.msg;
  },
}));
