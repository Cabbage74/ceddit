import { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft, Upload, CheckCircle, Loader2, Image } from 'lucide-react';
import { usePostStore } from '../stores/postStore';
import { getAccessToken } from '../api/client';

type Step = 'idle' | 'uploading' | 'confirming' | 'patching' | 'publishing' | 'done';

const DRAFT_KEY = 'ceddit_draft';

function loadDraft(): { title: string; content: string } {
  try {
    const raw = localStorage.getItem(DRAFT_KEY);
    if (raw) return JSON.parse(raw);
  } catch {}
  return { title: '', content: '' };
}

function saveDraft(title: string, content: string) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ title, content }));
  } catch {}
}

function clearDraft() {
  try { localStorage.removeItem(DRAFT_KEY); } catch {}
}

// Upload file to COS via backend proxy — avoids CORS issues entirely.
async function uploadViaProxy(
  postId: string,
  scene: 'post_content' | 'post_image',
  file: Blob,
  filename: string,
): Promise<{ object_key: string; etag: string; size: number; sha256: string }> {
  const form = new FormData();
  form.append('scene', scene);
  form.append('post_id', postId);
  form.append('file', file, filename);

  const token = getAccessToken();
  const headers: Record<string, string> = {};
  if (token) headers['Authorization'] = `Bearer ${token}`;

  const res = await fetch('/api/v1/storage/upload', {
    method: 'POST',
    headers,
    body: form,
  });
  const json = await res.json();
  if (json.code !== 1000) throw new Error(json.msg || '上传失败');
  return json.data;
}

