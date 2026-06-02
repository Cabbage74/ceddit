import { useEffect } from 'react';
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom';
import { useAuthStore } from './stores/authStore';
import Layout from './components/Layout';
import LoadingSpinner from './components/LoadingSpinner';
import FeedPage from './pages/FeedPage';
import LoginPage from './pages/LoginPage';
import SignupPage from './pages/SignupPage';
import PostDetailPage from './pages/PostDetailPage';
import CreatePostPage from './pages/CreatePostPage';
import PostListPage from './pages/PostListPage';
import EasyPostListPage from './pages/EasyPostListPage';
import FollowingListPage from './pages/FollowingListPage';
import FollowerListPage from './pages/FollowerListPage';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const { isLoggedIn, isLoading } = useAuthStore();
  if (isLoading) return <LoadingSpinner />;
  if (!isLoggedIn) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

export default function App() {
  const initialize = useAuthStore((s) => s.initialize);
  const isLoading = useAuthStore((s) => s.isLoading);

  useEffect(() => {
    initialize();
  }, [initialize]);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-scholar-bg flex items-center justify-center">
        <LoadingSpinner text="正在连接..." />
      </div>
    );
  }

  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          {/* Public */}
          <Route path="/" element={<FeedPage />} />
          <Route path="/login" element={<LoginPage />} />
          <Route path="/signup" element={<SignupPage />} />

          {/* Protected */}
          <Route path="/post/:id" element={
            <ProtectedRoute><PostDetailPage /></ProtectedRoute>
          } />
          <Route path="/create" element={
            <ProtectedRoute><CreatePostPage /></ProtectedRoute>
          } />
          <Route path="/posts" element={
            <ProtectedRoute><PostListPage /></ProtectedRoute>
          } />
          <Route path="/easypost" element={
            <ProtectedRoute><EasyPostListPage /></ProtectedRoute>
          } />
          <Route path="/users/:userId/following" element={
            <ProtectedRoute><FollowingListPage /></ProtectedRoute>
          } />
          <Route path="/users/:userId/followers" element={
            <ProtectedRoute><FollowerListPage /></ProtectedRoute>
          } />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}
