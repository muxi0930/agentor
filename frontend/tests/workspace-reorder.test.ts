import assert from "node:assert/strict";
import test from "node:test";
import type { State } from "../bindings/agentor/internal/workspace/model/models.ts";
import { nodeMove } from "../src/features/workspace/reorder.ts";

const state = {
  projects: [
    { id: "p1", versions: [
      { id: "v1", features: [
        { id: "f1", steps: [{ id: "s1" }, { id: "s2" }, { id: "s3" }] },
        { id: "f2", steps: [{ id: "s4" }] },
        { id: "f3", steps: [] },
      ] },
      { id: "v2", features: [{ id: "f4", steps: [] }] },
      { id: "v3", features: [] },
    ] },
    { id: "p2", versions: [{ id: "v4", features: [] }] },
    { id: "p3", versions: [] },
  ],
  selectedId: "s2", documents: [],
} as unknown as State;

test("四种对象均可在同父级向前或向后拖动，并计算一次保存所需的距离", () => {
  for (const [kind, prefix] of [["project", "p"], ["version", "v"], ["feature", "f"], ["step", "s"]]) {
    assert.deepEqual(nodeMove(state, prefix + "1", prefix + "3", true), { kind, id: prefix + "1", offset: 2 });
    assert.deepEqual(nodeMove(state, prefix + "3", prefix + "1", false), { kind, id: prefix + "3", offset: -2 });
    assert.deepEqual(nodeMove(state, prefix + "1", prefix + "3", false), { kind, id: prefix + "1", offset: 1 });
    assert.deepEqual(nodeMove(state, prefix + "3", prefix + "1", true), { kind, id: prefix + "3", offset: -1 });
  }
});

test("跨项目、跨版本、跨功能以及不同对象类型不能互相放置", () => {
  for (const [source, target] of [["v1", "v4"], ["f1", "f4"], ["s1", "s4"], ["p1", "v1"], ["v1", "f1"], ["f1", "s1"]]) {
    assert.equal(nodeMove(state, source, target, false), undefined);
    assert.equal(nodeMove(state, source, target, true), undefined);
  }
});

test("自身、相邻原位置和不存在的对象不会触发保存", () => {
  assert.equal(nodeMove(state, "s1", "s1", false), undefined);
  assert.equal(nodeMove(state, "s1", "s2", false), undefined);
  assert.equal(nodeMove(state, "s2", "s1", true), undefined);
  assert.equal(nodeMove(state, "missing", "s1", false), undefined);
  assert.equal(nodeMove(state, "s1", "missing", false), undefined);
});

test("计算落点不会修改顺序或当前选择", () => {
  const before = JSON.stringify(state);
  nodeMove(state, "s1", "s3", true);
  assert.equal(JSON.stringify(state), before);
});
