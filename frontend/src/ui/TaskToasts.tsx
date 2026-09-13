import { useEffect, useState } from "react";
import type { TaskItem } from "../reducer";

const TASK_LABELS: Record<TaskItem["status"], string> = {
  dispatched: "Queued",
  researching: "researching…",
  generating: "Generating",
  rendered: "Rendered",
  failed: "Failed",
};

export function TaskToasts({ tasks }: { tasks: Record<string, TaskItem> }) {
  const [hidden, setHidden] = useState<Set<string>>(() => new Set());
  const taskList = Object.values(tasks);

  useEffect(() => {
    const timers = taskList
      .filter((task) => task.status === "rendered" && !hidden.has(task.taskId))
      .map((task) =>
        window.setTimeout(() => {
          setHidden((current) => new Set(current).add(task.taskId));
        }, 2_600),
      );
    return () => timers.forEach((timer) => window.clearTimeout(timer));
  }, [tasks, hidden]);

  const visible = taskList.filter((task) => task.status === "failed" || !hidden.has(task.taskId));
  if (visible.length === 0) return null;

  return (
    <aside className="task-toasts" aria-label="Task activity" aria-live="polite">
      {visible.map((task) => {
        const content = (
          <>
            <span className="task-spinner" aria-hidden="true" />
            <span className="task-copy">
              <strong>{TASK_LABELS[task.status]}</strong>
              <span>{task.detail ?? task.surfaceId ?? task.taskId}</span>
            </span>
          </>
        );
        return task.status === "failed" ? (
          <details key={task.taskId} className="task-toast failed">
            <summary>{content}</summary>
            <p>{task.detail ?? `Task ${task.taskId} failed.`}</p>
          </details>
        ) : (
          <div key={task.taskId} className={`task-toast ${task.status}`} title={task.detail}>
            {content}
          </div>
        );
      })}
    </aside>
  );
}
