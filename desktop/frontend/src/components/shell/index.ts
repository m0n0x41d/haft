// Haft cockpit shell components. Lifted from the Haft Design System UI kit
// (haft-design-system/project/ui_kits/haft-tauri/Shell.jsx) and
// re-implemented as TypeScript React components consuming the existing
// Tailwind `@theme` tokens in src/index.css.
//
// These render the 48px icon rail, the 224px collapsible sidebar, and
// related chrome elements. Kept separate from primitives/ because they're
// shell-specific — not reusable outside the App.tsx orchestrator.
export { RailBtn, type RailBtnProps } from "./RailBtn";
export { SidebarTask, type SidebarTaskProps } from "./SidebarTask";
export {
  StatusDot,
  type StatusDotProps,
  type TaskStatus,
} from "./StatusDot";
