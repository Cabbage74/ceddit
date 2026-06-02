import { create } from 'zustand';
import { getFeed } from '../api/feed';
import { ErrorCode } from '../types/api';
import type { FeedItem } from '../types/post';

interface FeedState {
  items: FeedItem[];
  page: number;
  hasMore: boolean;
  isLoading: boolean;
  error: string | null;

  fetchFeed: (page?: number, size?: number) => Promise<void>;
  loadMore: () => Promise<void>;
  invalidate: () => void;
}

export const useFeedStore = create<FeedState>((set, get) => ({
  items: [],
  page: 1,
  hasMore: false,
  isLoading: false,
  error: null,

  fetchFeed: async (page = 1, size = 10) => {
    set({ isLoading: true, error: null });
    try {
      const res = await getFeed(page, size);
      if (res.code === ErrorCode.Success) {
        set({
          items: res.data.items,
          page: res.data.page,
          hasMore: res.data.has_more,
          isLoading: false,
        });
      } else {
        set({ error: res.msg, isLoading: false });
      }
    } catch (err: any) {
      set({ error: err.message || '加载失败', isLoading: false });
    }
  },

  loadMore: async () => {
    const { page, hasMore, isLoading, items } = get();
    if (!hasMore || isLoading) return;
    set({ isLoading: true });
    try {
      const res = await getFeed(page + 1);
      if (res.code === ErrorCode.Success) {
        set({
          items: [...items, ...res.data.items],
          page: res.data.page,
          hasMore: res.data.has_more,
          isLoading: false,
        });
      } else {
        set({ error: res.msg, isLoading: false });
      }
    } catch (err: any) {
      set({ error: err.message || '加载失败', isLoading: false });
    }
  },

  invalidate: () => set({ items: [], page: 1, hasMore: false }),
}));
