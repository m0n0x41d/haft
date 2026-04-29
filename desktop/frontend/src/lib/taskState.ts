import type { TaskState } from "./api";

export interface TaskStatusEvent {
  id: string;
  status: string;
  error_message: string;
}

export function mergeTaskStatusEvent(
  current: TaskState[],
  event: TaskStatusEvent,
): TaskState[] {
  const existingIndex = current.findIndex((task) => task.id === event.id);

  if (existingIndex === -1) {
    return current;
  }

  const merged = [...current];
  const existingTask = merged[existingIndex];

  merged[existingIndex] = {
    ...existingTask,
    status: event.status,
    error_message: event.error_message,
  };

  return merged;
}
