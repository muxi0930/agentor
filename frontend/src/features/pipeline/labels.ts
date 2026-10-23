export const actionNames: Record<string, string> = {
  "workspace.snapshot": "读取项目概况",
  "step.run": "执行开发步骤",
  "knowledge.read": "读取知识文档",
  "report.write": "生成运行报告",
  "worktree.create": "创建独立工作区",
  "worktree.merge": "合并到来源分支",
  "worktree.remove": "清理工作区与分支",
};
export const parameterHelp: Record<string, { label: string; help: string; placeholder: string }> = {
  versionId: { label: "版本", help: "选择工作区版本；留空读取项目下所有版本。", placeholder: "可选：版本 ID" },
  featureId: { label: "功能", help: "选择项目中的功能，限定读取范围；留空使用项目范围。", placeholder: "可选：功能 ID" },
  stepId: { label: "开发步骤", help: "选择要执行的步骤，按版本 / 功能 / 步骤展示。", placeholder: "必填：步骤 ID" },
  instruction: { label: "执行要求", help: "告诉执行助手本次需要完成什么工作。", placeholder: "例如：实现登录校验，并运行相关测试" },
  completionCriteria: { label: "验收标准", help: "说明如何判断完成；执行助手还须结构化汇报结果。", placeholder: "例如：新增测试通过，变更已提交" },
  documentId: { label: "知识文档", help: "填写已登记的知识文档 ID。", placeholder: "必填：文档 ID" },
  fileName: { label: "报告文件名", help: "使用 .md 文件名，不包含目录。", placeholder: "例如：result.md" },
  content: { label: "报告内容", help: "填写正文，或引用前面节点的完整输出结果。", placeholder: "报告正文" },
};
export const typeNames: Record<string, string> = { string: "文本", number: "数字", boolean: "是 / 否", any: "文本或节点结果" };
export const runLabels: Record<string, string> = { queued: "等待执行", running: "执行中", paused: "已暂停", completed: "成功", error: "失败", interrupted: "已中断", cancelled: "已取消", pending: "尚未执行" };
export function actionName(type: string) { return actionNames[type] ? `${type} · ${actionNames[type]}` : type; }
