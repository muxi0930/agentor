// 验收专用入口：复用真实业务服务，不启动桌面或飞书连接
package main

import (
 "encoding/json"
 "fmt"
 "net/http"
 "os"
 "agentor/internal/appdata"
 "agentor/internal/artifact"
 "agentor/internal/codex"
 cm "agentor/internal/codex/model"
 "agentor/internal/pipeline"
 pm "agentor/internal/pipeline/model"
 "agentor/internal/tasks"
 "agentor/internal/toolrouter"
 "agentor/internal/workspace"
 wm "agentor/internal/workspace/model"
)

func main() {
 data,err:=appdata.NewService("acceptance-20261023"); must(err)
 root:=data.GetInfo().DataDirectory
 files,err:=artifact.NewService(root); must(err)
 work,err:=workspace.NewService(root); must(err)
 runtime:=codex.NewService(data,files)
 execs,err:=tasks.NewService(data,runtime,work,files); must(err)
 pipes,err:=pipeline.NewService(data,work,execs,files,runtime); must(err)
 router:=toolrouter.New(execs,work,pipes,nil)
 execs.ConfigureTools(router.Specs); pipes.ConfigureTools(router.Specs)
 runtime.SetToolHandler(router.Call); runtime.SetEventHandler(func(e cm.Event) { execs.OnCodexEvent(e); if e.Method=="error" || e.Method=="turn/completed" {fmt.Println("EVENT",e.Method,e.Params)} })
 defer runtime.ServiceShutdown(); defer execs.ServiceShutdown(); defer pipes.ServiceShutdown()
 _,err=runtime.Connect(); must(err)
 server:=&http.Server{Addr:"127.0.0.1:9278"}
 http.HandleFunc("/",func(w http.ResponseWriter,r *http.Request){
  var req struct {Op string; ProjectID string; VersionID string; FeatureID string; Raw json.RawMessage; RunID string; Operation string; Directory string}
  err:=json.NewDecoder(r.Body).Decode(&req)
  var out any
  if err==nil { switch req.Op {
  case "status": out=runtime.GetStatus()
  case "limits": out,err=runtime.GetRateLimits()
  case "knowledge":
   var v struct{Title string; Content string};err=json.Unmarshal(req.Raw,&v);if err==nil{var d wm.Document;d,err=work.PrepareKnowledge(req.ProjectID,"",v.Title);if err==nil{out,err=work.SyncKnowledge(req.ProjectID,"",d.ID,v.Content)}}
  case "state": out=pipes.GetState()
  case "workspace": out=work.GetState()
  case "executions": out=execs.GetState()
  case "actions": out=pipes.GetActions()
  case "approvals": out=runtime.GetApprovals()
  case "template": out,err=pipes.SaveTemplate(string(req.Raw))
  case "task": out,err=pipes.SaveTask(req.ProjectID,string(req.Raw))
  case "validate": out,err=pipes.ValidateTask(req.ProjectID,string(req.Raw))
  case "instantiate":
   var v pm.InstantiateInput; err=json.Unmarshal(req.Raw,&v); if err==nil {out,err=pipes.Instantiate(v)}
  case "version":
   var v wm.Version; err=json.Unmarshal(req.Raw,&v); if err==nil {out,err=work.CreateVersion(req.ProjectID,v)}
  case "feature":
   var v wm.Feature; err=json.Unmarshal(req.Raw,&v); if err==nil {out,err=work.CreateFeature(req.ProjectID,req.VersionID,v)}
  case "step":
   var v wm.Step; err=json.Unmarshal(req.Raw,&v); if err==nil {out,err=work.CreateStep(req.ProjectID,req.FeatureID,v)}
  case "run":
   var v pm.RunInput; err=json.Unmarshal(req.Raw,&v)
   if err==nil && req.Directory!="" {
    // 仅在冻结运行目录期间使用隔离验收仓库，随后立即恢复项目配置
    for _,p:=range work.GetState().Projects {if p.ID==req.ProjectID {original:=p; p.Directory=req.Directory; _,err=work.SaveProject(p); if err==nil {out,err=pipes.RunTask(v)}; _,restoreErr:=work.SaveProject(original); if restoreErr!=nil {err=restoreErr}; break}}
   } else if err==nil {out,err=pipes.RunTask(v)}
  case "control": out,err=pipes.ControlRun(req.ProjectID,req.RunID,req.Operation)
  case "quit": out="closing"; go server.Close()
  default: err=fmt.Errorf("unknown op %s",req.Op)
  }}
  w.Header().Set("Content-Type","application/json")
  if err!=nil {json.NewEncoder(w).Encode(map[string]any{"error":err.Error()})} else {json.NewEncoder(w).Encode(map[string]any{"data":out})}
 })
 fmt.Println("READY",root)
 if err:=server.ListenAndServe(); err!=nil && err!=http.ErrServerClosed {must(err)}
}
func must(err error){if err!=nil {fmt.Fprintln(os.Stderr,err);os.Exit(1)}}
