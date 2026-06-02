import { getAccessToken } from './client';

// RAG Q&A streaming — uses fetch directly (not axios) because of SSE ReadableStream.
export async function streamQa(
  postId: string,
  question: string,
  topK = 5,
  maxTokens = 1024,
  onToken: (token: string) => void,
  onError: (err: string) => void,
  onDone: () => void,
): Promise<AbortController> {
  const controller = new AbortController();
  const token = getAccessToken();
  const params = new URLSearchParams({ question, topK: String(topK), maxTokens: String(maxTokens) });

  const headers: Record<string, string> = {};
  if (token) {
    headers['Authorization'] = `Bearer ${token}`;
  }

  try {
    const response = await fetch(`/api/v1/posts/${postId}/qa/stream?${params}`, {
      headers,
      signal: controller.signal,
    });

    if (!response.ok) {
      const text = await response.text();
      onError(text || `HTTP ${response.status}`);
      return controller;
    }

    if (!response.body) {
      onError('No response body');
      return controller;
    }

    const reader = response.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;

      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() || '';

      let currentEvent = '';
      for (const line of lines) {
        if (line.startsWith('event: ')) {
          currentEvent = line.slice(7).trim();
        } else if (line.startsWith('data: ')) {
          const data = line.slice(6);
          if (currentEvent === 'error') {
            onError(data);
            controller.abort();
            return controller;
          }
          if (data === '[DONE]') {
            onDone();
            return controller;
          }
          onToken(data);
        }
      }
    }
    // Stream ended without [DONE]
    onDone();
  } catch (err: any) {
    if (err.name !== 'AbortError') {
      onError(err.message || 'Network error');
    }
  }

  return controller;
}
