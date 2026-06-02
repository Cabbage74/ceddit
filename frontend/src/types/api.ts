// Generic API response envelope
export interface ApiResponse<T = null> {
  code: number;
  msg: string;
  data: T;
}

// Error codes from the backend
export const ErrorCode = {
  Success: 1000,
  InvalidParam: 1001,
  UserExist: 1002,
  UserNotExist: 1003,
  InvalidPassword: 1004,
  ServerBusy: 1005,
  NeedAuth: 1006,
  InvalidAuth: 1007,
  AuthExpired: 1008,
  NotOwner: 1009,
  ContentNotOK: 1010,
  DraftNotReady: 1011,
  Fail: 1012,
} as const;
