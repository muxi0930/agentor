import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { createServer } from "vite";

const server = await createServer({
  configFile: false,
  root: fileURLToPath(new URL("../", import.meta.url)),
  resolve: { alias: { "@": fileURLToPath(new URL("../src", import.meta.url)) } },
  plugins: [{
    name: "workspace-completion-test",
    resolveId(id) { if (id === "virtual:completion-test") return "\0completion-test"; },
    load(id) {
      if (id === "\0completion-test") return `
        export let values = [], cursor = 0, calls = [], failure = false;
        export function reset(state) { values = [{}, "versions", state, false, ""]; calls = []; failure = false; }
        export function begin() { cursor = 0; }
        export function fail() { failure = true; }
        export function useEffect() {}
        export function useState(initial) {
          const index = cursor++;
          if (!(index in values)) values[index] = initial;
          return [values[index], value => { values[index] = typeof value === "function" ? value(values[index]) : value; }];
        }
        export async function CompleteStep(id) {
          calls.push(id);
          if (failure) throw new Error("保存失败");
          const next = structuredClone(values[2]);
          next.projects[0].versions[0].features[0].steps[0].completed = true;
          return next;
        }
      `;
    },
    transform(source, id) {
      if (!id.endsWith("/features/workspace/WorkspacePage.tsx")) return;
      return source.replace('from "react";', 'from "virtual:completion-test";')
        .replace('from "../../../bindings/agentor/internal/workspace/service";', 'from "virtual:completion-test";');
    },
  }],
  server: { middlewareMode: true, watch: null, ws: false },
  appType: "custom",
});
after(() => server.close());
const hooks = await server.ssrLoadModule("virtual:completion-test");
const { WorkspacePage } = await server.ssrLoadModule("/src/features/workspace/WorkspacePage.tsx");
const state = () => ({ selectedId: "s", documents: [], projects: [{ id: "p", name: "项目", versions: [{ id: "v", name: "版本", features: [{ id: "f", title: "功能", steps: [{ id: "s", title: "步骤", prompt: "", completed: false }] }] }] }] });
function find(element, predicate) {
  if (!element || typeof element !== "object") return;
  if (predicate(element)) return element;
  for (const child of [element.props?.children].flat(Infinity)) {
    const result = find(child, predicate);
    if (result) return result;
  }
}
function render() {
  hooks.begin();
  let actions;
  function Capture() {
    const tree = WorkspacePage({ active: true, settings: { mode: "workspace" }, headerContainer: null, sidebarContainer: null });
    actions = find(tree, node => node.props?.header?.actions)?.props.header.actions;
    return actions;
  }
  const html = renderToStaticMarkup(createElement(Capture));
  const button = find(actions, node => node.props?.children?.includes?.("标记完成"));
  return { html, button };
}

test("标记完成位于编辑左侧，点击调用接口并刷新为禁用状态", async () => {
  hooks.reset(state());
  let view = render();
  assert.ok(view.html.indexOf("标记完成") < view.html.indexOf("编辑"));
  assert.equal(view.button.props.disabled, false);
  view.button.props.onClick();
  assert.equal(hooks.values[3], true);
  assert.equal(render().button.props.disabled, true);
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(hooks.calls, ["s"]);
  view = render();
  assert.equal(view.button.props.disabled, true);
  assert.match(view.html, /disabled/);
});

test("重新读取已完成步骤后禁用按钮，保存失败保留未完成状态并提示错误", async () => {
  const saved = state();
  saved.projects[0].versions[0].features[0].steps[0].completed = true;
  hooks.reset(saved);
  assert.equal(render().button.props.disabled, true);
  hooks.reset(state());
  hooks.fail();
  render().button.props.onClick();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(render().button.props.disabled, false);
  assert.equal(hooks.values[4], "保存失败");
});
