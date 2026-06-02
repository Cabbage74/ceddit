import { Loader2 } from 'lucide-react';

interface Props {
  text?: string;
}

export default function LoadingSpinner({ text = '加载中...' }: Props) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-scholar-muted">
      <Loader2 className="w-8 h-8 animate-spin mb-3" />
      <span className="text-sm">{text}</span>
    </div>
  );
}
