package codex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strconv"
	"sync"
	"time"

	"agentor/internal/constant"
)

type packet struct {
	ID     json.RawMessage `json:"id,omitempty"`     // 协议请求标识
	Method string          `json:"method,omitempty"` // 协议请求方法
	Params json.RawMessage `json:"params,omitempty"` // 原始请求参数
	Result json.RawMessage `json:"result,omitempty"` // 原始响应结果
	Error  *rpcError       `json:"error,omitempty"`  // 失败原因
}

type rpcError struct {
	Code    int    `json:"code"`    // 协议错误码
	Message string `json:"message"` // 可读提示信息
}

type client struct {
	cmd       *exec.Cmd              // 原生子进程
	stdin     io.WriteCloser         // 原生进程输入管道
	writeMu   sync.Mutex             // 串行保护协议写入
	mu        sync.Mutex             // 保护连接状态
	nextID    uint64                 // 下一个请求序号
	pending   map[string]chan packet // 等待响应的请求
	done      chan struct{}          // 连接结束通知
	once      sync.Once              // 保证仅记录一次退出
	err       error                  // 连接结束原因
	onMessage func(packet)           // 服务器消息处理入口
	onExit    func(error)            // 进程退出回调
}

// startClient 启动原生进程并监听协议消息和退出状态
func startClient(path, home string, env []string, onMessage func(packet), onExit func(error)) (*client, error) {
	cmd := exec.Command(path, "app-server")
	hideProcessWindow(cmd)
	cmd.Env = env
	cmd.Dir = home
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	// 原生错误输出可能包含请求数据，不将其写入应用日志
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, err
	}

	c := &client{cmd: cmd, stdin: stdin, pending: make(map[string]chan packet), done: make(chan struct{}), onMessage: onMessage, onExit: onExit}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)

		decoder := json.NewDecoder(stdout)
		for {
			var message packet
			if err := decoder.Decode(&message); err != nil {
				if !errors.Is(err, io.EOF) {
					c.finish(fmt.Errorf("Codex 协议读取失败: %w", err))
					cmd.Process.Kill()
				}
				return
			}

			c.receive(message)
		}
	}()
	go func() {
		<-readDone
		err := cmd.Wait()
		if err == nil {
			err = constant.ErrCodexServerClosed
		}
		c.finish(err)
	}()
	return c, nil
}

// receive 分发服务器通知并按标识匹配请求响应
func (c *client) receive(message packet) {
	if message.Method != "" {
		c.onMessage(message)
		return
	}

	c.mu.Lock()
	response := c.pending[string(message.ID)]
	delete(c.pending, string(message.ID))
	c.mu.Unlock()
	if response != nil {
		response <- message
	}
}

// finish 记录连接结束状态并通知退出观察者
func (c *client) finish(err error) {
	c.once.Do(func() {
		c.mu.Lock()
		c.err = err
		close(c.done)
		c.mu.Unlock()
		if c.onExit != nil {
			c.onExit(err)
		}
	})
}

// write 串行写入一条 JSON 协议消息
func (c *client) write(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return json.NewEncoder(c.stdin).Encode(value)
}

// request 发送请求并等待响应或上下文结束
func (c *client) request(ctx context.Context, method string, params any, target any) error {
	c.mu.Lock()
	select {
	case <-c.done:
		err := c.err
		c.mu.Unlock()
		return err
	default:
	}

	c.nextID++
	id := strconv.FormatUint(c.nextID, 10)
	response := make(chan packet, 1)
	c.pending[id] = response
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.write(map[string]any{"id": json.RawMessage(id), "method": method, "params": params}); err != nil {
		return err
	}

	select {
	case message := <-response:
		if message.Error != nil {
			return fmt.Errorf("%s: %s", method, message.Error.Message)
		}
		if target != nil {
			return json.Unmarshal(message.Result, target)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("%s: %w", method, ctx.Err())
	case <-c.done:
		c.mu.Lock()
		err := c.err
		c.mu.Unlock()
		return err
	}
}

// notify 发送无需响应的协议通知
func (c *client) notify(method string, params any) error {
	return c.write(map[string]any{"method": method, "params": params})
}

// close 关闭协议输入并等待原生进程退出
func (c *client) close() {
	c.stdin.Close()
	select {
	case <-c.done:
	case <-time.After(constant.CodexShutdownTimeout):
		c.cmd.Process.Kill()
		<-c.done
	}
}
