/**
 * Ported from Seaglass frontend/src/registry/MetricStrip.tsx; adapted to render
 * A2UI child IDs instead of registry data objects.
 */
import type { CSSProperties, ReactNode } from "react";

export function StatGroup({ children, columns }: { children: ReactNode[]; columns?: number }) {
  const count = Math.min(4, Math.max(1, columns ?? children.length));
  return (
    <div
      className="extended-stat-group"
      style={{
        "--extended-stat-columns": count,
        gridTemplateColumns: `repeat(auto-fit, minmax(min(100%, max(150px, calc((100% - ${(count - 1) * 12}px) / ${count}))), 1fr))`,
      } as CSSProperties}
    >
      {children}
    </div>
  );
}
