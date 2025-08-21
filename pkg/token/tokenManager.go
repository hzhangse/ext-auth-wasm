// pkg/token/global_manager.go
package token

import (
	"bst-auth/pkg/config"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/expr-lang/expr"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm"
	"github.com/higress-group/proxy-wasm-go-sdk/proxywasm/types"
	"github.com/higress-group/wasm-go/pkg/log"
)

var (
	globalTokenManager *TokenManager
	once               sync.Once
)

// GetTokenManager 获取全局唯一的 TokenManager
func GetTokenManager() *TokenManager {
	once.Do(func() {
		globalTokenManager = &TokenManager{
			token:        "",             // 当前 Token
			tokenMutex:   sync.RWMutex{}, // 保护 token 读写
			refreshMutex: sync.Mutex{},   // 防止并发刷新（关键！）
		}
	})
	return globalTokenManager
}

// TokenManager 管理全局 Token
// ✅ 设计原则：
//   - tokenMutex: 保护 token 变量本身（数据安全）
//   - refreshMutex: 保护“刷新行为”不被重复执行（行为安全）
type TokenManager struct {
	token        string       // 当前有效的 Token
	tokenMutex   sync.RWMutex // 读写锁：允许多读单写
	refreshMutex sync.Mutex   // 互斥锁：确保同一时间只有一个在刷新
}

// GetToken 获取当前 Token
// ✅ 读锁：允许多个 Goroutine 并发读取，无阻塞
func (tm *TokenManager) GetToken() string {
	tm.tokenMutex.RLock()
	defer tm.tokenMutex.RUnlock()
	return tm.token
}

// ClearToken 清空Token（使用写锁）
// ✅ 这个方法是关键！让外部能安全地清空Token
func (tm *TokenManager) ClearToken() {
	tm.tokenMutex.Lock()
	defer tm.tokenMutex.Unlock()
	tm.token = ""
	log.Infof("✅ Token clear")
}

func (tm *TokenManager) FetchToken(config config.SimpleConfig) types.Action {
	// 🔐 第一层锁：防惊群
	tm.refreshMutex.Lock()
	defer tm.refreshMutex.Unlock()

	// 🚪 再次检查 token 是否已存在（别人可能已经刷新好了）
	tm.tokenMutex.RLock()
	if tm.token != "" {
		log.Infof("✅ Token 已存在，直接复用")
		tm.tokenMutex.RUnlock()
		tm.InjectToken(config, nil)
		return types.ActionContinue
	}
	tm.tokenMutex.RUnlock()

	// 🌐 现在开始获取 token（异步）
	tm.RequestTokenAsync(config, func(token string, err error) {
		if err != nil {
			log.Errorf("❌ 获取 token 失败: %v", err)
			// ❌ 不能在这里 return，要通知 Envoy
			tm.sendResponse(500, "token.fetch.failed", nil, nil)
			return
		}

		if token == "" {
			log.Errorf("❌ 获取到空 token")
			tm.sendResponse(500, "token.empty", nil, nil)
			return
		}

		// ✅ 成功获取 token
		log.Infof("✅ 成功获取 token，长度: %d", len(token))
		tm.InjectToken(config, nil) // 注入到当前请求
		log.Debugf("恢复原始请求处理")

		// 🎉 恢复被暂停的请求
		proxywasm.ResumeHttpRequest()
	})

	// ⏸️ 暂停当前请求，等待 token 获取完成
	log.Debugf("暂停请求处理，等待 token 获取完成")
	return types.HeaderStopAllIterationAndWatermark
}

func (tm *TokenManager) RequestTokenAsync(config config.SimpleConfig, callback func(string, error)) {
	// 构建请求...
	formData := make(url.Values)
	for k, v := range config.TokenConfig.Credential.FormFields {
		formData.Set(k, v)
	}
	body := []byte(formData.Encode())

	headers := [][2]string{{"content-type", "application/x-www-form-urlencoded"}}
	for k, v := range config.TokenConfig.Credential.HeadFields {
		headers = append(headers, [2]string{k, v})
	}

	err := config.TokenService.Client.Call(
		"POST", config.TokenConfig.TokenPath, headers, body,
		func(statusCode int, h http.Header, body []byte) {
			if statusCode == 200 {
				token, err := tm.extractTokenFromResponse(body, config.TokenConfig.TokenExtraction.ResponsePath)
				if err == nil && token != "" {
					tm.tokenMutex.Lock()
					tm.token = token
					tm.tokenMutex.Unlock()
					callback(token, nil)
					return
				}
				callback("", fmt.Errorf("extract failed: %v", err))
				return
			}
			callback("", fmt.Errorf("http %d", statusCode))
		},
		config.TokenConfig.Timeout,
	)

	if err != nil {
		callback("", fmt.Errorf("call failed: %w", err))
	}
}

