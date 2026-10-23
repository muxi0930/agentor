package artifact

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"agentor/internal/appdata"
	"agentor/internal/constant"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// API 按职责列出服务的全部公开方法
type API interface {
	ImportFile(threadID, path string) (Attachment, error)                                     // 复制本地文件至应用附件目录
	ChooseFiles(threadID string) ([]Attachment, error)                                        // 通过原生对话框选择会话输入文件
	SavePastedImage(threadID, name, encoded string) (Attachment, error)                       // 解码并保存 WebView 粘贴的图片
	GetAttachments(threadID string) []Attachment                                              // 按创建顺序读取当前会话附件
	ResolveInputs(threadID string, ids []string) ([]Attachment, error)                        // 核验输入附件属于当前会话且已经可用
	BindTurn(threadID, turnID, messageID string, ids []string) error                          // 将附件绑定到实际发送的用户消息
	OpenAttachment(id string) error                                                           // 通过系统应用打开附件
	LocateAttachment(id string) error                                                         // 在文件管理器定位附件
	Middleware(next http.Handler) http.Handler                                                // 向 WebView 提供已登记的附件资源
	DiscardDraftAttachment(id string) error                                                   // 只清理尚未发送的应用托管附件副本
	CreateHTMLPreview(threadID, source string) (string, error)                                // 创建随消息视图挂载的临时预览，源码仍保留在 Codex 历史中
	ReleaseHTMLPreview(url string)                                                            // 释放消息视图不再使用的临时预览
	RegisterProduced(owner ProducedOwner, path, directory string) (Attachment, error)         // 将项目目录内的文件登记为指定任务的产物
	GetExecutionArtifacts(executionID string) []Attachment                                    // 按创建顺序读取指定步骤执行的产物
	SavePipelineReport(runID, projectID, requestID, name, content string) (Attachment, error) // 将 Markdown 报告保存为指定流水线运行的产物
	GetPipelineArtifacts(runID string) []Attachment                                           // 读取指定流水线运行的产物
	SaveRequestInput(requestID, name, mediaType string, input io.Reader) (Attachment, error)  // 保存远程请求的独立输入附件
	RecordFailedInput(requestID, name, reason string) error                                   // 保留未下载成功的附件及原因
	GetRequestAttachments(requestID string) []Attachment                                      // 按接收顺序读取请求原始附件
	CopyRequestInputs(requestID, threadID, executionID, stepID string) ([]string, error)      // 为每次执行复制输入以保留独立归属
	GetAttachment(id string) (Attachment, error)                                              // 按稳定标识读取附件记录
}

var _ API = (*Service)(nil)

type Attachment struct {
	PipelineRunID string `json:"pipelineRunId"` // 所属流水线运行
	ID            string `json:"id"`            // 附件稳定标识
	Name          string `json:"name"`          // 文件显示名称
	MediaType     string `json:"mediaType"`     // 文件媒体类型
	Size          int64  `json:"size"`          // 文件字节数
	Status        string `json:"status"`        // 导入状态
	Error         string `json:"error"`         // 导入失败原因
	ThreadID      string `json:"threadId"`      // 所属会话
	TurnID        string `json:"turnId"`        // 所属对话轮次
	MessageID     string `json:"messageId"`     // 所属消息
	Kind          string `json:"kind"`          // 输入附件或任务产物
	Source        string `json:"source"`        // 文件来源
	LocalPath     string `json:"localPath"`     // 应用托管副本路径
	URL           string `json:"url"`           // WebView 资源地址
	CreatedAt     string `json:"createdAt"`     // 创建时间
	ExecutionID   string `json:"executionId"`   // 所属步骤执行
	StepID        string `json:"stepId"`        // 所属工作区步骤
	ProjectID     string `json:"projectId"`     // 所属项目
	RequestID     string `json:"requestId"`     // 所属远程请求
}

