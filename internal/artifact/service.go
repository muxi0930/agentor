package artifact

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"errors"
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
	"github.com/wailsapp/wails/v3/pkg/application"
)

type Attachment struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Status    string `json:"status"`
	Error     string `json:"error"`
	ThreadID  string `json:"threadId"`
	TurnID    string `json:"turnId"`
	MessageID string `json:"messageId"`
	Kind      string `json:"kind"`
	Source    string `json:"source"`
	LocalPath string `json:"localPath"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
}

type Service struct {
	mu    sync.Mutex
	root  string
	items map[string]Attachment
}

func NewService(dataDirectory string) (*Service, error) {
	s := &Service{root: filepath.Join(dataDirectory, "attachments"), items: make(map[string]Attachment)}
	if err := os.MkdirAll(s.root, 0700); err != nil {
		return nil, err
	}
	if err := appdata.ReadJSON(filepath.Join(s.root, "index.json"), &s.items); err != nil {
		return nil, err
	}
	changed := false
	for id, item := range s.items {
		if item.Status == "pending" {
			item.Status, item.Error = "failed", "上次文件导入被中断"
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

func (s *Service) persist() error {
	return appdata.WriteJSON(filepath.Join(s.root, "index.json"), s.items)
}

func (s *Service) save(threadID, name, source, mediaType string, input io.Reader) (Attachment, error) {
	if threadID == "" {
		return Attachment{}, errors.New("请先打开一个会话")
	}
	name = filepath.Base(name)
	if name == "" || name == "." {
		return Attachment{}, errors.New("文件名不能为空")
	}
	id := rand.Text()
	directory := filepath.Join(s.root, id)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return Attachment{}, err
	}
	item := Attachment{ID: id, Name: name, MediaType: mediaType, Status: "pending", ThreadID: threadID, Kind: "input", Source: source,
		LocalPath: filepath.Join(directory, name), URL: "/agentor/attachments/" + id, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
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
		item.Status, item.Error = "failed", err.Error()
	} else {
		item.Status = "ready"
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
		return Attachment{}, errors.New("请选择普通文件")
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

func (s *Service) SavePastedImage(threadID, name, encoded string) (Attachment, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return Attachment{}, errors.New("图片数据无法读取")
	}
	mediaType := http.DetectContentType(data)
	if !strings.HasPrefix(mediaType, "image/") {
		return Attachment{}, errors.New("剪贴板内容不是支持的图片")
	}
	return s.save(threadID, name, "clipboard", mediaType, bytes.NewReader(data))
}

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

//wails:ignore
func (s *Service) ResolveInputs(threadID string, ids []string) ([]Attachment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]Attachment, 0, len(ids))
	for _, id := range ids {
		item, found := s.items[id]
		if !found || item.ThreadID != threadID {
			return nil, errors.New("附件不属于当前会话")
		}
		if item.Status != "ready" {
			return nil, fmt.Errorf("附件 %s 尚未准备好: %s", item.Name, item.Error)
		}
		items = append(items, item)
	}
	return items, nil
}

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
			return errors.New("附件不属于当前会话")
		}
		item.TurnID, item.MessageID = turnID, messageID
		s.items[id] = item
	}
	return s.persist()
}

func (s *Service) OpenAttachment(id string) error {
	s.mu.Lock()
	item, found := s.items[id]
	s.mu.Unlock()
	if !found || item.Status != "ready" {
		return errors.New("附件不可用")
	}
	return application.Get().Browser.OpenFile(item.LocalPath)
}

func (s *Service) LocateAttachment(id string) error {
	s.mu.Lock()
	item, found := s.items[id]
	s.mu.Unlock()
	if !found {
		return errors.New("附件不存在")
	}
	return application.Get().Env.OpenFileManager(item.LocalPath, true)
}

//wails:ignore
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		const prefix = "/agentor/attachments/"
		if !strings.HasPrefix(request.URL.Path, prefix) {
			next.ServeHTTP(w, request)
			return
		}
		id := strings.TrimPrefix(request.URL.Path, prefix)
		s.mu.Lock()
		item, found := s.items[id]
		s.mu.Unlock()
		if !found || item.Status != "ready" {
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
