package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/brinkmai/adp-openai-gateway/internal/adp"
)

// OpenAIHandler OpenAI协议处理器
type OpenAIHandler struct {
	client *adp.Client
}

// NewOpenAIHandler 创建处理器
func NewOpenAIHandler(client *adp.Client) *OpenAIHandler {
	return &OpenAIHandler{client: client}
}

// ChatRequest 聊天请求
type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []adp.Message `json:"messages"`
	Stream   bool          `json:"stream"`
}

// GetModels 获取模型列表
func (h *OpenAIHandler) GetModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data": []gin.H{
			{
				"id":       "adp-default",
				"object":   "model",
				"created":  time.Now().Unix(),
				"owned_by": "tencent-adp",
			},
		},
	})
}

// ChatCompletions 处理聊天完成请求
func (h *OpenAIHandler) ChatCompletions(c *gin.Context) {
	var req ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "Invalid request body",
				"type":    "invalid_request_error",
				"code":    "invalid_request",
			},
		})
		return
	}

	if len(req.Messages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{
				"message": "messages is required and must be a non-empty array",
				"type":    "invalid_request_error",
				"code":    "invalid_messages",
			},
		})
		return
	}

	requestID := fmt.Sprintf("chatcmpl-%s", uuid.New().String())
	created := time.Now().Unix()
	model := req.Model
	if model == "" {
		model = "adp-default"
	}

	if req.Stream {
		h.handleStreamRequest(c, req.Messages, requestID, created, model)
	} else {
		h.handleNonStreamRequest(c, req.Messages, requestID, created, model)
	}
}

func (h *OpenAIHandler) handleNonStreamRequest(c *gin.Context, messages []adp.Message, requestID string, created int64, model string) {
	result, err := h.client.Chat(messages, adp.ChatOptions{
		Stream: false,
	})

	if err != nil {
		log.Printf("[OpenAIHandler] 请求失败: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": err.Error(),
				"type":    "api_error",
				"code":    "internal_error",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"id":      requestID,
		"object":  "chat.completion",
		"created": created,
		"model":   model,
		"choices": []gin.H{
			{
				"index": 0,
				"message": gin.H{
					"role":    "assistant",
					"content": result.Content,
				},
				"finish_reason": "stop",
			},
		},
		"usage": gin.H{
			"prompt_tokens":     0,
			"completion_tokens": 0,
			"total_tokens":      0,
		},
	})
}

func (h *OpenAIHandler) handleStreamRequest(c *gin.Context, messages []adp.Message, requestID string, created int64, model string) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Streaming not supported",
				"type":    "api_error",
			},
		})
		return
	}

	done := make(chan struct{})
	errCh := make(chan error, 1)

	go func() {
		_, err := h.client.Chat(messages, adp.ChatOptions{
			Stream: true,
			OnChunk: func(chunk adp.Chunk) {
				var data []byte
				
				switch chunk.Type {
				case "content":
					data, _ = json.Marshal(gin.H{
						"id":      requestID,
						"object":  "chat.completion.chunk",
						"created": created,
						"model":   model,
						"choices": []gin.H{
							{
								"index":         0,
								"delta":         gin.H{"content": chunk.Content},
								"finish_reason": nil,
							},
						},
					})
					c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
					flusher.Flush()

				case "done":
					data, _ = json.Marshal(gin.H{
						"id":      requestID,
						"object":  "chat.completion.chunk",
						"created": created,
						"model":   model,
						"choices": []gin.H{
							{
								"index":         0,
								"delta":         gin.H{},
								"finish_reason": "stop",
							},
						},
					})
					c.Writer.Write([]byte(fmt.Sprintf("data: %s\n\n", data)))
					c.Writer.Write([]byte("data: [DONE]\n\n"))
					flusher.Flush()
					close(done)
				}
			},
		})

		if err != nil {
			errCh <- err
		}
	}()

	select {
	case <-done:
		return
	case err := <-errCh:
		log.Printf("[OpenAIHandler] 流式请求失败: %v", err)
		// 错误已经无法发送到客户端，因为已经开始流式响应
		return
	case <-c.Request.Context().Done():
		return
	}
}