type Service struct {
	mu       sync.Mutex            // 附件索引与预览读写锁
	root     string                // 应用附件目录
	items    map[string]Attachment // 已登记的附件
	previews map[string]string     // 当前挂载的 HTML 预览源码
}

// NewService 初始化服务并读取本地记录
func NewService(dataDirectory string) (*Service, error) {
	s := &Service{root: filepath.Join(dataDirectory, constant.ArtifactDirectoryName), items: make(map[string]Attachment), previews: make(map[string]string)}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return nil, err
	}
	if err := appdata.ReadJSON(filepath.Join(s.root, constant.ArtifactIndexFilename), &s.items); err != nil {
		return nil, err
	}

	changed := false
	for id, item := range s.items {
		if item.Status == constant.ArtifactStatusPending {
			item.Status, item.Error = constant.ArtifactStatusFailed, constant.ArtifactImportInterruptedMessage
			s.items[id] = item
			changed = true
		}
	}
	if changed {
		if err := s.persist(); err != nil {
			return nil, err
		}
	}
	return s, nil
}

// ImportFile 复制本地文件至应用附件目录
//
//wails:ignore
func (s *Service) ImportFile(threadID, path string) (Attachment, error) {
	file, err := os.Open(path)
	if err != nil {
		return Attachment{}, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return Attachment{}, err
	}
	if !info.Mode().IsRegular() {
		return Attachment{}, constant.ErrArtifactRegularFileRequired
	}

	header := make([]byte, 512)
	count, err := file.Read(header)
	if err != nil && err != io.EOF {
		return Attachment{}, err
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return Attachment{}, err
	}
	return s.save(threadID, filepath.Base(path), "local", http.DetectContentType(header[:count]), file)
}

// ChooseFiles 通过原生对话框选择会话输入文件
func (s *Service) ChooseFiles(threadID string) ([]Attachment, error) {
	app := application.Get()
	paths, err := app.Dialog.OpenFile().AttachToWindow(app.Window.Current()).CanChooseFiles(true).CanChooseDirectories(false).PromptForMultipleSelection()
	if err != nil {
		return nil, err
	}

	items := make([]Attachment, 0, len(paths))
	for _, path := range paths {
		item, err := s.ImportFile(threadID, path)
		if err != nil {
			return items, err
		}
		items = append(items, item)
	}
	return items, nil
}

// SavePastedImage 解码并保存 WebView 粘贴的图片
func (s *Service) SavePastedImage(threadID, name, encoded string) (Attachment, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Attachment{}, constant.ErrArtifactImageDataInvalid
	}

	mediaType := http.DetectContentType(data)
	if !strings.HasPrefix(mediaType, "image/") {
		return Attachment{}, constant.ErrArtifactImageTypeUnsupported
	}
	return s.save(threadID, name, "clipboard", mediaType, bytes.NewReader(data))
}

// GetAttachments 按创建顺序读取当前会话附件
func (s *Service) GetAttachments(threadID string) []Attachment {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := []Attachment{}
	for _, item := range s.items {
		if item.ThreadID == threadID {
			items = append(items, item)
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt < items[j].CreatedAt })
	return items
}

// ResolveInputs 核验输入附件属于当前会话且已经可用
//
//wails:ignore
func (s *Service) ResolveInputs(threadID string, ids []string) ([]Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		item, found := s.items[id]
		if !found || item.ThreadID != threadID {
			return nil, constant.ErrArtifactAttachmentOutsideThread
		}
		if item.Status != constant.ArtifactStatusReady {
			return nil, fmt.Errorf("附件 %s 尚未准备好: %s", item.Name, item.Error)
		}
		items = append(items, item)
	}
	return items, nil
}

// BindTurn 将附件绑定到实际发送的用户消息
//
//wails:ignore
func (s *Service) BindTurn(threadID, turnID, messageID string, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, id := range ids {
		item, found := s.items[id]
		if !found || item.ThreadID != threadID {
			return constant.ErrArtifactAttachmentOutsideThread
		}
		item.TurnID, item.MessageID = turnID, messageID
		s.items[id] = item
	}
	return s.persist()
}

