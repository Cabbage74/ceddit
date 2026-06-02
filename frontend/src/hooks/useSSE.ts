import { useState, useCallback, useRef } from 'react';
import { streamQa } from '../api/rag';

export function useSSE() {
  const [tokens, setTokens] = useState<string[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  const start = useCallback(
    async (
      postId: string,
      question: string,
      topK = 5,
      maxTokens = 1024,
    ) => {
      setTokens([]);
      setError(null);
      setIsStreaming(true);

      controllerRef.current = await streamQa(
        postId,
        question,
        topK,
        maxTokens,
        // onToken
        (token: string) => {
          setTokens((prev) => [...prev, token]);
        },
        // onError
        (err: string) => {
          setError(err);
          setIsStreaming(false);
        },
        // onDone
        () => {
          setIsStreaming(false);
        },
      );
    },
    [],
  );

  const stop = useCallback(() => {
    controllerRef.current?.abort();
    setError('已取消');
    setIsStreaming(false);
  }, []);

  const reset = useCallback(() => {
    setTokens([]);
    setError(null);
    setIsStreaming(false);
  }, []);

  const fullText = tokens.join('');

  return { tokens, fullText, isStreaming, error, start, stop, reset };
}
