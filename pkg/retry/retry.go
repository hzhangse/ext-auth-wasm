package retry

import (
	"bst-auth/pkg/config"
	"bst-auth/pkg/token"
	"fmt"
	"net/http"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

type RetryContext struct {
	OriginalHeaders [][2]string
	OriginalBody    []byte
	RetryCount      int
	MaxRetries      int
}

const (
	ContextKey = "retry-context"
)

// HandleRetry 处理重试逻辑
func HandleRetry(ctx wrapper.HttpContext, config config.SimpleConfig) types.Action {
	// 获取重试上下文
	retryCtx, ok := ctx.GetContext(ContextKey).(*RetryContext)
	if !ok {
		log.Warn("Failed to get retry context")
		return types.ActionContinue
	}

	// 检查是否达到最大重试次数
	if retryCtx.RetryCount >= retryCtx.MaxRetries {
		log.Infof("Max retries reached (%d), returning response as is", retryCtx.MaxRetries)
		return types.ActionContinue
	}

	// 增加重试计数
	retryCtx.RetryCount++
	ctx.SetContext(ContextKey, retryCtx)

	log.Infof("Retrying request, attempt %d/%d", retryCtx.RetryCount, retryCtx.MaxRetries)

	// 重新发送原始请求
	// 从OriginalHeaders中提取URL相关信息
	var path, authority, method, scheme = "", "", "GET", "http" // 默认值

	for _, header := range retryCtx.OriginalHeaders {
		switch header[0] {
		case ":path":
			path = header[1]
		case ":authority":
			authority = header[1]
		case ":method":
			method = header[1]
		case "scheme":
			scheme = header[1]
		}
	}

	// 如果scheme未在headers中指定，则根据tls信息判断
	if scheme == "http" {
		// 检查是否是 TLS 请求
		for _, header := range retryCtx.OriginalHeaders {
			if header[0] == "x-forwarded-proto" && header[1] == "https" {
				scheme = "https"
				break
			}
		}
	}

	// 检查必要信息是否存在
	if path == "" || authority == "" {
		log.Warnf("❌ Missing required headers - path: '%s', authority: '%s'", path, authority)
		return types.ActionContinue
	}
	// ✅ 构建用于日志的完整 URL（仅用于日志）
	fullURL := fmt.Sprintf("%s://%s%s", scheme, authority, path)
	log.Infof("🔄 Retrying request - URL: %s, Method: %s", fullURL, method)

	// 记录请求体信息（如果存在）
	if len(retryCtx.OriginalBody) > 0 {
		log.Debugf("Request Body: %s", string(retryCtx.OriginalBody))
	}

	// 构建请求头
	headers := make([][2]string, 0, len(retryCtx.OriginalHeaders))

	for _, h := range retryCtx.OriginalHeaders {
		//删除所有伪头（:method, :path, :authority, :scheme）
		switch h[0] {
		case ":method", ":path", ":authority", ":scheme", "host", "Host":
			continue
		}
		headers = append(headers, h)
	}
	//headers = append(headers, [2]string{":authority", "bst-agent.com"}) // ← 你自定义的值

	client := config.TokenService.Client

	// 发送 HTTP 请求
	err := client.Call(method, path, headers, retryCtx.OriginalBody,
		func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			// 记录响应信息
			log.Infof("Received response for retry request - Status: %d", statusCode)

			// 将新的响应发送给客户端
			var respHeaders [][2]string
			for k, v := range responseHeaders {
				respHeaders = append(respHeaders, [2]string{k, v[0]})
			}
			proxywasm.SendHttpResponse(uint32(statusCode), respHeaders, responseBody, -1)
			log.Infof("✅ Retry request completed successfully")
		}, 5000) // 5秒超时

	if err != nil {
		log.Errorf("Failed to send retry request: %v", err)
		return types.ActionContinue
	}

	return types.ActionPause
}

