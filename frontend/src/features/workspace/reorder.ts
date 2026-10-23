import type { State } from "../../../bindings/agentor/internal/workspace/model/models";
import { locate } from "./selection.ts";

export function nodeMove(state: State, sourceId: string, targetId: string, after: boolean) {
  const source = locate(state, sourceId);
  if (!source || sourceId === targetId) return;
  const siblings = source.kind === "project" ? state.projects : source.kind === "version" ? source.project.versions : source.kind === "feature" ? source.version?.features : source.feature?.steps;
  const ids = siblings?.map(item => item.id) || [];
  const from = ids.indexOf(sourceId), target = ids.indexOf(targetId);
  if (from < 0 || target < 0) return;
  const to = target + Number(after) - Number(from < target);
  if (to !== from) return { kind: source.kind, id: sourceId, offset: to - from };
}
