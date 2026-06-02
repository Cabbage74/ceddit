export interface FollowListItem {
  user_id: string;
  username: string;
  created_at: string;
}

export interface FollowerListItem {
  user_id: string;
  username: string;
  is_mutual: boolean;
  created_at: string;
}

export interface FollowListResponse {
  list: FollowListItem[];
  next_cursor: string;
  total: number;
}

export interface FollowerListResponse {
  list: FollowerListItem[];
  next_cursor: string;
  total: number;
}

export interface FollowResult {
  id: string;
  from_user_id: string;
  to_user_id: string;
  created_at: string;
}
