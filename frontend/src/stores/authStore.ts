import { create } from 'zustand';
import * as authApi from '../api/auth';
import { setAccessToken, getAccessToken } from '../api/client';
import { getFollowingList } from '../api/relation';
import { ErrorCode } from '../types/api';

// Decode JWT payload to extract user_id.
// JWTs use base64url (not standard base64), which atob can't always handle.
function decodeJWT(token: string): { user_id?: number; username?: string } | null {
  try {
    const payload = token.split('.')[1];
    // Convert base64url → base64
    const base64 = payload.replace(/-/g, '+').replace(/_/g, '/');
    const decoded = atob(base64);
    return JSON.parse(decoded);
  } catch {
    return null;
  }
}

interface AuthState {
  accessToken: string | null;
  userID: string | null;
  username: string | null;
  isLoading: boolean;
  isLoggedIn: boolean;
  // Cache of user IDs the current user follows — persists across page navigation
  followingSet: Set<string>;

  signup: (username: string, password: string) => Promise<string | null>;
  login: (username: string, password: string) => Promise<string | null>;
  logout: () => Promise<void>;
  initialize: () => Promise<void>;
  setAccessToken: (token: string | null) => void;
  setFollowing: (userId: string, following: boolean) => void;
  clear: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  accessToken: null,
  userID: null,
  username: null,
  isLoading: true,
  isLoggedIn: false,
  followingSet: new Set<string>(),

  setAccessToken: (token: string | null) => {
    setAccessToken(token);
    if (token) {
      const claims = decodeJWT(token);
      set({ accessToken: token, userID: claims?.user_id || null, username: claims?.username || null });
    } else {
      set({ accessToken: null });
    }
  },

  signup: async (username: string, password: string) => {
    const res = await authApi.signup(username, password, password);
    if (res.code === ErrorCode.Success) {
      const token = res.data.access_token;
      setAccessToken(token);
      set({
        accessToken: token,
        username,
        isLoading: false,
        isLoggedIn: true,
      });
      return null;
    }
    return res.msg;
  },

  login: async (username: string, password: string) => {
    const res = await authApi.login(username, password);
    if (res.code === ErrorCode.Success) {
      const token = res.data.access_token;
      setAccessToken(token);
      set({
        accessToken: token,
        username,
        isLoading: false,
        isLoggedIn: true,
      });
      return null;
    }
    return res.msg;
  },

  logout: async () => {
    try {
      await authApi.logout();
    } finally {
      setAccessToken(null);
      set({ accessToken: null, userID: null, username: null, isLoading: false, isLoggedIn: false });
    }
  },

  initialize: async () => {
    set({ isLoading: true });
    try {
      const res = await authApi.refreshToken();
      if (res.code === ErrorCode.Success) {
        const token = res.data.access_token;
        setAccessToken(token);
        const claims = decodeJWT(token);
        set({ accessToken: token, isLoading: false, isLoggedIn: true });
        // Rebuild following cache from server so it survives page refresh
        if (claims?.user_id) {
          try {
            const gRes = await getFollowingList(String(claims.user_id), undefined, 200);
            if (gRes.code === ErrorCode.Success) {
              const s = new Set<string>();
              for (const item of gRes.data.list) s.add(item.user_id);
              set({ followingSet: s });
            }
          } catch {}
        }
        return;
      }
    } catch {
      // No refresh cookie or refresh failed — stay anonymous
    }
    setAccessToken(null);
    set({ accessToken: null, isLoading: false, isLoggedIn: false });
  },

  setFollowing: (userId: string, following: boolean) => {
    set((state) => {
      const next = new Set(state.followingSet);
      if (following) next.add(userId);
      else next.delete(userId);
      return { followingSet: next };
    });
  },

  clear: () => {
    setAccessToken(null);
    set({ accessToken: null, userID: null, username: null, isLoading: false, isLoggedIn: false, followingSet: new Set() });
  },
}));
