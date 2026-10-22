import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const server = await createServer({
  configFile: false,
  root: fileURLToPath(new URL("../", import.meta.url)),
  resolve: { alias: { "@": fileURLToPath(new URL("../src", import.meta.url)) } },
  plugins: [{
    name: "data-directory-restart-test",
    resolveId(id) { if (id === "virtual:restart-test") return "\0restart-test"; },
    load(id) {
      if (id === "\0restart-test") return `
        export let values = [], cursor = 0, calls = [], failure = false;
        export function reset() { values = [false, false, { running: 0, queued: 0, tasks: [] }, "", false]; calls = []; failure = false; }
        export function begin() { cursor = 0; }
        export function fail() { failure = true; }
        export function useEffect() {}
        export function useState(initial) {
          const index = cursor++;
          if (!(index in values)) values[index] = initial;
          return [values[index], value => { values[index] = value; }];
        }
        export async function RestartDataDirectory(target) {
          calls.push(target);
          if (failure) throw new Error("配置文件无法写入");
        }
      `;
    },
    transform(source, id) {
      if (!id.endsWith("/app/ExitApplication.tsx")) return;
      return source.replace('from "react";', 'from "virtual:restart-test";')
        .replace('from "../../bindings/agentor/internal/lifecycle/service";', 'from "virtual:restart-test";');
    },
  }],
  server: { middlewareMode: true, watch: null, ws: false },
  appType: "custom",
});
after(() => server.close());
const hooks = await server.ssrLoadModule("virtual:restart-test");
const { ExitApplication } = await server.ssrLoadModule("/src/app/ExitApplication.tsx");
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
  return ExitApplication({ dataDirectory: { current: "C:/Users/测试/old", target: "D:/资料/agentor" } });
}
function button(tree, label) { return find(tree, node => [node.props?.children].flat(Infinity).includes(label)); }

test("选择目录后先确认，显示数据风险和两个目录，确认后才保存并重启", async () => {
  hooks.reset();
  button(render(), "应用并重启").props.onClick();
  assert.equal(hooks.values[1], true);
  assert.deepEqual(hooks.calls, []);
  const tree = render();
  assert.ok(find(tree, node => typeof node.props?.children === "string" && node.props.children.includes("现有数据可能丢失") && node.props.children.includes("自行拷贝")));
  assert.ok(find(tree, node => node.props?.children === "C:/Users/测试/old"));
  assert.ok(find(tree, node => node.props?.children === "D:/资料/agentor"));
  const confirm = button(tree, "确认切换并重启");
  assert.equal(confirm.props.disabled, false);
  confirm.props.onClick();
  await new Promise(resolve => setImmediate(resolve));
  assert.deepEqual(hooks.calls, ["D:/资料/agentor"]);
  assert.equal(hooks.values[4], true);
});

test("配置保存失败时保持应用可用并显示错误", async () => {
  hooks.reset();
  hooks.fail();
  button(render(), "确认切换并重启").props.onClick();
  await new Promise(resolve => setImmediate(resolve));
  assert.equal(hooks.values[4], false);
  assert.equal(hooks.values[3], "配置文件无法写入");
});
