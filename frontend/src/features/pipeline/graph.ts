import type { Action, Branch, InputField, Task } from "../../../bindings/agentor/internal/pipeline/model/models";
import type { ActionSpec } from "../../../bindings/agentor/internal/pipeline/model/models";

export function newNode(type: string, specs: ActionSpec[], position: { x: number; y: number }): GraphNode {
  const kind = type.startsWith("end.") ? "end" : (type === "agent.run" || type === "agent.resolve-conflicts") ? "agent" : "static";
  const spec = specs.find(item => item.name === type);
  return { id: `${type.replace(/\./g, "-")}-${crypto.randomUUID().slice(0, 6)}`, type, kind, name: "", params: {}, position,
    branches: kind === "end" ? [] : (spec?.branches || [{ name: "success", description: "验收完成", target: "" }]).map(branch => ({ ...branch, target: "" })),
    prompt: type === "agent.resolve-conflicts" ? "读取输入 worktree 的 repository、directory、sourceBranch 和 branch。在独立工作区 directory 中确认当前分支为 branch，检查双方工作区干净后，将 sourceBranch 合并到当前分支。出现冲突时理解双方修改意图，保留双方有效改动，逐个解决并暂存冲突文件，完成合并提交。若已有进行中的合并，先检查并继续处理。不得丢弃任一方改动，不得重置、强推或修改来源工作区；无法判断时如实失败。提交标题使用 fix: Resolve merge conflicts，并追加 Co-authored-by: Codex <codex@openai.com>。完成后通过 pipeline_complete_node 汇报 outcome=success、branch=success，output 为 JSON：summary（处理摘要）、resolvedFiles（已解决文件数组）、unresolvedFiles（未解决文件数组）、mergeCommit（独立工作区 HEAD）。无法完成时汇报 outcome=failure，并说明原因及未解决文件。宿主将在成功后将独立工作区快进合入来源分支并登记合并状态。" : "", criteria: type === "agent.resolve-conflicts" ? "独立工作区无未合并文件，无待提交修改，合并提交已完成，来源分支为当前 HEAD 的祖先；output 如实列出处理结果。不得仅删除冲突标记就声称成功。" : "", permission: "workspace-write", maxRetries: 0, timeoutSeconds: 900, failureTarget: "", stepSlot: "", stepId: "" };
}
export function newTask(projectId: string): Graph {
  return { id: "", projectId, templateId: "", directory: "", name: "", entry: "", inputs: [], actions: [] };
}
export function canConnect(task: Graph, source: string, target: string): boolean {
  if (source === target || !task.actions.some(node => node.id === target) || task.actions.find(node => node.id === source)?.kind === "end") return false;
  const seen = new Set<string>();
  const reaches = (id: string): boolean => {
    if (id === source) return true;
    if (seen.has(id)) return false;
    seen.add(id);
    const node = task.actions.find(item => item.id === id);
    return !!node && [...(node.branches || []).map(branch => branch.target), node.failureTarget].filter(Boolean).some(reaches);
  };
  return !reaches(target);
}
export function connectNodes(task: Graph, source: string, handle: string, target: string): Graph {
  if (target && !canConnect(task, source, target)) return task;
  return { ...task, actions: task.actions.map(node => node.id !== source ? node : handle === "__failure" ? { ...node, failureTarget: target } : { ...node, branches: node.branches.map(branch => branch.name === handle ? { ...branch, target } : branch) }) };
}
export function removeNodes(task: Graph, ids: string[]): Graph {
  return { ...task, entry: ids.includes(task.entry) ? "" : task.entry, actions: task.actions.filter(node => !ids.includes(node.id)).map(node => ({ ...node, failureTarget: ids.includes(node.failureTarget) ? "" : node.failureTarget, branches: node.branches.map(branch => ({ ...branch, target: ids.includes(branch.target) ? "" : branch.target })) })) };
}

export type GraphNode = Omit<Action, "branches" | "params"> & { branches: Branch[]; params: Record<string, unknown> };
export type Graph = Omit<Task, "actions" | "inputs"> & { actions: GraphNode[]; inputs: InputField[] };
export function normalizeTask(task: Task): Graph { return { ...task, inputs: task.inputs || [], actions: (task.actions || []).map(node => ({ ...node, params: node.params || {}, branches: node.branches || [] })) }; }
