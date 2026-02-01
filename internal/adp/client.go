package adp

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/brinkmai/adp-openai-gateway/internal/token"
)

// Client ADP WebSocket客户端
type Client struct {
	tokenService    *token.Service
	conn            *websocket.Conn
	pendingRequests sync.Map
	isAuthenticated bool
	mu              sync.Mutex
}

// PendingRequest 等待中的请求
type PendingRequest struct {
	ResultCh      chan *ChatResult
	ErrorCh       chan error
	Stream        bool
	OnChunk       func(chunk Chunk)
	FullContent   string
	LastContent   string
}

// ChatResult 聊天结果
type ChatResult struct {
	Content   string
	RequestID string
}

// Chunk 流式数据块
type Chunk struct {
	Type    string // content, thought, done
	Content string
	IsEnd   bool
}

// Message OpenAI格式消息
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string 或 []ContentPart
}

// ContentPart 多模态内容部分
type ContentPart struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}

// ImageURL 图片URL
type ImageURL struct {
	URL string `json:"url"`
}

// ChatOptions 聊天选项
type ChatOptions struct {
	SessionID      string
	Stream         bool
	OnChunk        func(chunk Chunk)
	IncludeThought bool
	Timeout        time.Duration
}

// NewClient 创建ADP客户端
func NewClient(secretId, secretKey, botAppKey string) *Client {
	return &Client{
		tokenService: token.NewService(secretId, secretKey, botAppKey),
	}
}

// ensureConnected 确保WebSocket连接
func (c *Client) ensureConnected() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil && c.isAuthenticated {
		log.Println("[ADPClient] WebSocket已连接，复用现有连接")
		return nil
	}

	// 关闭旧连接
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.isAuthenticated = false
	}

	log.Println("[ADPClient] ========== 建立WebSocket连接 ==========")
	wsToken, err := c.tokenService.GetWsToken()
	if err != nil {
		return fmt.Errorf("获取Token失败: %w", err)
	}

	wsURL := "wss://wss.lke.cloud.tencent.com/v1/qbot/chat/conn/?EIO=4&transport=websocket"
	log.Printf("[ADPClient] WebSocket URL: %s", wsURL)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("WebSocket连接失败: %w", err)
	}
	c.conn = conn

	// 等待握手和鉴权
	authDone := make(chan error, 1)
	
	go func() {
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				authDone <- fmt.Errorf("读取消息失败: %w", err)
				return
			}

			msg := string(message)
			log.Printf("[ADPClient] 收到消息: %s", truncate(msg, 200))

			if msg == "0" || strings.HasPrefix(msg, "0{") {
				// 服务器握手响应，发送鉴权
				authMsg := fmt.Sprintf(`40{"token":"%s"}`, wsToken)
				log.Printf("[ADPClient] 发送鉴权消息")
				if err := conn.WriteMessage(websocket.TextMessage, []byte(authMsg)); err != nil {
					authDone <- fmt.Errorf("发送鉴权失败: %w", err)
					return
				}
			} else if msg == "40" || strings.HasPrefix(msg, "40{") {
				// 鉴权成功
				log.Println("[ADPClient] 鉴权成功！")
				c.isAuthenticated = true
				authDone <- nil
				
				// 继续处理后续消息
				c.handleMessages()
				return
			} else if strings.HasPrefix(msg, "44") {
				// 错误消息
				authDone <- fmt.Errorf("连接错误: %s", msg[2:])
				return
			}
		}
	}()

	select {
	case err := <-authDone:
		return err
	case <-time.After(10 * time.Second):
		conn.Close()
		return fmt.Errorf("连接超时")
	}
}

// handleMessages 处理WebSocket消息
func (c *Client) handleMessages() {
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("[ADPClient] 读取消息错误: %v", err)
			c.isAuthenticated = false
			return
		}

		msg := string(message)
		log.Printf("[ADPClient] 收到消息: %s", truncate(msg, 200))

		if msg == "2" {
			// 心跳ping，回复pong
			c.conn.WriteMessage(websocket.TextMessage, []byte("3"))
		} else if strings.HasPrefix(msg, "42") {
			c.handleSocketIOMessage(msg)
		}
	}
}

// handleSocketIOMessage 处理Socket.IO格式消息
func (c *Client) handleSocketIOMessage(msg string) {
	content := msg[2:]
	
	// 跳过可能的消息ID数字
	bracketIndex := strings.Index(content, "[")
	if bracketIndex > 0 {
		content = content[bracketIndex:]
	}

	var parsed []json.RawMessage
	if err := json.Unmarshal([]byte(content), &parsed); err != nil {
		log.Printf("[ADPClient] 解析消息失败: %v", err)
		return
	}

	if len(parsed) < 2 {
		log.Printf("[ADPClient] 消息格式不正确")
		return
	}

	var eventName string
	if err := json.Unmarshal(parsed[0], &eventName); err != nil {
		log.Printf("[ADPClient] 解析事件名失败: %v", err)
		return
	}

	log.Printf("[ADPClient] 收到事件[%s]: %s", eventName, truncate(string(parsed[1]), 200))

	switch eventName {
	case "reply":
		c.handleEvent("reply", parsed[1])
	case "thought":
		c.handleEvent("thought", parsed[1])
	case "error":
		log.Printf("[ADPClient] 服务器返回错误: %s", string(parsed[1]))
	}
}

