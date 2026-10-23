package constant

// artifact 模块的固定配置与提示文案
const (
	ArtifactPreviewBridge = `<!doctype html><meta charset="utf-8"><script>
(() => {
  const send = value => parent.postMessage({channel:"agentor-preview", ...value}, "*");
  window.agentor = {followup: text => { if (typeof text === "string") send({type:"followup", text}); }};
  addEventListener("DOMContentLoaded", () => {
    const resize = () => send({type:"resize", height:document.documentElement.scrollHeight});
    new ResizeObserver(resize).observe(document.body); resize();
  });
  addEventListener("error", event => send({type:"error", text:event.message}));
})();
</script>`
	ArtifactImportInterruptedMessage = "上次文件导入被中断"
)

// artifact 模块的业务标识
const (
	ArtifactStatusPending = "pending"
	ArtifactStatusReady   = "ready"
	ArtifactStatusFailed  = "failed"
)

// artifact 模块的状态与协议标识
const (
	ArtifactIndexFilename = "index.json"
	ArtifactDirectoryName = "attachments"
)
