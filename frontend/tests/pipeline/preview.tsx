import React, { useState } from "react";
import { createRoot } from "react-dom/client";
import "../../src/index.css";
import "../../src/features/pipeline/pipeline.css";
import { PipelineCanvas } from "../../src/features/pipeline/PipelineCanvas";
import { newNode, newTask, type Graph } from "../../src/features/pipeline/graph";
import type { ActionSpec } from "../../bindings/agentor/internal/pipeline/model/models";
import { RunInputs } from "../../src/features/pipeline/RunInputs";
const specs: ActionSpec[] = [{ name: "report.write", description: "生成运行报告", parameters: [{ name: "fileName", type: "string", required: true }, { name: "content", type: "any", required: true }], result: "报告", requiresProject: false, branches: [{ name: "success", description: "完成", target: "" }] }];
function Preview() {
 const [task, setTask] = useState<Graph>(() => { const stored = localStorage.getItem("pipeline-preview"); if (stored) return JSON.parse(stored); const task = newTask(""); task.name = "图交互测试"; const agent = newNode("agent.run", specs, { x: 40, y: 50 }); agent.id = "agent"; agent.prompt = "完成检查"; agent.criteria = "验证通过"; const end = newNode("end.success", specs, { x: 420, y: 50 }); end.id = "done"; task.entry = agent.id; task.actions = [agent, end]; return task; });
 return <main className="flex h-screen min-h-0 flex-col gap-3 p-4"><header className="flex gap-4 text-sm"><h1>流水线节点画布</h1><button onClick={() => localStorage.setItem("pipeline-preview", JSON.stringify(task))}>保存布局</button><button onClick={() => document.documentElement.classList.toggle("dark")}>切换主题</button></header><PipelineCanvas task={task} specs={specs} template={location.search === "?template"} onChange={setTask} /><output data-testid="graph" className="hidden">{JSON.stringify(task)}</output></main>;
}
function InputPreview() {
 const [values, setValues] = useState<Record<string, unknown>>({}), [valid, setValid] = useState(true);
 const task = { ...newTask(""), inputs: [{ name: "payload", type: "any", required: true }] };
 return <main className="p-4"><RunInputs task={task} values={values} onChange={setValues} onValidityChange={setValid} /><button disabled={!valid}>运行流水线</button><output data-testid="input-value">{JSON.stringify(values)}</output></main>;
}
createRoot(document.getElementById("root")!).render(location.search === "?inputs" ? <InputPreview /> : <Preview />);
