import { useEffect, useMemo, useState } from "react";
import {
  Background,
  Controls,
  MarkerType,
  ReactFlow,
  type Edge,
  type Node,
  Position,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Step } from "@/types/apiV2";

interface StepsDagProps {
  flowId: number;
  steps: Step[];
  selectedStepId: number | null;
  onSelectStep?: (stepId: number) => void;
}

const safeNumber = (value: unknown): number | null => {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  if (typeof value === "string") {
    const parsed = Number.parseInt(value, 10);
    if (Number.isFinite(parsed)) {
      return parsed;
    }
  }
  return null;
};

const buildDepthMap = (steps: Step[]): Record<number, number> => {
  const byID = new Map(steps.map((step) => [step.id, step] as const));
  const memo = new Map<number, number>();
  const visiting = new Set<number>();

  const visit = (id: number): number => {
    if (memo.has(id)) {
      return memo.get(id) ?? 0;
    }
    if (visiting.has(id)) {
      return 0;
    }
    visiting.add(id);
    const step = byID.get(id);
    const deps = Array.isArray(step?.depends_on) ? step?.depends_on : [];
    if (!step || deps.length === 0) {
      memo.set(id, 0);
      visiting.delete(id);
      return 0;
    }
    const depth =
      Math.max(
        ...deps.map((dep) => (byID.has(dep) ? visit(dep) : 0)),
      ) + 1;
    memo.set(id, depth);
    visiting.delete(id);
    return depth;
  };

  steps.forEach((step) => {
    visit(step.id);
  });

  return Object.fromEntries(memo.entries());
};

const buildNodes = (
  steps: Step[],
  selectedStepId: number | null,
  overrides: Record<number, { x: number; y: number }>,
): Node[] => {
  const depthMap = buildDepthMap(steps);
  const columns = new Map<number, number[]>();
  steps.forEach((step) => {
    const depth = depthMap[step.id] ?? 0;
    const column = columns.get(depth) ?? [];
    column.push(step.id);
    columns.set(depth, column);
  });

  return steps.map((step) => {
    const id = String(step.id);
    const depth = depthMap[step.id] ?? 0;
    const siblings = columns.get(depth) ?? [step.id];
    const index = siblings.indexOf(step.id);
    const selected = step.id === selectedStepId;
    const override = overrides[step.id];
    return {
      id,
      position: override ?? {
        x: depth * 300 + 32,
        y: index * 120 + 32,
      },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        label: (
          <div className="space-y-1 text-left">
            <div className="flex items-center justify-between gap-2">
              <div className="text-xs font-semibold text-slate-500">#{step.id}</div>
              <Badge variant="outline" className="border-slate-200 bg-slate-50 text-[11px] text-slate-600">
                {step.type}
              </Badge>
            </div>
            <div className="text-sm font-semibold text-slate-900">{step.name || "未命名 Step"}</div>
            {Array.isArray(step.depends_on) && step.depends_on.length > 0 ? (
              <div className="text-[11px] text-slate-500">depends: {step.depends_on.join(", ")}</div>
            ) : null}
          </div>
        ),
      },
      style: {
        width: 240,
        borderRadius: 16,
        border: selected ? "2px solid rgb(199, 210, 254)" : "1px solid rgb(226, 232, 240)",
        boxShadow: selected ? "0 0 0 4px rgba(99, 102, 241, 0.10)" : "0 8px 24px rgba(15, 23, 42, 0.08)",
        background: "#ffffff",
        padding: 12,
      },
    };
  });
};

const buildEdges = (steps: Step[]): Edge[] => {
  const valid = new Set(steps.map((step) => step.id));
  return steps.flatMap((step) =>
    (Array.isArray(step.depends_on) ? step.depends_on : [])
      .filter((dep) => valid.has(dep))
      .map((dep) => ({
        id: `${dep}->${step.id}`,
        source: String(dep),
        target: String(step.id),
        markerEnd: { type: MarkerType.ArrowClosed, color: "#94a3b8" },
        style: { stroke: "#94a3b8", strokeWidth: 1.5 },
      })),
  );
};

const storageKey = (flowId: number) => `v2:flow:${flowId}:dag_positions`;

export default function StepsDag({ flowId, steps, selectedStepId, onSelectStep }: StepsDagProps) {
  const [positions, setPositions] = useState<Record<number, { x: number; y: number }>>({});

  useEffect(() => {
    try {
      const raw = localStorage.getItem(storageKey(flowId));
      if (!raw) {
        setPositions({});
        return;
      }
      const parsed = JSON.parse(raw) as Record<string, unknown>;
      const next: Record<number, { x: number; y: number }> = {};
      Object.entries(parsed).forEach(([key, value]) => {
        const stepId = safeNumber(key);
        const pos = value as { x?: unknown; y?: unknown };
        if (stepId == null) {
          return;
        }
        const x = safeNumber(pos?.x);
        const y = safeNumber(pos?.y);
        if (x == null || y == null) {
          return;
        }
        next[stepId] = { x, y };
      });
      setPositions(next);
    } catch {
      setPositions({});
    }
  }, [flowId]);

  const nodes = useMemo(() => buildNodes(steps, selectedStepId, positions), [steps, selectedStepId, positions]);
  const edges = useMemo(() => buildEdges(steps), [steps]);

  if (steps.length === 0) {
    return (
      <div className="rounded-2xl border border-slate-200 bg-white p-4">
        <p className="text-sm font-semibold text-slate-900">DAG 视图</p>
        <p className="mt-2 text-sm text-slate-500">还没有 Step，先在左侧创建 Step。</p>
      </div>
    );
  }

  return (
    <div className="rounded-2xl border border-slate-200 bg-white p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-sm font-semibold text-slate-900">DAG 视图（编排预览）</p>
          <p className="mt-1 text-xs leading-5 text-slate-500">
            可拖拽调整布局（仅本地保存）。依赖关系来自 Step 的 depends_on；后端暂未提供更新依赖的接口。
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          onClick={() => {
            setPositions({});
            localStorage.removeItem(storageKey(flowId));
          }}
        >
          重置布局
        </Button>
      </div>

      <div className="mt-3 h-[520px] overflow-hidden rounded-2xl border border-slate-200 bg-slate-50">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          fitView
          nodesDraggable
          nodesConnectable={false}
          elementsSelectable
          onNodeClick={(_, node) => {
            const id = safeNumber(node.id);
            if (id != null) {
              onSelectStep?.(id);
            }
          }}
          onNodeDragStop={(_, node) => {
            const id = safeNumber(node.id);
            if (id == null) {
              return;
            }
            const next = { ...positions, [id]: node.position };
            setPositions(next);
            try {
              localStorage.setItem(storageKey(flowId), JSON.stringify(next));
            } catch {
              // ignore
            }
          }}
          proOptions={{ hideAttribution: true }}
        >
          <Background color="#cbd5e1" gap={18} />
          <Controls showInteractive={false} />
        </ReactFlow>
      </div>
    </div>
  );
}
