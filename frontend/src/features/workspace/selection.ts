import type { Feature, Project, State, Step, Version } from "../../../bindings/agentor/internal/workspace/model/models";

export type NodeKind = "project" | "version" | "feature" | "step";
export type Selection = { kind: NodeKind; project: Project; version?: Version; feature?: Feature; step?: Step };

export function locate(state: State, id: string): Selection | undefined {
  for (const project of state.projects || []) {
    if (project.id === id) return {kind:"project",project};
    for (const version of project.versions || []) {
      if (version.id === id) return {kind:"version",project,version};
      for (const feature of version.features || []) {
        if (feature.id === id) return {kind:"feature",project,version,feature};
        for (const step of feature.steps || []) if (step.id === id) return {kind:"step",project,version,feature,step};
      }
    }
  }
}

export function nodeIdentity(selection: Selection) {
  return selection.kind === "project" ? { id:selection.project.id, title:selection.project.name } : selection.kind === "version" ? {id:selection.version!.id,title:selection.version!.name} : selection.kind === "feature" ? {id:selection.feature!.id,title:selection.feature!.title} : {id:selection.step!.id,title:selection.step!.title};
}
