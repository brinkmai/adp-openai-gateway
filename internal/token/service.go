package token

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Service 腾讯云ADP Token服务
type Service struct {
	secretId     string
	secretKey    string
	botAppKey    string
	cachedToken  string
	expireTime   time.Time
	mu           sync.RWMutex
}

// NewService 创建Token服务
func NewService(secretId, secretKey, botAppKey string) *Service {
	return &Service{
		secretId:  secretId,
		secretKey: secretKey,
		botAppKey: botAppKey,
	}
}

// GetWsToken 获取WebSocket Token
func (s *Service) GetWsToken() (string, error) {
	s.mu.RLock()
	if s.cachedToken != "" && time.Now().Add(30*time.Second).Before(s.expireTime) {
		log.Println("[TokenService] 使用缓存Token")
		token := s.cachedToken
		s.mu.RUnlock()
		return token, nil
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	// 再次检查（防止并发）
	if s.cachedToken != "" && time.Now().Add(30*time.Second).Before(s.expireTime) {
		return s.cachedToken, nil
	}

	action := "GetWsToken"
	payload := map[string]interface{}{
		"Type":      5, // API访客模式
		"BotAppKey": s.botAppKey,
	}

	log.Println("[TokenService] ========== 请求Token ==========")
	log.Printf("[TokenService] Action: %s", action)
	log.Printf("[TokenService] BotAppKey: %s...", s.botAppKey[:min(8, len(s.botAppKey))])

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("序列化请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", "https://lke.tencentcloudapi.com/", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	// 构建签名
	timestamp := time.Now().Unix()
	s.buildHeaders(req, action, body, timestamp)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	log.Printf("[TokenService] HTTP状态码: %d", resp.StatusCode)
	log.Printf("[TokenService] 响应: %s", string(respBody))

	var result struct {
		Response struct {
			Token string `json:"Token"`
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if result.Response.Error != nil {
		return "", fmt.Errorf("API错误: %s - %s", result.Response.Error.Code, result.Response.Error.Message)
	}

	s.cachedToken = result.Response.Token
	s.expireTime = time.Now().Add(4*time.Minute + 30*time.Second)

	log.Printf("[TokenService] Token获取成功, 长度: %d", len(s.cachedToken))
	return s.cachedToken, nil
}

// buildHeaders 构建请求头（TC3签名）
func (s *Service) buildHeaders(req *http.Request, action string, payload []byte, timestamp int64) {
	service := "lke"
	host := "lke.tencentcloudapi.com"
	region := "ap-guangzhou"
	version := "2023-11-30"
	contentType := "application/json"

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")

	// CanonicalRequest
	httpRequestMethod := "POST"
	canonicalUri := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		contentType, host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedPayload := sha256Hash(payload)
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalUri, canonicalQueryString, canonicalHeaders, signedHeaders, hashedPayload)

	// StringToSign
	algorithm := "TC3-HMAC-SHA256"
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256Hash([]byte(canonicalRequest))
	stringToSign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, timestamp, credentialScope, hashedCanonicalRequest)

	// Signature
	signature := s.sign(date, service, stringToSign)

	// Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, s.secretId, credentialScope, signedHeaders, signature)

	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", authorization)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", timestamp))
	req.Header.Set("X-TC-Region", region)
}

// sign 生成TC3签名
func (s *Service) sign(date, service, stringToSign string) string {
	kDate := hmacSHA256([]byte("TC3"+s.secretKey), []byte(date))
	kService := hmacSHA256(kDate, []byte(service))
	kSigning := hmacSHA256(kService, []byte("tc3_request"))
	return hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))
}

func sha256Hash(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
