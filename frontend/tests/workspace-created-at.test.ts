import assert from "node:assert/strict";
import test from "node:test";
import { formatCreatedAt } from "../src/features/workspace/createdAt.ts";

test("三个层级的创建时间按本地时间展示，历史缺失时间保持未知", () => {
  const value = "2026-10-21T03:04:05.123456789Z";
  const date = new Date(value);
  const pad = (part: number) => String(part).padStart(2, "0");
  const expected = `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`;
  for (const node of [{ createdAt: value }, { createdAt: value }, { createdAt: value }]) {
    assert.equal(formatCreatedAt(node.createdAt), expected);
  }
  assert.equal(formatCreatedAt(undefined), "创建时间未知");
  assert.equal(formatCreatedAt(""), "创建时间未知");
});
