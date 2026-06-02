import { sha256 } from 'js-sha256';

// Compute SHA-256 hash of a string or Blob.
// Uses js-sha256 (pure JS, works on HTTP without crypto.subtle).
export async function computeSHA256(data: string | Blob): Promise<string> {
  if (typeof data === 'string') {
    return sha256(data);
  }
  // For Blob, read as ArrayBuffer then hash
  const buffer = await data.arrayBuffer();
  return sha256(new Uint8Array(buffer));
}
