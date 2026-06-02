import { BookOpen } from 'lucide-react';

interface Props {
  title?: string;
  description?: string;
  action?: { label: string; onClick: () => void };
}

export default function EmptyState({
  title = '暂无内容',
  description = '这里还没有任何内容',
  action,
}: Props) {
  return (
    <div className="flex flex-col items-center justify-center py-20 text-scholar-muted">
      <BookOpen className="w-12 h-12 mb-4 opacity-40" />
      <h3 className="text-lg font-medium text-scholar-text mb-1">{title}</h3>
      <p className="text-sm mb-4">{description}</p>
      {action && (
        <button
          onClick={action.onClick}
          className="px-4 py-2 bg-scholar-accent text-white text-sm rounded-lg hover:bg-scholar-accentHover transition-colors"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}
