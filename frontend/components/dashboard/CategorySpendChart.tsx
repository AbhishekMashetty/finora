"use client";

// "Spend by category" bar chart — composition of actual spend across
// categories, distinct from the budget-vs-actual bullet bars below it on
// the reports page (that's target-vs-actual per category; this is "where
// did the money go" at a glance). Built per the dataviz skill: categorical
// identity -> categorical color, fixed order, never cycled past 8 slots (a
// 9th+ category folds into "Other"), direct labels on the axis instead of a
// separate legend, recessive gridlines, rounded bar ends anchored to the
// baseline.

import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
  type TooltipContentProps,
} from "recharts";
import type { NameType, ValueType } from "recharts/types/component/DefaultTooltipContent";
import { formatNumber } from "@/lib/format";
import type { CategorySummary } from "@/lib/types";

const CHART_COLOR_VARS = [
  "var(--color-chart-1)",
  "var(--color-chart-2)",
  "var(--color-chart-3)",
  "var(--color-chart-4)",
  "var(--color-chart-5)",
  "var(--color-chart-6)",
  "var(--color-chart-7)",
  "var(--color-chart-8)",
];

interface ChartRow {
  name: string;
  actual: number;
}

function buildChartData(categories: CategorySummary[]): ChartRow[] {
  const sorted = [...categories].sort((a, b) => b.actual - a.actual);
  if (sorted.length <= 8) {
    return sorted.map((c) => ({ name: c.category, actual: c.actual }));
  }
  const top = sorted.slice(0, 7).map((c) => ({ name: c.category, actual: c.actual }));
  const otherTotal = sorted.slice(7).reduce((sum, c) => sum + c.actual, 0);
  return [...top, { name: "Other", actual: otherTotal }];
}

function ChartTooltip({ active, payload }: TooltipContentProps<ValueType, NameType>) {
  if (!active || !payload?.length) return null;
  const row = payload[0].payload as ChartRow;
  return (
    <div className="rounded-control border border-hairline bg-surface-raised px-3 py-2 text-xs shadow-[var(--shadow-popover)]">
      <p className="font-medium capitalize text-ink-primary">{row.name}</p>
      <p className="mt-0.5 tabular-nums text-ink-secondary">{formatNumber(row.actual)}</p>
    </div>
  );
}

export function CategorySpendChart({ categories }: { categories: CategorySummary[] }) {
  const data = buildChartData(categories);
  const height = Math.max(180, data.length * 44);

  return (
    <div style={{ height }}>
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} layout="vertical" margin={{ top: 4, right: 24, bottom: 4, left: 4 }}>
          <CartesianGrid horizontal={false} stroke="var(--color-grid)" />
          <XAxis
            type="number"
            tickFormatter={(v: number) => formatNumber(v, { maximumFractionDigits: 0 })}
            tick={{ fill: "var(--color-ink-muted)", fontSize: 12 }}
            axisLine={false}
            tickLine={false}
          />
          <YAxis
            type="category"
            dataKey="name"
            width={110}
            tick={{ fill: "var(--color-ink-secondary)", fontSize: 12 }}
            tickFormatter={(v: string) => (v.length > 14 ? `${v.slice(0, 13)}…` : v)}
            axisLine={false}
            tickLine={false}
            className="capitalize"
          />
          <Tooltip cursor={{ fill: "var(--color-plane)" }} content={ChartTooltip} />
          <Bar dataKey="actual" radius={[0, 4, 4, 0]} maxBarSize={28}>
            {data.map((row, i) => (
              <Cell key={row.name} fill={CHART_COLOR_VARS[i % CHART_COLOR_VARS.length]} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
