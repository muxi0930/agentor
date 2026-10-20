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
)

type packet struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	writeMu   sync.Mutex
	mu        sync.Mutex
	nextID    uint64
	pending   map[string]chan packet
	done      chan struct{}
	once      sync.Once
	err       error
	onMessage func(packet)
	onExit    func(error)
}

func startClient(path, home string, env []string, onMessage func(packet), onExit func(error)) (*client, error) {
	cmd := exec.Command(path, "app-server")
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
	// Upstream stderr can contain request data. Do not expose it as application logs.
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
			err = errors.New("Codex app-server 已关闭")
		}
		c.finish(err)
	}()
	return c, nil
}

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

func (c *client) write(value any) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return json.NewEncoder(c.stdin).Encode(value)
}

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
	defer func() { c.mu.Lock(); delete(c.pending, id); c.mu.Unlock() }()
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

func (c *client) notify(method string, params any) error {
	return c.write(map[string]any{"method": method, "params": params})
}

func (c *client) close() {
	c.stdin.Close()
	select {
	case <-c.done:
	case <-time.After(2 * time.Second):
		c.cmd.Process.Kill()
		<-c.done
	}
}
