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
    name: "chat-input-test",
    enforce: "pre",
    resolveId(id) { if (id === "virtual:chat-input-test") return "\0chat-input-test"; },
    load(id) {
      if (id === "\0chat-input-test") return `
        export let chat;
        export function setChat(value) { chat = value; }
        export function useChat() { return chat; }
      `;
    },
    transform(source, id) {
      if (id.endsWith("/features/chat/ChatPage.tsx")) {
        return source.replace('from "./useChat";', 'from "virtual:chat-input-test";');
      }
    },
  }],
  server: { middlewareMode: true, watch: null, ws: false },
  appType: "custom",
});
after(() => server.close());
const fixture = await server.ssrLoadModule("virtual:chat-input-test");
const { ChatPage } = await server.ssrLoadModule("/src/features/chat/ChatPage.tsx");

function find(element, predicate) {
  if (!element || typeof element !== "object") return;
  if (predicate(element)) return element;
  for (const child of [element.props?.children].flat(Infinity)) {
    const result = find(child, predicate);
    if (result) return result;
  }
}

// 只替换会话服务，直接取得实际输入框和表单的事件处理函数
function setup({ active = false, invalidImages = false } = {}) {
  const sent = [];
  const chat = {
    selected: "thread-1", threads: [], models: [], attachments: [],
    draftFiles: invalidImages ? [{ mediaType: "image/png" }] : [],
    conversation: { turns: [], activeTurnId: active ? "turn-1" : "" },
    draft: "", setDraft(value) { chat.draft = value; },
    send() { sent.push(chat.draft); chat.draft = ""; },
  };
  fixture.setChat(chat);
  let input, form;
  function Capture() {
    const tree = ChatPage({ settings: {}, status: { phase: "ready" }, sidebarContainer: null });
    input = find(tree, node => node.props?.id === "chat-input");
    form = find(tree, node => node.type === "form");
    return null;
  }
  function render() { renderToStaticMarkup(createElement(Capture)); }
  render();
  assert.ok(input, "应渲染可用的聊天输入框");
  let composing = false;
  return {
    chat, sent,
    change(value) { input.props.onChange({ target: { value } }); render(); },
    composition(type) {
      composing = type === "start";
      input.props[type === "start" ? "onCompositionStart" : "onCompositionEnd"]?.({});
    },
    key(options = {}) {
      const { keyCode = 13, isComposing = composing, ...keys } = options;
      const event = { key: "Enter", shiftKey: false, ...keys,
        nativeEvent: { keyCode, isComposing }, defaultPrevented: false,
        preventDefault() { this.defaultPrevented = true; },
      };
      input.props.onKeyDown(event);
      return event;
    },
    submit() { form.props.onSubmit({ preventDefault() {} }); },
  };
}

test("组合中的确认回车不发送、不阻止输入法默认行为，最终文本保留", () => {
  const view = setup();
  view.composition("start");
  view.change("eng");
  assert.equal(view.key().defaultPrevented, false);
  view.composition("end");
  view.change("english");
  assert.deepEqual(view.sent, []);
  assert.equal(view.chat.draft, "english");
  assert.equal(view.key().defaultPrevented, true);
  assert.deepEqual(view.sent, ["english"]);
});

test("WebKit 先 compositionend 后确认回车：isComposing=false、keyCode=229 不发送", () => {
  for (const active of [false, true]) {
    const view = setup({ active });
    view.composition("start");
    view.change("english");
    view.composition("end");
    assert.equal(view.key({ keyCode: 229 }).defaultPrevented, false);
    assert.deepEqual(view.sent, []);
    assert.equal(view.chat.draft, "english");
    // 下一次正常回车立即恢复发送，不使用时间窗口吞掉按键
    assert.equal(view.key().defaultPrevented, true);
    assert.deepEqual(view.sent, ["english"]);
  }
});

test("组合结束没有后续确认键时，下一次正常回车仍发送", () => {
  const view = setup();
  view.composition("start");
  view.change("english");
  view.composition("end");
  view.key();
  assert.deepEqual(view.sent, ["english"]);
});

test("正常回车发送，Shift+Enter 保留原生换行，其他按键不发送", () => {
  const view = setup();
  view.change("first");
  assert.equal(view.key({ shiftKey: true }).defaultPrevented, false);
  assert.equal(view.key({ key: "a", keyCode: 65 }).defaultPrevented, false);
  assert.deepEqual(view.sent, []);
  // 原生 textarea 换行随后触发 onChange
  view.change("first\nsecond");
  assert.equal(view.key().defaultPrevented, true);
  assert.deepEqual(view.sent, ["first\nsecond"]);
});

test("组合输入中的 Shift+Enter 同样交给输入法且保留文本", () => {
  const view = setup();
  view.composition("start");
  view.change("eng");
  assert.equal(view.key({ shiftKey: true }).defaultPrevented, false);
  assert.deepEqual(view.sent, []);
  assert.equal(view.chat.draft, "eng");
});

test("原有图片限制与发送按钮表单提交保持生效", () => {
  const blocked = setup({ invalidImages: true });
  blocked.change("english");
  assert.equal(blocked.key().defaultPrevented, true);
  blocked.submit();
  assert.deepEqual(blocked.sent, []);
  assert.equal(blocked.chat.draft, "english");
  const view = setup();
  view.change("english");
  view.submit();
  assert.deepEqual(view.sent, ["english"]);
});
