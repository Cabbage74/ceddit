import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { BookOpen } from 'lucide-react';
import { useAuthStore } from '../stores/authStore';

export default function LoginPage() {
  const [username, setUsername] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const login = useAuthStore((s) => s.login);
  const navigate = useNavigate();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!username.trim() || !password) {
      setError('请填写用户名和密码');
      return;
    }
    setLoading(true);
    setError(null);
    const err = await login(username.trim(), password);
    setLoading(false);
    if (err) {
      setError(err);
    } else {
      navigate('/');
    }
  };

  return (
    <div className="min-h-[80vh] flex items-center justify-center">
      <div className="w-full max-w-sm">
        <div className="text-center mb-8">
          <BookOpen className="w-10 h-10 text-scholar-accent mx-auto mb-3" />
          <h1 className="text-2xl font-bold text-scholar-text">登录 Ceddit</h1>
          <p className="text-sm text-scholar-muted mt-1">知识文章社区</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-white border border-scholar-border rounded-lg p-6 space-y-4">
          {error && (
            <div className="text-sm text-red-500 bg-red-50 rounded-lg px-3 py-2">{error}</div>
          )}
          <div>
            <label className="block text-sm font-medium text-scholar-text mb-1">用户名</label>
            <input
              type="text"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              className="w-full text-sm bg-scholar-bg border border-scholar-border rounded-lg px-3 py-2 outline-none focus:border-scholar-accent/50 transition-colors"
              placeholder="输入用户名"
            />
          </div>
          <div>
            <label className="block text-sm font-medium text-scholar-text mb-1">密码</label>
            <input
              type="password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              className="w-full text-sm bg-scholar-bg border border-scholar-border rounded-lg px-3 py-2 outline-none focus:border-scholar-accent/50 transition-colors"
              placeholder="输入密码"
            />
          </div>
          <button
            type="submit"
            disabled={loading}
            className="w-full py-2 text-sm font-medium text-white bg-scholar-accent hover:bg-scholar-accentHover rounded-lg transition-colors disabled:opacity-50"
          >
            {loading ? '登录中...' : '登录'}
          </button>
        </form>

        <p className="text-center text-sm text-scholar-muted mt-4">
          还没有账号？{' '}
          <Link to="/signup" className="text-scholar-accent hover:underline">
            注册
          </Link>
        </p>
      </div>
    </div>
  );
}
