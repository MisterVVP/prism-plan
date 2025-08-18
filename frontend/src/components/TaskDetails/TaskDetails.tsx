import type { Task, Category } from '../../types';
import { palette } from '../../palette';

const titleMap: Record<Category, string> = {
  critical: 'Critical',
  fun: 'Fun',
  important: 'Important',
  normal: 'Normal'
};

export default function TaskDetails({ task, onBack }: { task: Task; onBack: () => void }) {
  return (
    <div className="mx-auto max-w-md">
      <button
        type="button"
        onClick={onBack}
        className="mb-4 flex items-center gap-2 text-sm font-medium text-gray-600 hover:text-gray-800"
      >
        ← Back
      </button>
      <div
        className="rounded-lg border-l-4 bg-white p-4 shadow"
        style={{ borderColor: palette[task.category] }}
      >
        <h2 className="mb-2 text-xl font-bold">{task.title}</h2>
        <div className="mb-2 flex items-center gap-2 text-sm text-gray-500">
          <span className="h-2 w-2 rounded-full" style={{ backgroundColor: palette[task.category] }} />
          {titleMap[task.category]}
        </div>
        {task.notes && (
          <p className="whitespace-pre-wrap text-sm text-gray-700">{task.notes}</p>
        )}
      </div>
    </div>
  );
}
