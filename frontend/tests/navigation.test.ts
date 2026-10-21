import assert from "node:assert/strict";
import { test } from "node:test";
import { move, visit, type NavigationHistory } from "../src/app/navigation.ts";

test("页面导航支持返回、前进和历史边界", () => {
  let history: NavigationHistory = { entries: [], index: -1 };
  history = visit(history, { page: "home", mode: "chat" });
  history = visit(history, { page: "home", mode: "workspace" });
  history = visit(history, { page: "settings", mode: "workspace" });
  assert.equal(move(history, 1), history);
  history = move(history, -1);
  assert.deepEqual(history.entries[history.index], { page: "home", mode: "workspace" });
  assert.equal(move(history, 1).entries[2].page, "settings");
  history = move(history, -1);
  assert.deepEqual(history.entries[history.index], { page: "home", mode: "chat" });
  assert.equal(move(history, -1), history);
});

test("重复导航不入栈，返回后新导航截断前进历史", () => {
  let history: NavigationHistory = { entries: [], index: -1 };
  history = visit(history, { page: "home", mode: "chat" });
  assert.equal(visit(history, { page: "home", mode: "chat" }), history);
  history = visit(history, { page: "settings", mode: "chat" });
  history = visit(move(history, -1), { page: "controller", mode: "chat" });
  assert.deepEqual(history.entries.map(entry => entry.page), ["home", "controller"]);
  assert.equal(move(history, 1), history);
});
