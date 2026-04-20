// Haft cockpit shared primitives. Lifted from the Haft Design System UI kit
// (haft-design-system/project/ui_kits/haft-tauri/Primitives.jsx) and
// re-implemented as TypeScript React components using the existing Tailwind
// `@theme` tokens in src/index.css. The kit's bespoke kit.css is intentionally
// not imported — token names map cleanly to Tailwind utility classes.
//
// Visual contract is documented per-component; the design rules live in the
// design system README under haft-design-system/README.md.
export { Eyebrow, type EyebrowProps } from "./Eyebrow";
export { Button, type ButtonProps, type ButtonVariant } from "./Button";
export { Badge, type BadgeProps, type BadgeTone } from "./Badge";
export { Card, type CardProps, type CardState } from "./Card";
export { Input, type InputProps } from "./Input";
export {
  StatCard,
  type StatCardProps,
  type StatCardVariant,
} from "./StatCard";
export { MonoId, type MonoIdProps, type MonoIdTone } from "./MonoId";
export { Pill, type PillProps, type PillTone } from "./Pill";
