import "@xyflow/react/dist/style.css";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Background, ConnectionLineType, Controls, Handle, MarkerType, MiniMap, Position, ReactFlow, ReactFlowProvider, applyNodeChanges, useReactFlow, useUpdateNodeInternals, type Edge, type Node, type NodeProps } from "@xyflow/react";
import { Bot, Box, Check, Flag, GripVertical, Plus, X } from "lucide-react";
import type { ActionResult, ActionSpec } from "../../../bindings/agentor/internal/pipeline/model/models";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { actionName, actionNames, runLabels } from "./labels";
import { canConnect, connectNodes, newNode, removeNodes, type Graph, type GraphNode } from "./graph";
import { NodeEditor, type StepOption } from "./NodeEditor";

type CardNode = Node<{ node: GraphNode; entry: boolean; status: string; readOnly: boolean }, "pipeline">;
function NodeCard({ id, data, selected }: NodeProps<CardNode>) {
  const { node } = data;
  const update = useUpdateNodeInternals();
  const signature = node.branches.map(branch => branch.name).join("|");
  useEffect(() => update(id), [id, signature, node.kind, update]);
  const Icon = node.kind === "agent" ? Bot : node.kind === "end" ? Flag : Box;
  const ports = [...node.branches.map(branch => ({ name: branch.name, label: branch.description || branch.name })), ...(node.kind === "agent" ? [{ name: "__failure", label: "重试耗尽" }] : [])];
  return <div className={`pipeline-graph-node ${selected ? "is-selected" : ""} ${node.kind === "end" ? "is-end" : ""}`} data-status={data.status} data-kind={node.kind}>
    <Handle type="target" position={Position.Left} isConnectable={!data.readOnly} aria-label={`${node.id} 入口`} />
    <div className="flex items-center gap-2"><Icon className="size-4 shrink-0 text-primary" /><span className="truncate text-xs font-medium">{node.kind === "agent" ? "Agent" : node.kind === "end" ? "终点" : "Go 静态动作"}</span>{data.entry && <span className="ml-auto text-[10px] font-medium text-primary">入口</span>}</div>
    <p className="mt-2 truncate text-xs font-semibold" title={actionName(node.type)}>{actionName(node.type)}</p>
    {node.name && <p className="mt-1 truncate text-xs text-muted-foreground" title={node.name}>{node.name}</p>}
    {data.status !== "pending" && <p className={`pipeline-node-status mt-2 inline-flex items-center gap-1 rounded px-2 py-1 text-xs font-semibold ${data.status === "completed" ? "bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300" : data.status === "error" ? "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300" : ""}`}>{data.status === "completed" && <Check aria-hidden="true" className="size-3.5" />}{data.status === "error" && <X aria-hidden="true" className="size-3.5" />}{runLabels[data.status]}</p>}
    {ports.length > 0 && <div className="mt-3 space-y-1 border-t pt-2">{ports.map(port => <div key={port.name} className="relative flex h-5 justify-end text-[10px] text-muted-foreground"><span className="max-w-full truncate pr-2" title={port.label}>{port.name === "__failure" ? port.label : port.name}</span><Handle id={port.name} type="source" position={Position.Right} isConnectable={!data.readOnly} style={{ right: -19, top: "50%" }} aria-label={`${node.id} 分支 ${port.name}`} /></div>)}</div>}
  </div>;
}
const nodeTypes = { pipeline: NodeCard };
const emptyResults: ActionResult[] = [];
export function PipelineCanvas(props: CanvasProps) { return <ReactFlowProvider><Canvas {...props} /></ReactFlowProvider>; }
type CanvasProps = { task: Graph; specs: ActionSpec[]; onChange?: (task: Graph) => void; template?: boolean; steps?: StepOption[]; results?: ActionResult[]; currentNode?: string; runStatus?: string; onIssue?: (key: string, error: string) => void };
function Canvas({ task, specs, onChange, template, steps = [], results = emptyResults, currentNode, runStatus, onIssue = () => {} }: CanvasProps) {
  const readOnly = !onChange;
  const [selected, setSelected] = useState(""), [search, setSearch] = useState("");
  const [edgeSelection, setEdgeSelection] = useState<string[]>([]);
  const { screenToFlowPosition, getViewport } = useReactFlow();
  const [transientNodes, setTransientNodes] = useState<CardNode[]>([]);
  const nodes = useMemo<CardNode[]>(() => task.actions.map(node => ({ id: node.id, type: "pipeline", position: node.position || { x: 0, y: 0 }, selected: node.id === selected, data: { node, entry: task.entry === node.id, status: [...results].reverse().find(result => result.id === node.id)?.status || (currentNode === node.id ? runStatus || "running" : "pending"), readOnly } })), [task, selected, results, currentNode, runStatus, readOnly]);
  useEffect(() => setTransientNodes(nodes), [nodes]);
  const edges = useMemo<Edge[]>(() => task.actions.flatMap(node => [...node.branches, ...(node.failureTarget ? [{ name: "__failure", description: "重试耗尽", target: node.failureTarget }] : [])].filter(branch => branch.target).map(branch => ({ id: `${node.id}:${branch.name}`, source: node.id, target: branch.target, sourceHandle: branch.name, label: branch.name === "__failure" ? "重试耗尽" : branch.name, type: "default", selected: edgeSelection.includes(`${node.id}:${branch.name}`), markerEnd: { type: MarkerType.ArrowClosed }, style: { stroke: branch.name === "__failure" ? "var(--destructive)" : "var(--primary)" }, labelStyle: { fill: "var(--foreground)", fontSize: 11, transform: "translateY(-14px)" }, labelShowBg: false }))), [task, edgeSelection]);
  const add = useCallback((type: string, position?: { x: number; y: number }) => {
    if (!onChange) return;
    const viewport = getViewport();
    const node = newNode(type, specs, position || { x: (100 - viewport.x) / viewport.zoom, y: (100 - viewport.y) / viewport.zoom });
    onChange({ ...task, entry: task.entry || node.id, actions: [...task.actions, node] });
    setSelected(node.id);
  }, [task, specs, getViewport, onChange]);
  const selectedNode = task.actions.find(node => node.id === selected);
  const palette = [{ name: "agent.run", description: "提示词与验收标准" }, { name: "agent.resolve-conflicts", description: "连接合并节点的 diverged 分支，解决冲突并完成合并提交" }, ...[...specs].sort((a, b) => a.name.localeCompare(b.name, "en")), { name: "end.success", description: "以成功结束整条流水线" }, { name: "end.failure", description: "以失败结束整条流水线" }].filter(item => `${item.name} ${actionNames[item.name]}`.toLowerCase().includes(search.toLowerCase()));
  return <div className={`pipeline-canvas-layout ${readOnly ? "is-readonly" : ""}`}>
    {!readOnly && <aside className="pipeline-palette"><div className="space-y-2 border-b p-3"><Input className="h-8" aria-label="搜索节点" placeholder="搜索、拖入或点击添加" value={search} onChange={event => setSearch(event.target.value)} /></div><div className="space-y-1 overflow-y-auto p-2">{palette.map(item => <button key={item.name} type="button" draggable onDragStart={event => { event.dataTransfer.setData("application/agentor-node", item.name); event.dataTransfer.effectAllowed = "copy"; }} onClick={() => add(item.name)} className="pipeline-palette-item" title={item.description}><GripVertical className="size-3 shrink-0 text-muted-foreground" /><span className="min-w-0 flex-1"><span className="block truncate text-[11px] font-medium">{item.name}</span><span className="block truncate text-[11px] text-muted-foreground">{actionNames[item.name]}</span></span><Plus className="size-3 shrink-0" /></button>)}</div></aside>}
    <div className="pipeline-canvas" onDragOver={event => { if (!readOnly) { event.preventDefault(); event.dataTransfer.dropEffect = "copy"; } }} onDrop={event => { event.preventDefault(); const type = event.dataTransfer.getData("application/agentor-node"); if (palette.some(item => item.name === type)) add(type, screenToFlowPosition({ x: event.clientX, y: event.clientY })); }}>
      <ReactFlow<CardNode> nodes={transientNodes} edges={edges} nodeTypes={nodeTypes} minZoom={0.25} maxZoom={1.75} fitView fitViewOptions={{ padding: 0.25, maxZoom: 1 }} nodesDraggable={!readOnly} nodesConnectable={!readOnly} deleteKeyCode={readOnly || selectedNode ? null : ["Backspace", "Delete"]} onNodeClick={(_, node) => { setSelected(node.id); setEdgeSelection([]); }} onPaneClick={() => { setSelected(""); setEdgeSelection([]); }} onEdgeClick={(_, edge) => { setEdgeSelection([edge.id]); setSelected(""); }}
        onNodesChange={changes => { setTransientNodes(current => applyNodeChanges(changes, current)); const deleted = changes.filter(change => change.type === "remove").map(change => change.id); if (deleted.length) onChange?.(removeNodes(task, deleted)); }}
        onNodeDragStop={(_, moved) => onChange?.({ ...task, actions: task.actions.map(node => node.id === moved.id ? { ...node, position: moved.position } : node) })}
        connectionLineType={ConnectionLineType.Bezier}
        connectionRadius={24}
        onConnectEnd={(event, connection) => {
          const handle = connection.fromHandle;
          const target = event.target;
          if (connection.isValid || connection.toNode || handle?.type !== "source" || !handle.id || !(target instanceof Element) || !target.classList.contains("react-flow__pane")) return;
          if (edges.some(edge => edge.source === handle.nodeId && edge.sourceHandle === handle.id)) {
            onChange?.(connectNodes(task, handle.nodeId, handle.id, ""));
            setEdgeSelection([]);
          }
        }}
        isValidConnection={connection => !!connection.source && !!connection.target && canConnect(task, connection.source, connection.target)}
        onConnect={connection => { if (connection.sourceHandle) onChange?.(connectNodes(task, connection.source, connection.sourceHandle, connection.target)); }}
        onEdgesDelete={deleted => { let next = task; for (const edge of deleted) next = connectNodes(next, edge.source, edge.sourceHandle || "", ""); onChange?.(next); }}
        onEdgesChange={changes => { const selectedEdges = changes.flatMap(change => change.type === "select" && change.selected ? [change.id] : []); if (selectedEdges.length) setEdgeSelection(selectedEdges); }}>
        <Background gap={20} size={1} /><Controls showInteractive={false} /><MiniMap style={{ width: 110, height: 76 }} pannable zoomable nodeColor="var(--primary)" maskColor="color-mix(in srgb, var(--muted) 55%, transparent)" />
      </ReactFlow>
      {!task.actions.length && <div className="pointer-events-none absolute inset-0 flex items-center justify-center text-center"><div className="rounded-xl border bg-background/90 p-5"><p className="text-sm font-medium">从节点开始编排</p><p className="mt-2 text-xs text-muted-foreground">将左侧节点拖入画布，再拖动出口连接下一个节点。</p></div></div>}
      {!readOnly && edgeSelection.length > 0 && <div className="absolute left-3 top-3 z-10"><Button size="sm" variant="outline" onClick={() => { let next = task; for (const edge of edges.filter(edge => edgeSelection.includes(edge.id))) next = connectNodes(next, edge.source, edge.sourceHandle || "", ""); onChange?.(next); setEdgeSelection([]); }}><X />移除选中连线</Button></div>}
      {selectedNode && <NodeEditor key={selectedNode.id} node={selectedNode} task={task} specs={specs} template={template} steps={steps} readOnly={readOnly} currentStatus={selectedNode.id === currentNode ? runStatus : undefined} result={[...results].reverse().find(result => result.id === selected)} onChange={node => onChange?.({ ...task, actions: task.actions.map(item => item.id === node.id ? node : item) })} onEntry={() => onChange?.({ ...task, entry: selected })} onDelete={() => { onChange?.(removeNodes(task, [selected])); setSelected(""); }} onClose={() => setSelected("")} onIssue={onIssue} />}
      <div className="pointer-events-none absolute bottom-3 left-16 rounded bg-background/85 px-2 py-1 text-[10px] text-muted-foreground">{task.actions.length} 个节点 · 单一路径执行{!readOnly && " · 拖动连线 / 点击节点配置"}</div>
    </div>
  </div>;
}