// extractTokenFromResponse 从响应中提取 Token
func (tm *TokenManager) extractTokenFromResponse(responseBody []byte, responsePath string) (string, error) {
	if responsePath == "" {
		responsePath = "datas"
	}

	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %v", err)
	}

	current := response
	fields := strings.Split(responsePath, ".")
	for i, field := range fields {
		if i == len(fields)-1 {
			if token, ok := current[field].(string); ok && token != "" {
				return token, nil
			}
			return "", fmt.Errorf("路径 %s 未找到有效 Token", responsePath)
		}
		if next, ok := current[field].(map[string]interface{}); ok {
			current = next
		} else {
			return "", fmt.Errorf("路径中断于: %s", field)
		}
	}
	return "", fmt.Errorf("未找到 Token")
}

func (tm *TokenManager) InjectToken(config config.SimpleConfig, body []byte) {
	log.Infof("开始注入token到请求中")

	// 从上下文中获取token
	token := tm.GetToken()
	if token == "" {
		return
	}
	// 根据配置注入token
	for _, injection := range config.TokenConfig.TokenInjection {
		log.Debugf("根据配置注入token，类型: %s，键: %s，格式: %s", injection.Type, injection.Key, injection.Format)

		// 替换格式中的{token}占位符
		formattedValue := strings.Replace(injection.Format, "{token}", token, -1)

		switch injection.Type {
		case "header":
			log.Debugf("将token注入到请求头: %s", injection.Key)
			_ = proxywasm.AddHttpRequestHeader(injection.Key, formattedValue)
		case "form_body", "form":
			log.Debugf("将token注入到表单体，键: %s", injection.Key)
			modifiedBody := tm.addTokenToFormBody(body, formattedValue, injection.Key)
			_ = proxywasm.ReplaceHttpRequestBody(modifiedBody)
		default:
			log.Warnf("未知的注入类型: %s", injection.Type)
		}
	}

	log.Infof("Token注入完成")
}

// addTokenToFormBody 将token添加到表单数据中
func (tm *TokenManager) addTokenToFormBody(originalBody []byte, token string, tokenFieldName string) []byte {
	if tokenFieldName == "" {
		tokenFieldName = "token" // default field name
	}

	log.Debugf("将token添加到表单数据，字段名: %s", tokenFieldName)

	// Parse original form data
	originalValues, err := url.ParseQuery(string(originalBody))
	if err != nil {
		// If parsing fails, assume original body is empty or not in form format
		log.Warnf("解析原始表单数据失败，创建新的表单: %v", err)
		originalValues = make(url.Values)
	}

	// Add token field
	originalValues.Set(tokenFieldName, token)
	log.Debugf("已设置token字段")

	// Return modified form data
	result := []byte(originalValues.Encode())
	log.Debugf("修改后的表单数据: %s", string(result))
	return result
}

func (tm *TokenManager) sendResponse(statusCode uint32, statusCodeDetailData string, headers http.Header, body []byte) error {
	var ret [][2]string
	for k, vs := range headers {
		for _, v := range vs {
			ret = append(ret, [2]string{k, v})
		}
	}

	return proxywasm.SendHttpResponseWithDetail(statusCode, statusCodeDetailData, ret, body, -1)
}

// IsTokenInvalid 判断响应是否表示 token 无效
func (tm *TokenManager) IsTokenInvalid(responseBody []byte, config config.SimpleConfig) bool {
	log.Debugf("使用表达式检查token是否无效，条件: %s，响应体: %s",
		config.TokenConfig.InvalidTokenCondition, string(responseBody))

	// 如果没有配置条件，使用默认方法
	if config.TokenConfig.InvalidTokenCondition == "" {
		return false
	}

	// 解析响应 JSON
	var response map[string]interface{}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Errorf("解析响应JSON失败: %v", err)
		return false
	}

	// 构建表达式执行环境
	env := make(map[string]interface{})

	// 1. 注入自定义函数（如 contains）
	env["contains"] = func(a, b interface{}) bool {
		s, ok1 := a.(string)
		sub, ok2 := b.(string)
		return ok1 && ok2 && strings.Contains(s, sub)
	}

	// Add the has function to check if a field exists
	env["has"] = func(mapVar map[string]interface{}, key string) bool {
		_, exists := mapVar[key]
		return exists
	}

	// 2. 注入顶层字段，支持直接写 code != 0
	for k, v := range response {
		// 避免覆盖自定义函数（如 contains）
		if _, exists := env[k]; !exists {
			env[k] = v
		}
	}

	// 3. 注入整个 response，支持嵌套访问（如 result.succ）
	env["response"] = response

	// 编译表达式
	program, err := expr.Compile(config.TokenConfig.InvalidTokenCondition, expr.Env(env))
	if err != nil {
		log.Errorf("编译表达式失败: %v", err)
		return false
	}

	// 执行表达式
	output, err := expr.Run(program, env)
	if err != nil {
		log.Errorf("执行表达式失败: %v", err)
		return false
	}

	// 检查返回值是否为布尔类型
	result, ok := output.(bool)
	if !ok {
		log.Warnf("表达式返回值不是布尔类型: %T, 值: %v", output, output)
		return false
	}

	if result {
		log.Infof("❌表达式判断为 true，token 无效，需要重新获取")
	} else {
		log.Debugf("✅  表达式判断为 false，token 有效")
	}

	return result
}
