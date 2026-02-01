package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"github.com/brinkmai/adp-openai-gateway/internal/adp"
	"github.com/brinkmai/adp-openai-gateway/internal/handler"
)

func main() {
	// 加载配置
	if err := godotenv.Load(".env"); err != nil {
		log.Println("[Gateway] 未找到.env，使用环境变量")
	}

	// 验证配置
	secretId := os.Getenv("SECRET_ID")
	secretKey := os.Getenv("SECRET_KEY")
	botAppKey := os.Getenv("ADP_BOT_APP_KEY")

	if secretId == "" || secretKey == "" || botAppKey == "" {
		log.Fatal("[Gateway] 错误: 缺少必要环境变量 SECRET_ID, SECRET_KEY, ADP_BOT_APP_KEY")
	}

	// 初始化客户端
	client := adp.NewClient(secretId, secretKey, botAppKey)
	openaiHandler := handler.NewOpenAIHandler(client)

	// 设置Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		return ""
	}))

	// 路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "adp-openai-gateway-go",
		})
	})

	r.GET("/v1/models", openaiHandler.GetModels)
	r.POST("/v1/chat/completions", openaiHandler.ChatCompletions)

	// 获取端口
	port := os.Getenv("PORT")
	if port == "" {
		port = "3100"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	addr := host + ":" + port

	log.Printf("[Gateway] ADP-OpenAI Gateway (Go) 运行在 %s", addr)
	log.Printf("[Gateway] API端点: http://%s/v1/chat/completions", addr)

	// 优雅关闭
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		log.Println("[Gateway] 收到关闭信号，正在关闭...")
		client.Disconnect()
		os.Exit(0)
	}()

	if err := r.Run(addr); err != nil {
		log.Fatalf("[Gateway] 启动失败: %v", err)
	}
}
