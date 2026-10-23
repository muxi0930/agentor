import { test, expect } from "@playwright/test";

test("已连出口拖至空白解除连线，目标端点显示吸附圈", async ({page}) => {
 await page.goto("/tests/pipeline/index.html");
 const source=page.locator('[data-id="agent"] [data-handleid="success"]'), target=page.locator('[data-id="done"] .target');
 const a=await source.boundingBox(), b=await target.boundingBox(); if(!a||!b)throw Error("缺少端点");
 await page.mouse.move(a.x+a.width/2,a.y+a.height/2);await page.mouse.down();await page.mouse.move(b.x+b.width/2,b.y+b.height/2,{steps:15});
 await expect(target).toHaveClass(/connectingto/); await expect(target).toHaveClass(/valid/);
 expect(await target.evaluate(el=>getComputedStyle(el,"::after").borderWidth)).toBe("2px");
 await page.mouse.up();
 await expect(page.locator(".react-flow__edge")).toHaveCount(1);
 expect(await page.locator(".react-flow__edge-path").getAttribute("d")).toContain("C");
 await expect(page.locator(".react-flow__edge-textbg")).toHaveCount(0);
 const next=await source.boundingBox();if(!next)throw Error("缺少出口");
 await page.mouse.move(next.x+5,next.y+5);await page.mouse.down();await page.mouse.move(700,550,{steps:15});await page.mouse.up();
 await expect(page.locator(".react-flow__edge")).toHaveCount(0);
});

test("Agent 参数表单保留重命名及运行输入引用",async({page})=>{
 await page.addInitScript(()=>{
  localStorage.setItem("pipeline-preview",JSON.stringify({id:"",projectId:"",templateId:"",directory:"",name:"表单验收",entry:"agent",inputs:[{name:"payload",type:"string",required:true}],actions:[{id:"agent",type:"agent.run",kind:"agent",name:"",params:{existing:{input:"payload"}},position:{x:40,y:50},branches:[{name:"success",description:"完成",target:""}],prompt:"验收",criteria:"验收",permission:"workspace-write",maxRetries:0,timeoutSeconds:900,failureTarget:"",stepSlot:"",stepId:""}]}));
 });
 await page.goto("/tests/pipeline/index.html");
 await page.locator('[data-id="agent"]').click();
 await expect(page.getByLabel("输入数据（JSON，支持引用）")).toHaveCount(0);
 await expect(page.getByLabel("参数值引用")).toContainText("payload");
 await page.getByLabel("参数 existing 名称").fill("renamed");
 await page.getByLabel("参数 existing 名称").press("Tab");
 await expect(page.getByLabel("参数 renamed 名称")).toHaveValue("renamed");
 const graph=JSON.parse(await page.getByTestId("graph").textContent()||"{}");
 expect(graph.actions[0].params).toEqual({renamed:{input:"payload"}});
 await page.getByRole("button",{name:"添加参数",exact:true}).click();
 await expect(page.getByLabel("参数 param_1 名称")).toBeVisible();
 await page.getByRole("button",{name:"移除参数 param_1"}).click();
 await expect(page.getByLabel("参数 param_1 名称")).toHaveCount(0);
 await page.screenshot({path:"../docs/testing/2026-10-23-pipeline/screenshots/agent-input-form.png"});
});

test("冲突节点自动填充模板，缩略图遮罩半透明",async({page})=>{
 await page.goto("/tests/pipeline/index.html");
 await page.locator(".pipeline-palette-item").filter({hasText:"agent.resolve-conflicts"}).click();
 const panel=page.getByRole("dialog");
 await expect(panel.getByLabel("执行要求",{exact:true})).toContainText("合并提交");
 await expect(panel.getByLabel("验收标准",{exact:true})).toContainText("无未合并文件");
 const graph=JSON.parse(await page.getByTestId("graph").textContent()||"{}");
 expect(graph.actions.at(-1).type).toBe("agent.resolve-conflicts");
 const mask=page.locator(".react-flow__minimap-mask");
 const fill=await mask.evaluate(el=>getComputedStyle(el).fill);
 expect(fill).toMatch(/0\.55|55%/);
 await page.screenshot({path:"../docs/testing/2026-10-23-pipeline/screenshots/conflict-preset.png"});
});

test("模板步骤绑定开关自动建立单步骤绑定",async({page})=>{
 await page.goto("/tests/pipeline/index.html?template");
 await page.locator('[data-id="agent"]').click();
 await expect(page.getByLabel("项目步骤插槽（可选）")).toHaveCount(0);
 const toggle=page.getByRole("switch",{name:"需要绑定项目步骤"});
 await expect(toggle).not.toBeChecked();
 await toggle.click();await expect(toggle).toBeChecked();
 let graph=JSON.parse(await page.getByTestId("graph").textContent()||"{}");
 expect(graph.actions[0].stepSlot).toBe("agent");
 await toggle.click();
 graph=JSON.parse(await page.getByTestId("graph").textContent()||"{}");
 expect(graph.actions[0].stepSlot).toBe("");
});
