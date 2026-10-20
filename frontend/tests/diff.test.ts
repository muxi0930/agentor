import assert from "node:assert/strict";
import test from "node:test";
import { parseDiff } from "../src/features/chat/diff.ts";

test("Codex add and delete items carry whole file contents rather than unified diff", () => {
  assert.deepEqual(parseDiff("CHECK\n", "add"), [{kind:"added",oldLine:null,newLine:1,text:"CHECK"}]);
  assert.deepEqual(parseDiff("-literal\n", "delete"), [{kind:"removed",oldLine:1,newLine:null,text:"-literal"}]);
});

test("diff line numbers follow hunks and preserve changed content resembling file headers", () => {
  const rows = parseDiff("--- a/file\n+++ b/file\n@@ -10,2 +20,2 @@\n--- old value\n+++ new value\n unchanged\n");
  assert.equal(rows.filter(row => row.kind === "header").length, 3);
  assert.deepEqual(rows[3], { kind: "removed", oldLine: 10, newLine: null, text: "-- old value" });
  assert.deepEqual(rows[4], { kind: "added", oldLine: null, newLine: 20, text: "++ new value" });
  assert.deepEqual(rows[5], { kind: "context", oldLine: 11, newLine: 21, text: " unchanged" });
});

test("a following file starts a new diff section", () => {
  const rows = parseDiff("@@ -1 +1 @@\n-old\n+new\ndiff --git a/next b/next\n--- a/next\n+++ b/next\n@@ -4 +7 @@\n+value");
  assert.equal(rows[4].kind, "header");
  assert.equal(rows[5].kind, "header");
  assert.deepEqual(rows[7], { kind: "added", oldLine: null, newLine: 7, text: "value" });
});
