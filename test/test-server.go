package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

// 定义返回的 JSON 结构
type Response struct {
	Code  int    `json:"code"`
	Datas string `json:"datas"`
}

type Response1 struct {
	Code  int    `json:"code"`
	Value string `json:"value"`
}

// 全局计数器，用于 /test 接口
var (
	counterMu    sync.Mutex // 互斥锁
	requestCount int        // 请求计数器
)

func main() {
	// 初始化随机数种子
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/bst/oa/auth/getToken", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-External-Token", "test-token")

		randomValue := fmt.Sprintf("random-%d", rand.Intn(100000))

		response := Response{
			Code:  0,
			Datas: randomValue,
		}

		json.NewEncoder(w).Encode(response)
	})

	http.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-External-Token", "test-token")

		// ✅ 获取 token
		token := r.Header.Get("Authorization")
		if token == "" {
			token = r.Header.Get("Token")
		}
		if token == "" {
			token = r.Header.Get("token")
		}

		// ✅ 如果没有 token，返回错误
		if token == "" {
			response := Response1{
				Code:  1,
				Value: "wrong",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// ✅ 验证 token 格式
		token = strings.TrimSpace(token)
		if strings.HasPrefix(token, "Bearer ") {
			token = strings.TrimPrefix(token, "Bearer ")
		}
		if token == "" {
			response := Response1{
				Code:  1,
				Value: "wrong",
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// ✅ 计数逻辑：加锁
		counterMu.Lock()
		requestCount++
		current := requestCount
		counterMu.Unlock()

		// ✅ 判断是否是第 4 次、第 8 次、第 12 次……
		if current%4 == 0 {
			response := Response1{
				Code:  1,
				Value: fmt.Sprintf("blocked-on-%d", current),
			}
			json.NewEncoder(w).Encode(response)
			return
		}

		// ✅ 否则返回正常数据
		randomValue := fmt.Sprintf("random-%d", rand.Intn(100000))
		response := Response1{
			Code:  0,
			Value: randomValue,
		}
		json.NewEncoder(w).Encode(response)
	})

	fmt.Println("Test service started at :8084")
	if err := http.ListenAndServe(":8084", nil); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
