import { Link, useNavigate } from 'react-router-dom';
import { BookOpen, PlusCircle, LogOut, User } from 'lucide-react';
import { useAuthStore } from '../stores/authStore';

export default function Navbar() {
  const { isLoggedIn, username, logout } = useAuthStore();
  const navigate = useNavigate();

  const handleLogout = async () => {
    await logout();
    navigate('/');
  };

  return (
    <nav className="sticky top-0 z-50 bg-white/80 backdrop-blur-md border-b border-scholar-border">
      <div className="max-w-4xl mx-auto px-4 h-14 flex items-center justify-between">
        {/* Logo */}
        <Link to="/" className="flex items-center gap-2 text-scholar-text hover:text-scholar-accent transition-colors">
          <BookOpen className="w-6 h-6" />
          <span className="text-lg font-semibold tracking-wide">Ceddit</span>
        </Link>

        {/* Nav Links */}
        <div className="flex items-center gap-1">
          {isLoggedIn && (
            <>
              <Link
                to="/create"
                className="flex items-center gap-1.5 px-3 py-1.5 text-sm text-scholar-accent hover:bg-scholar-bg rounded-lg transition-colors"
              >
                <PlusCircle className="w-4 h-4" />
                <span className="hidden sm:inline">写文章</span>
              </Link>
              <Link
                to="/posts"
                className="px-3 py-1.5 text-sm text-scholar-muted hover:text-scholar-text hover:bg-scholar-bg rounded-lg transition-colors"
              >
                发现
              </Link>
              <span className="flex items-center gap-1 px-3 py-1.5 text-sm text-scholar-muted">
                <User className="w-4 h-4" />
                {username}
              </span>
              <button
                onClick={handleLogout}
                className="flex items-center gap-1 px-3 py-1.5 text-sm text-scholar-muted hover:text-red-500 hover:bg-red-50 rounded-lg transition-colors"
              >
                <LogOut className="w-4 h-4" />
                <span className="hidden sm:inline">登出</span>
              </button>
            </>
          )}
          {!isLoggedIn && (
            <>
              <Link
                to="/login"
                className="px-3 py-1.5 text-sm text-scholar-muted hover:text-scholar-text hover:bg-scholar-bg rounded-lg transition-colors"
              >
                登录
              </Link>
              <Link
                to="/signup"
                className="px-4 py-1.5 text-sm text-white bg-scholar-accent hover:bg-scholar-accentHover rounded-lg transition-colors"
              >
                注册
              </Link>
            </>
          )}
        </div>
      </div>
    </nav>
  );
}
