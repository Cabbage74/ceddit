import { useCallback } from 'react';

export function useIdempotency() {
  const generateKey = useCallback(() => crypto.randomUUID(), []);
  return { generateKey };
}