// OpenAttachment 通过系统应用打开附件
func (s *Service) OpenAttachment(id string) error {
	s.mu.Lock()
	item, found := s.items[id]
	s.mu.Unlock()
	if !found || item.Status != constant.ArtifactStatusReady {
		return constant.ErrArtifactAttachmentUnavailable
	}
	return application.Get().Browser.OpenFile(item.LocalPath)
}

// LocateAttachment 在文件管理器定位附件
func (s *Service) LocateAttachment(id string) error {
	s.mu.Lock()
	item, found := s.items[id]
	s.mu.Unlock()
	if !found {
		return constant.ErrArtifactAttachmentNotFound
	}
	return application.Get().Env.OpenFileManager(item.LocalPath, true)
}

// Middleware 向 WebView 提供已登记的附件资源
//
//wails:ignore
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if strings.HasPrefix(request.URL.Path, "/agentor/previews/") {
			s.servePreview(w, request)
			return
		}

		const prefix = "/agentor/attachments/"
		if !strings.HasPrefix(request.URL.Path, prefix) {
			next.ServeHTTP(w, request)
			return
		}

		id := strings.TrimPrefix(request.URL.Path, prefix)
		s.mu.Lock()
		item, found := s.items[id]
		s.mu.Unlock()
		if !found || item.Status != constant.ArtifactStatusReady {
			http.NotFound(w, request)
			return
		}

		file, err := os.Open(item.LocalPath)
		if err != nil {
			http.NotFound(w, request)
			return
		}
		defer file.Close()

		info, err := file.Stat()
		if err != nil {
			http.NotFound(w, request)
			return
		}

		w.Header().Set("Content-Type", item.MediaType)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if !strings.HasPrefix(item.MediaType, "image/") {
			w.Header().Set("Content-Disposition", "attachment")
		}
		http.ServeContent(w, request, item.Name, info.ModTime(), file)
	})
}

// persist 保存附件索引
func (s *Service) persist() error {
	return appdata.WriteJSON(filepath.Join(s.root, constant.ArtifactIndexFilename), s.items)
}

// save 保存会话输入附件
func (s *Service) save(threadID, name, source, mediaType string, input io.Reader) (Attachment, error) {
	return s.saveOwned(Attachment{ThreadID: threadID, Source: source, Kind: "input"}, name, mediaType, input)
}

// saveOwned 保存已有业务归属的托管附件
func (s *Service) saveOwned(owner Attachment, name, mediaType string, input io.Reader) (Attachment, error) {
	if owner.ThreadID == "" && owner.RequestID == "" {
		return Attachment{}, constant.ErrArtifactAttachmentOwnerRequired
	}

	name = filepath.Base(name)
	if name == "" || name == "." {
		return Attachment{}, constant.ErrArtifactFilenameRequired
	}

	id := rand.Text()
	directory := filepath.Join(s.root, id)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return Attachment{}, err
	}

	item := owner
	item.ID, item.Name, item.MediaType, item.Status = id, name, mediaType, constant.ArtifactStatusPending
	item.LocalPath, item.URL, item.CreatedAt = filepath.Join(directory, name), "/agentor/attachments/"+id, time.Now().UTC().Format(time.RFC3339Nano)
	s.mu.Lock()
	s.items[id] = item
	err := s.persist()
	s.mu.Unlock()
	if err != nil {
		return item, err
	}

	file, err := os.OpenFile(item.LocalPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err == nil {
		item.Size, err = io.Copy(file, input)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
	}

	if err != nil {
		item.Status, item.Error = constant.ArtifactStatusFailed, err.Error()
	} else {
		item.Status = constant.ArtifactStatusReady
	}
	s.mu.Lock()
	s.items[id] = item
	saveErr := s.persist()
	s.mu.Unlock()
	if err != nil {
		return item, err
	}
	return item, saveErr
}
