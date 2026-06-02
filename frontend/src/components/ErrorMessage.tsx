import { AlertCircle } from 'lucide-react';

interface Props {
  message: string;
  onRetry?: () => void;
}

export default function ErrorMessage({ message, onRetry }: Props) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-scholar-muted">
      <AlertCircle className="w-10 h-10 text-red-400 mb-3" />
      <p className="text-sm mb-4">{message}</p>
      {onRetry && (
        <button
          onClick={onRetry}
          className="text-sm text-scholar-accent hover:text-scholar-accentHover underline underline-offset-2"
        >
          重试
        </button>
      )}
    </div>
  );
}
