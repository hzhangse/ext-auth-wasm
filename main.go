package main

import (
	"bst-auth/pkg/config"
	"bst-auth/pkg/retry"
	"bst-auth/pkg/token"

	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
	"github.com/higress-group/wasm-go/pkg/wrapper"
)

func main() {}

const (
	ContextKey = "retry-context"
)

func init() {
	wrapper.SetCtx(
		"token-server",
		wrapper.ParseConfig(config.ParseConfig),
		wrapper.ProcessRequestHeaders(onHttpRequestHeaders),
		wrapper.ProcessRequestBody(onHttpRequestBody),
		wrapper.ProcessResponseHeaders(onHttpResponseHeaders),
		wrapper.ProcessResponseBody(onHttpResponseBody), //
	)
}

func onHttpRequestHeaders(ctx wrapper.HttpContext, config config.SimpleConfig) types.Action {
	log.Infof("on HttpRequest Headers start")
	if !config.TokenConfig.Enabled {
		return types.ActionContinue
	}

	headers, err := proxywasm.GetHttpRequestHeaders()

	if err != nil {
		log.Warnf("Failed to get request headers: %v", err)
		return types.ActionContinue
	}

	// 初始化重试上下文
	retry.InitializeRetryContext(ctx, headers, config)

	return token.GetTokenManager().FetchToken(config)
}

func onHttpRequestBody(ctx wrapper.HttpContext, config config.SimpleConfig, body []byte) types.Action {
	log.Infof("on HttpRequest Body start")
	if !config.TokenConfig.Enabled {
		return types.ActionContinue
	}

	retry.SetOriginalBody(ctx, body)

	return types.ActionContinue
}

func onHttpResponseHeaders(ctx wrapper.HttpContext, config config.SimpleConfig) types.Action {
	log.Infof("on HttpResponse Headers start")
	if !config.TokenConfig.Enabled {
		return types.ActionContinue
	}

	// 检查 Content-Type 是否为 JSON
	contentType, err := proxywasm.GetHttpResponseHeader("content-type")
	if err != nil || contentType == "" {
		return types.ActionContinue
	}

	if contentType != "application/json" && contentType != "application/json; charset=utf-8" {
		return types.ActionContinue
	}
	// 暂停响应处理，等待检查响应体
	return types.HeaderStopIteration // 暂停响应处理，等待检查响应体
}

func onHttpResponseBody(ctx wrapper.HttpContext, config config.SimpleConfig, body []byte) types.Action {
	log.Infof("on token wasm plugin  HttpResponse Body start")
	if !config.TokenConfig.Enabled {
		return types.ActionContinue
	}

	if token.GetTokenManager().IsTokenInvalid(body, config) {

		// 处理重试逻辑
		return retry.HandleRetryWithToken(ctx, config, token.GetTokenManager())
	}
	log.Infof("on token wasm plugin HttpResponse Body end")
	return types.ActionContinue
}
