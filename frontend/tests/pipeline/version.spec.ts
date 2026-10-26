import { test, expect } from "@playwright/test";
import { join } from "node:path";
import { tmpdir } from "node:os";

test("仅绑定 worktree 的版本显示树状图标并提示完整目录", async ({ page }) => {
 await page.goto("/tests/pipeline/index.html?version");
 const icon = page.getByLabel("Worktree：/projects/worktrees/version-v1", {exact:true});
 await expect(icon).toHaveCount(1);
 await expect(page.locator('[aria-label^="Worktree："]')).toHaveCount(1);
 await expect(page.getByRole("button",{name:"V2 · 项目目录开发",exact:true})).toBeVisible();
 await icon.hover();
 await expect(page.getByRole("tooltip").filter({hasText:"/projects/worktrees/version-v1"})).toBeVisible();
 await page.screenshot({path:join(tmpdir(),"agentor-version-worktree.png")});
});
