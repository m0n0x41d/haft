import { useState } from "react";

import { Archive, MoreHorizontal } from "lucide-react";

import type { TaskState } from "../../lib/api";
import { StatusDot } from "./StatusDot";

export interface SidebarTaskProps {
  task: TaskState;
  selected: boolean;
  onSelect: () => void;
  onArchive: () => void;
}

/**
 * Sidebar task row. Shows status dot + truncated title + context menu
 * trigger (hidden until hover — group affordance). Context menu only
 * offers Archive; delete is intentionally absent per design constraint
 * (no destructive primary actions in the sidebar).
 */
export function SidebarTask({
  task,
  selected,
  onSelect,
  onArchive,
}: SidebarTaskProps) {
  const [menuOpen, setMenuOpen] = useState(false);

  return (
    <div className="group relative">
      <div
        role="button"
        tabIndex={0}
        onClick={onSelect}
        onContextMenu={(e) => {
          e.preventDefault();
          setMenuOpen(!menuOpen);
        }}
        onKeyDown={(e) => {
          if (e.key === "Enter") onSelect();
        }}
        className={`flex w-full cursor-pointer items-center gap-1.5 rounded px-2 py-1 text-xs transition-colors ${
          selected
            ? "bg-surface-2 text-text-primary"
            : "text-text-secondary hover:bg-surface-2"
        }`}
      >
        <StatusDot status={task.status} />
        <span className="flex-1 truncate text-left">{task.title}</span>
        <span
          role="button"
          tabIndex={0}
          onClick={(e) => {
            e.stopPropagation();
            setMenuOpen(!menuOpen);
          }}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.stopPropagation();
              setMenuOpen(!menuOpen);
            }
          }}
          className="shrink-0 cursor-pointer p-0.5 text-text-muted opacity-0 transition-opacity hover:text-text-primary group-hover:opacity-100"
        >
          <MoreHorizontal size={12} />
        </span>
      </div>

      {menuOpen ? (
        <>
          <div
            className="fixed inset-0 z-40"
            onClick={() => setMenuOpen(false)}
          />
          <div className="absolute right-0 top-full z-50 mt-1 w-36 rounded-lg border border-border bg-surface-1 py-1 shadow-xl">
            <button
              onClick={() => {
                setMenuOpen(false);
                onArchive();
              }}
              className="flex w-full items-center gap-2 px-3 py-1.5 text-xs text-text-secondary transition-colors hover:bg-surface-2"
            >
              <Archive size={12} />
              Archive
            </button>
          </div>
        </>
      ) : null}
    </div>
  );
}