export default function CreatePostPage() {
  const navigate = useNavigate();
  const { createDraft, confirmContent, patchTitle, publish } = usePostStore();

  const saved = loadDraft();
  const [title, setTitle] = useState(saved.title);
  const [content, setContent] = useState(saved.content);
  const [step, setStep] = useState<Step>('idle');
  const [error, setError] = useState<string | null>(null);
  const [stepMessages, setStepMessages] = useState<string[]>([]);
  const [uploadingImages, setUploadingImages] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [dragOver, setDragOver] = useState(false);

  // One draft for everything — no orphans.
  const draftIdRef = useRef<string | null>(null);
  const getOrCreateDraft = useCallback(async (): Promise<string | null> => {
    if (draftIdRef.current) return draftIdRef.current;
    const id = await createDraft();
    if (id) draftIdRef.current = id;
    return id;
  }, [createDraft]);

  useEffect(() => {
    const timer = setTimeout(() => saveDraft(title, content), 500);
    return () => clearTimeout(timer);
  }, [title, content]);

  const addMessage = (msg: string) => setStepMessages((prev) => [...prev, msg]);

  const insertAtCursor = useCallback((text: string) => {
    const ta = textareaRef.current;
    const start = ta?.selectionStart ?? 0;
    const end = ta?.selectionEnd ?? 0;
    setContent((prev) => prev.slice(0, start) + text + prev.slice(end));
    requestAnimationFrame(() => {
      ta?.focus();
      const pos = start + text.length;
      ta?.setSelectionRange(pos, pos);
    });
  }, []);

  const handleFiles = useCallback(async (files: FileList | File[]) => {
    const imageFiles = Array.from(files).filter((f) => f.type.startsWith('image/'));
    if (!imageFiles.length) { setError('只支持图片文件'); return; }

    setUploadingImages(true);
    setError(null);

    const postId = await getOrCreateDraft();
    if (!postId) { addMessage('创建草稿失败'); setUploadingImages(false); return; }

    for (const file of imageFiles) {
      addMessage(`正在上传: ${file.name}...`);
      try {
        const result = await uploadViaProxy(postId, 'post_image', file, file.name);
        const imgUrl = `/api/v1/storage/object?key=${encodeURIComponent(result.object_key)}`;
        insertAtCursor('\n' + `![${file.name}](${imgUrl})` + '\n');
        addMessage(`已插入: ${file.name}`);
      } catch (err: any) {
        addMessage(`失败: ${err.message}`);
      }
    }
    setUploadingImages(false);
  }, [getOrCreateDraft, insertAtCursor, addMessage]);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault(); setDragOver(false); handleFiles(e.dataTransfer.files);
  }, [handleFiles]);

  const handlePaste = useCallback((e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    const files: File[] = [];
    for (const item of items) {
      if (item.type.startsWith('image/')) { const f = item.getAsFile(); if (f) files.push(f); }
    }
    if (files.length) { e.preventDefault(); handleFiles(files); }
  }, [handleFiles]);

  const handlePublish = async () => {
    if (!title.trim()) { setError('请输入标题'); return; }
    if (!content.trim()) { setError('请输入内容'); return; }

    setError(null); setStepMessages([]);

    try {
      const postId = (await getOrCreateDraft())!;
      if (!postId) throw new Error('创建草稿失败');

      setStep('uploading');
      addMessage('正在上传内容...');
      const blob = new Blob([content], { type: 'text/markdown' });
      const up = await uploadViaProxy(postId, 'post_content', blob, 'content.md');
      addMessage('内容上传成功');

      setStep('confirming');
      addMessage('正在验证...');
      const cerr = await confirmContent(postId, {
        object_key: up.object_key, etag: up.etag, size: up.size, sha256: up.sha256,
      });
      if (cerr) throw new Error(cerr);

      setStep('patching');
      const perr = await patchTitle(postId, title.trim());
      if (perr) throw new Error(perr);

      setStep('publishing');
      addMessage('正在发布...');
      const puberr = await publish(postId);
      if (puberr) throw new Error(puberr);
      addMessage('发布成功！');

      clearDraft();
      setStep('done');
      setTimeout(() => navigate(`/post/${postId}`), 1000);
    } catch (err: any) {
      setError(err.message); addMessage(`错误: ${err.message}`); setStep('idle');
    }
  };

  const busy = step !== 'idle' && step !== 'done';

  return (
    <div className="max-w-2xl mx-auto">
      <div className="flex items-center gap-4 mb-6">
        <button onClick={() => navigate(-1)} className="text-scholar-muted hover:text-scholar-text"><ArrowLeft className="w-5 h-5" /></button>
        <h1 className="text-2xl font-bold text-scholar-text">写文章</h1>
      </div>

      {error && <div className="mb-4 text-sm text-red-500 bg-red-50 border border-red-200 rounded-lg px-4 py-3">{error}</div>}

      {stepMessages.length > 0 && (
        <div className="mb-4 bg-scholar-bg border border-scholar-border rounded-lg p-4">
          {stepMessages.map((m, i) => (
            <div key={i} className="flex items-center gap-2 text-sm text-scholar-muted py-0.5">
              {i === stepMessages.length - 1 && busy ? <Loader2 className="w-3.5 h-3.5 animate-spin text-scholar-accent" /> : <CheckCircle className="w-3.5 h-3.5 text-green-500" />}
              {m}
            </div>
          ))}
        </div>
      )}

      <div className="bg-white border border-scholar-border rounded-lg p-6 space-y-4">
        <div>
          <label className="block text-sm font-medium text-scholar-text mb-1">标题</label>
          <input type="text" value={title} onChange={e => setTitle(e.target.value)} disabled={busy}
            className="w-full text-lg bg-scholar-bg border border-scholar-border rounded-lg px-4 py-2.5 outline-none focus:border-scholar-accent/50 disabled:opacity-50"
            placeholder="输入文章标题" />
        </div>

        <div>
          <div className="flex items-center justify-between mb-1">
            <label className="text-sm font-medium text-scholar-text">内容 <span className="text-scholar-muted font-normal">(Markdown)</span></label>
            <label className="flex items-center gap-1 text-xs text-scholar-accent cursor-pointer hover:text-scholar-accentHover">
              <Image className="w-3.5 h-3.5" />插入图片
              <input type="file" accept="image/*" multiple className="hidden"
                onChange={e => e.target.files && handleFiles(e.target.files)} disabled={busy || uploadingImages} />
            </label>
          </div>
          <div onDragOver={e => { e.preventDefault(); setDragOver(true); }} onDragLeave={() => setDragOver(false)} onDrop={handleDrop}
            className={`relative ${dragOver ? 'ring-2 ring-scholar-accent rounded-lg' : ''}`}>
            <textarea ref={textareaRef} value={content} onChange={e => setContent(e.target.value)}
              onPaste={handlePaste} disabled={busy} rows={18}
              className="w-full text-sm bg-scholar-bg border border-scholar-border rounded-lg px-4 py-3 outline-none focus:border-scholar-accent/50 disabled:opacity-50 font-mono resize-y"
              placeholder="用 Markdown 书写你的文章内容...&#10;&#10;支持拖拽或粘贴图片" />
            {dragOver && (
              <div className="absolute inset-0 bg-scholar-accent/5 rounded-lg flex items-center justify-center pointer-events-none">
                <div className="bg-white rounded-lg px-4 py-2 shadow-lg text-sm text-scholar-accent font-medium">松开以插入图片</div>
              </div>
            )}
          </div>
          <p className="text-xs text-scholar-muted mt-1">
            {uploadingImages ? '正在上传图片...' : '支持拖拽、粘贴或点击上方按钮插入图片 · 草稿自动保存'}
          </p>
        </div>

        <div className="flex items-center justify-between pt-2">
          <button onClick={() => { clearDraft(); setTitle(''); setContent(''); draftIdRef.current = null; }}
            disabled={busy} className="text-xs text-scholar-muted hover:text-red-500">清除草稿</button>
          <button onClick={handlePublish} disabled={busy || uploadingImages}
            className="flex items-center gap-2 px-6 py-2.5 text-sm font-medium text-white bg-scholar-accent hover:bg-scholar-accentHover rounded-lg disabled:opacity-50">
            {busy ? <><Loader2 className="w-4 h-4 animate-spin" />发布中...</> : <><Upload className="w-4 h-4" />发布文章</>}
          </button>
        </div>
      </div>
    </div>
  );
}