// handleEvent 处理ADP事件
func (c *Client) handleEvent(eventType string, data json.RawMessage) {
	var wrapper struct {
		Payload struct {
			RequestID string `json:"request_id"`
			Content   string `json:"content"`
			CanRating bool   `json:"can_rating"`
			IsFinal   bool   `json:"is_final"`
			Thought   string `json:"thought"`
		} `json:"payload"`
	}

	if err := json.Unmarshal(data, &wrapper); err != nil {
		log.Printf("[ADPClient] 解析事件数据失败: %v", err)
		return
	}

	payload := wrapper.Payload
	requestID := payload.RequestID

	// 查找对应的请求
	var req *PendingRequest
	if v, ok := c.pendingRequests.Load(requestID); ok {
		req = v.(*PendingRequest)
	} else {
		// 如果找不到，尝试用第一个pending request
		c.pendingRequests.Range(func(key, value any) bool {
			log.Printf("[ADPClient] 使用fallback请求: %v", key)
			req = value.(*PendingRequest)
			requestID = key.(string)
			return false
		})
	}

	if req == nil {
		log.Printf("[ADPClient] 未找到请求: %s", payload.RequestID)
		return
	}

	log.Printf("[ADPClient] 处理事件: %s, can_rating: %v, is_final: %v, content长度: %d",
		eventType, payload.CanRating, payload.IsFinal, len(payload.Content))

	switch eventType {
	case "reply":
		// 忽略 can_rating=false 的回显消息
		if !payload.CanRating {
			log.Println("[ADPClient] 跳过回显消息（can_rating=false）")
			return
		}

		if req.Stream && req.OnChunk != nil {
			if payload.Content != "" && payload.Content != req.LastContent {
				newContent := payload.Content
				if req.FullContent != "" && strings.HasPrefix(payload.Content, req.FullContent) {
					newContent = payload.Content[len(req.FullContent):]
				}
				
				if newContent != "" {
					log.Printf("[ADPClient] 发送增量内容: %s...", truncate(newContent, 50))
					req.OnChunk(Chunk{
						Type:    "content",
						Content: newContent,
						IsEnd:   payload.IsFinal,
					})
				}
				req.LastContent = payload.Content
			}
		}

		if payload.Content != "" {
			req.FullContent = payload.Content
		}

		// can_rating=true 且 is_final=true 时结束
		if payload.CanRating && payload.IsFinal {
			log.Printf("[ADPClient] 收到最终响应，完成请求，内容: %s...", truncate(payload.Content, 50))
			
			if !req.Stream {
				req.ResultCh <- &ChatResult{
					Content:   req.FullContent,
					RequestID: requestID,
				}
			} else if req.OnChunk != nil {
				req.OnChunk(Chunk{Type: "done"})
			}
			c.pendingRequests.Delete(requestID)
		}

	case "thought":
		if req.Stream && req.OnChunk != nil && payload.Thought != "" {
			req.OnChunk(Chunk{
				Type:    "thought",
				Content: payload.Thought,
			})
		}
	}
}

// Chat 发送聊天请求
func (c *Client) Chat(messages []Message, opts ChatOptions) (*ChatResult, error) {
	if err := c.ensureConnected(); err != nil {
		return nil, err
	}

	requestID := uuid.New().String()
	sessionID := opts.SessionID
	if sessionID == "" {
		sessionID = uuid.New().String()
	}

	// 获取最后一条消息内容
	lastMsg := messages[len(messages)-1]
	var content string
	
	switch v := lastMsg.Content.(type) {
	case string:
		content = v
	case []interface{}:
		for _, part := range v {
			if p, ok := part.(map[string]interface{}); ok {
				if p["type"] == "text" {
					if text, ok := p["text"].(string); ok {
						content += text
					}
				}
			}
		}
	}

	// 构建消息
	payload := map[string]interface{}{
		"payload": map[string]interface{}{
			"session_id": sessionID,
			"request_id": requestID,
			"content":    content,
		},
	}

	log.Println("[ADPClient] ========== 发送聊天请求 ==========")
	log.Printf("[ADPClient] request_id: %s", requestID)
	log.Printf("[ADPClient] session_id: %s", sessionID)
	log.Printf("[ADPClient] 消息内容: %s", truncate(content, 100))
	log.Printf("[ADPClient] Stream模式: %v", opts.Stream)

	req := &PendingRequest{
		ResultCh: make(chan *ChatResult, 1),
		ErrorCh:  make(chan error, 1),
		Stream:   opts.Stream,
		OnChunk:  opts.OnChunk,
	}
	c.pendingRequests.Store(requestID, req)

	// 发送消息
	payloadBytes, _ := json.Marshal(payload)
	msg := fmt.Sprintf(`42["send",%s]`, string(payloadBytes))
	log.Printf("[ADPClient] 发送消息: %s", truncate(msg, 250))

	c.mu.Lock()
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(msg))
	c.mu.Unlock()
	if err != nil {
		c.pendingRequests.Delete(requestID)
		return nil, fmt.Errorf("发送消息失败: %w", err)
	}

	// 等待响应
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	if opts.Stream {
		// 流式模式，等待done
		select {
		case <-time.After(timeout):
			c.pendingRequests.Delete(requestID)
			return nil, fmt.Errorf("请求超时")
		case err := <-req.ErrorCh:
			return nil, err
		case result := <-req.ResultCh:
			return result, nil
		}
	}

	select {
	case <-time.After(timeout):
		c.pendingRequests.Delete(requestID)
		return nil, fmt.Errorf("请求超时")
	case err := <-req.ErrorCh:
		return nil, err
	case result := <-req.ResultCh:
		return result, nil
	}
}

// Disconnect 断开连接
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
		c.isAuthenticated = false
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