func HandleRetryWithToken(ctx wrapper.HttpContext, config config.SimpleConfig, tm *token.TokenManager) types.Action {
	retryCtx, ok := ctx.GetContext(ContextKey).(*RetryContext)
	if !ok {
		log.Warn("Failed to get retry context")
		return types.ActionContinue
	}

	if retryCtx.RetryCount >= retryCtx.MaxRetries {
		log.Infof("Max retries reached (%d), giving up", retryCtx.MaxRetries)
		return types.ActionContinue
	}

	retryCtx.RetryCount++
	ctx.SetContext(ContextKey, retryCtx)

	log.Infof("Attempting retry %d/%d with fresh token", retryCtx.RetryCount, retryCtx.MaxRetries)

	// 1️⃣ 先发起 token 请求（异步）
	tm.RequestTokenAsync(config, func(token string, err error) {
		if err != nil {
			log.Errorf("Failed to fetch token for retry: %v", err)
			// ❌ 不能在这里调用 AbortWithPanic
			// 改为：发送错误响应
			proxywasm.SendHttpResponse(500, [][2]string{
				{"content-type", "text/plain"},
			}, []byte("Failed to fetch token"), -1)
			return
		}

		log.Infof("✅ Token fetched successfully, length: %d", len(token))

		// 2️⃣ 构建原始请求
		var path, authority, method, scheme = "", "", "GET", "http"
		for _, header := range retryCtx.OriginalHeaders {
			switch header[0] {
			case ":path":
				path = header[1]
			case ":authority":
				authority = header[1]
			case ":method":
				method = header[1]
			case "scheme":
				scheme = header[1]
			}
		}

		if scheme == "http" {
			for _, header := range retryCtx.OriginalHeaders {
				if header[0] == "x-forwarded-proto" && header[1] == "https" {
					scheme = "https"
					break
				}
			}
		}

		if path == "" || authority == "" {
			log.Warnf("❌ Missing required headers - path: '%s', authority: '%s'", path, authority)
			proxywasm.SendHttpResponse(500, [][2]string{{"content-type", "text/plain"}}, []byte("Invalid request"), -1)
			return
		}

		// 构建 headers（注入 token）
		headers := [][2]string{}
		for _, h := range retryCtx.OriginalHeaders {
			switch h[0] {
			case ":method", ":path", ":authority", ":scheme", "host", "Host":
				continue
			}
			headers = append(headers, h)
		}
		headers = append(headers, [2]string{"Authorization", "Bearer " + token})

		// 3️⃣ 发送重试请求
		client := config.TokenService.Client
		err = client.Call(method, path, headers, retryCtx.OriginalBody, func(statusCode int, responseHeaders http.Header, responseBody []byte) {
			var respHeaders [][2]string
			for k, v := range responseHeaders {
				if len(v) > 0 {
					respHeaders = append(respHeaders, [2]string{k, v[0]})
				}
			}
			proxywasm.SendHttpResponse(uint32(statusCode), respHeaders, responseBody, -1)
			log.Infof("✅ Retry request completed with status %d", statusCode)
		}, 5000)

		if err != nil {
			log.Errorf("Failed to send retry request: %v", err)
			proxywasm.SendHttpResponse(500, [][2]string{{"content-type", "text/plain"}}, []byte("Request failed"), -1)
		}
	})

	// 🛑 暂停请求处理，等待 token 获取和重试完成
	return types.ActionPause
}

// InitializeRetryContext 初始化重试上下文
func InitializeRetryContext(ctx wrapper.HttpContext, headers [][2]string, config config.SimpleConfig) *RetryContext {
	retryCtx := &RetryContext{
		OriginalHeaders: headers,
		MaxRetries:      config.TokenConfig.RetrySendTimes,
		RetryCount:      0,
	}
	ctx.SetContext(ContextKey, retryCtx)
	return retryCtx
}

// SetOriginalBody 设置原始请求体
func SetOriginalBody(ctx wrapper.HttpContext, body []byte) {
	retryCtx, ok := ctx.GetContext(ContextKey).(*RetryContext)
	if !ok {
		log.Warn("Failed to get retry context")
		return
	}

	retryCtx.OriginalBody = make([]byte, len(body))
	copy(retryCtx.OriginalBody, body)

	ctx.SetContext(ContextKey, retryCtx)
}
