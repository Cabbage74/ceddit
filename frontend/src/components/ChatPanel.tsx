import { useState, useRef, useEffect } from 'react';
import { Send, Loader2, Zap, Trash2 } from 'lucide-react';
import { useSSE } from '../hooks/useSSE';

interface Message {
  role: 'user' | 'assistant' | 'error';
  content: string;
}

interface Props {
  postId: string;
}

export default function ChatPanel({ postId }: Props) {
  const [question, setQuestion] = useState('');
  const [messages, setMessages] = useState<Message[]>([]);
  const { fullText, isStreaming, error, start, stop, reset } = useSSE();
  const inputRef = useRef<HTMLInputElement>(null);
  const bottomRef = useRef<HTMLDivElement>(null);

  // Append streaming text as latest assistant message
  useEffect(() => {
    if (fullText) {
      setMessages((prev) => {
        const copy = [...prev];
        const last = copy[copy.length - 1];
        if (last && last.role === 'assistant') {
          copy[copy.length - 1] = { role: 'assistant', content: fullText };
        } else {
          copy.push({ role: 'assistant', content: fullText });
        }
        return copy;
      });
    }
  }, [fullText]);

  // Handle error
  useEffect(() => {
    if (error) {
      setMessages((prev) => [...prev, { role: 'error', content: error }]);
    }
  }, [error]);

  // Auto-scroll
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, fullText]);

  const handleAsk = async () => {
    const q = question.trim();
    if (!q || isStreaming) return;
    setMessages((prev) => [...prev, { role: 'user', content: q }]);
    setQuestion('');
    reset();
    await start(postId, q);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleAsk();
    }
  };

  const handleClear = () => {
    reset();
    setMessages([]);
  };

  return (
    <div className="bg-white border border-scholar-border rounded-lg overflow-hidden">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-scholar-border bg-scholar-bg/50">
        <div className="flex items-center gap-2 text-sm font-medium text-scholar-text">
          <Zap className="w-4 h-4 text-amber-500" />
          AI 问答
        </div>
        {messages.length > 0 && (
          <button onClick={handleClear} className="text-xs text-scholar-muted hover:text-red-500 transition-colors">
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>

      {/* Messages */}
      <div className="h-64 overflow-y-auto p-4 space-y-3">
        {messages.length === 0 && !isStreaming && (
          <p className="text-sm text-scholar-muted text-center py-12">
            针对这篇文章提出你的问题，AI 会结合文章内容为你解答
          </p>
        )}
        {messages.map((msg, i) => (
          <div
            key={i}
            className={`text-sm leading-relaxed ${
              msg.role === 'user'
                ? 'text-right'
                : msg.role === 'error'
                ? 'text-red-500 bg-red-50 rounded-lg p-2'
                : 'text-scholar-text'
            }`}
          >
            {msg.role === 'user' ? (
              <span className="inline-block bg-scholar-accent/10 text-scholar-accent rounded-lg px-3 py-1.5 max-w-[80%] text-left">
                {msg.content}
              </span>
            ) : msg.role === 'assistant' ? (
              <div className="whitespace-pre-wrap">{msg.content}</div>
            ) : (
              msg.content
            )}
          </div>
        ))}
        {isStreaming && fullText === '' && (
          <div className="flex items-center gap-2 text-sm text-scholar-muted">
            <Loader2 className="w-3.5 h-3.5 animate-spin" />
            思考中...
          </div>
        )}
        <div ref={bottomRef} />
      </div>

      {/* Input */}
      <div className="flex items-center gap-2 px-4 py-3 border-t border-scholar-border">
        <input
          ref={inputRef}
          type="text"
          value={question}
          onChange={(e) => setQuestion(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入你的问题..."
          disabled={isStreaming}
          className="flex-1 text-sm bg-scholar-bg border border-scholar-border rounded-lg px-3 py-2 outline-none focus:border-scholar-accent/50 transition-colors disabled:opacity-50"
        />
        <button
          onClick={isStreaming ? stop : handleAsk}
          disabled={!question.trim() && !isStreaming}
          className="p-2 text-white bg-scholar-accent hover:bg-scholar-accentHover rounded-lg disabled:opacity-50 transition-colors"
        >
          {isStreaming ? (
            <Loader2 className="w-4 h-4 animate-spin" />
          ) : (
            <Send className="w-4 h-4" />
          )}
        </button>
      </div>
    </div>
  );
}
