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
  server: { middlewareMode: true, watch: null, ws: false },
  appType: "custom",
});
after(() => server.close());
const { MessageView } = await server.ssrLoadModule("/src/features/chat/MessageView.tsx");
const { applyNotification, mergeTurn } = await server.ssrLoadModule("/src/features/chat/state.ts");
const render = (item, streaming = false) => renderToStaticMarkup(createElement(MessageView, { item, streaming, onError: assert.fail }));

test("历史和实时会话均不渲染空思考区块", () => {
  for (const status of ["completed", "inProgress"]) {
    for (const summary of [[], ["", " \n\t"]]) {
      for (const streaming of [false, true]) {
        assert.equal(render({ id: "reason-1", type: "reasoning", status, summary, reasoningContent: [] }, streaming), "");
      }
    }
  }
});

test("历史摘要和上游提供的内容保留原有折叠入口", () => {
  for (const fields of [{ summary: ["检查调用链"] }, { summary: [" "], reasoningContent: ["已提供的内容"] }]) {
    const html = render({ id: "reason-1", type: "reasoning", status: "completed", ...fields });
    assert.match(html, /思考过程/);
    assert.match(html, /lucide-brain/);
    assert.match(html, /aria-expanded="false"/);
    assert.doesNotMatch(html, /未提供思考摘要|正在思考/);
  }
});

test("空记录后续收到摘要时显示，完成与历史合并不抹除摘要", () => {
  const empty = { id: "reason-1", type: "reasoning", summary: [], reasoningContent: [] };
  let state = { turns: [], activeTurnId: "turn-1" };
  state = applyNotification(state, "item/started", { turnId: "turn-1", item: empty });
  assert.equal(render(state.turns[0].items[0], true), "");
  state = applyNotification(state, "item/reasoning/summaryTextDelta", { turnId: "turn-1", itemId: "reason-1", summaryIndex: 0, delta: "检查调用链" });
  assert.match(render(state.turns[0].items[0], true), /思考过程/);
  assert.match(render(state.turns[0].items[0], true), /data-status="running"/);
  state = applyNotification(state, "item/completed", { turnId: "turn-1", item: empty });
  const history = mergeTurn(state.turns, { id: "turn-1", status: "completed", items: [empty] });
  assert.deepEqual(history[0].items[0].summary, ["检查调用链"]);
  assert.match(render(history[0].items[0]), /思考过程/);
});

test("无思考摘要的命令仍正常显示", () => {
  assert.match(render({ id: "command-1", type: "commandExecution", command: "pwd", status: "completed", summary: [] }), /pwd/);
});
