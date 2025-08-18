# Guide

## **install go enviroment**

use gvm as go version managerment see from [see](https://github.com/moovweb/gvm)

sudo apt-get install bison

1. Install [Bison](https://www.gnu.org/software/bison/):

   `sudo apt-get install bison`
2. Install gvm:

   `bash < <(curl -s -S -L https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer)`
3. Install go:

   `gvm install go1.24.4`
   `gvm use go1.24.4 --default`

## **build and run**

   add to your vscode setting.json

```
    "go.gopath": "/home/ryan/.gvm/pkgsets/go1.24.4/global",
    "go.goroot": "/home/ryan/.gvm/gos/go1.24.4",
```

   `make build`

## 在higress里配置 mcp server

1. 编辑全局配置：higress-config

```yaml
data:
  higress: |-
    mcpServer:
      sse_path_suffix: /sse  # SSE 连接的路径后缀
      enable: true          # 启用 MCP Server
      redis:
        address: 10.10.12.122:6379 # Redis服务地址。这里需要使用本机的内网 IP，不可以使用 127.0.0.1
        username: "" # Redis用户名（可选）
        password: "" # Redis密码（可选）
        db: 0 # Redis数据库（可选）
      match_list:          # MCP Server 会话保持路由规则（当匹配下面路径时，将被识别为一个 MCP 会话，通过 SSE 等机制进行会话保持）
        - match_rule_domain: "*"
          match_rule_path: /user   # 路径对应路由配置里的路由条件路径，这里是前缀匹配
          match_rule_type: "prefix" # 这里是前缀匹配
        - match_rule_domain: "*"
          match_rule_path: /bst-oa-mcp
          match_rule_type: "prefix"
```

2. 创建服务来源
   可以选择固定地址选择服务源,创建之后再服务列表里可以看到具体的服务项目，后缀名staic对应通过ip:port方式添加的服务
3. 添加业务服务路由,可在console上添加，对应其实就是ingress配置，如下所示

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    higress.io/destination: bst-oa-test.static:80
    higress.io/ignore-path-case: "false"
  creationTimestamp: null
  labels:
    higress.io/domain_bst-agent.com: "true"
    higress.io/domain_oa.bst-agent.com: "true"
    higress.io/resource-definer: higress
  name: bst-oa
  namespace: higress-system
spec:
  ingressClassName: higress
  rules:
  - host: bst-agent.com
    http:
      paths:
      - backend:
          resource:
            apiGroup: networking.higress.io
            kind: McpBridge
            name: default
        path: /bst
        pathType: Prefix


```

4. 添加mcp服务路由，添加方式同上一步，但区别在于需要配置两个插件

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  annotations:
    disabled.higress.io/rewrite-path: /bst
    higress.io/destination: bst-oa-test.static:80
    higress.io/enable-rewrite: "false"
    higress.io/ignore-path-case: "false"
  creationTimestamp: null
  labels:
    higress.io/domain_bst-agent.com: "true"
    higress.io/domain_oa.bst-agent.com: "true"
    higress.io/resource-definer: higress
  name: bst-oa-mcp
  namespace: higress-system
spec:
  ingressClassName: higress
  rules:
  - host: bst-agent.com
    http:
      paths:
      - backend:
          resource:
            apiGroup: networking.higress.io
            kind: McpBridge
            name: default
        path: /bst-oa-mcp
        pathType: Prefix

```

   插件1：mcp服务器插件，用于生成对应的mcp服务和工具提示词

```yaml
server:
  name: "bst-oa-mcp-server"
tools:
- args:
  - description: "员工邮箱地址，用于唯一标识员工"
    name: "email"
    position: "body"
    required: true
    type: "string"
  - description: "考勤日期，格式为 yyyy-MM-dd"
    name: "kqdate"
    position: "body"
    required: true
    type: "string"
  description: "查询员工考勤记录 - 根据指定日期和员工邮箱查询当天的打卡详情"
  name: "query_attendance"
  requestTemplate:
    headers:
    - key: "Content-Type"
      value: "application/x-www-form-urlencoded"
    method: "POST"
    url: "http://oa.bst-agent.com/bst/common/attendance/queryDailyDetialInfo"
  responseTemplate:
    prependBody: |-
      # API Response Information

      Below is the response from an API call. To help you understand the data, I've provided:

      1. A detailed description of all fields in the response structure
      2. The complete API response

      ## Response Structure

      > Content-Type: application/json

      - **code**: 状态码，0 表示成功 (Type: integer)
      - **datas**:  (Type: array)
        - **datas[].email**: 员工邮箱 (Type: string)
        - **datas[].kqdate**: 考勤日期 (Type: string)
        - **datas[].onedeptName**: 所属部门 (Type: string)
        - **datas[].signinAddress**: 签到地点 (Type: string)
        - **datas[].signindate**:  (Type: string)
        - **datas[].signintime**: 打卡签到时间（时分秒） (Type: string)
        - **datas[].signoutAddress**: 签退地点 (Type: string)
        - **datas[].signoutdate**:  (Type: string)
        - **datas[].signouttime**: 打卡签退时间（时分秒） (Type: string)
        - **datas[].status**: 状态（1: 正常, 2: 迟到, 3: 早退等） (Type: string)
        - **datas[].subcompanyName**: 所属分公司 (Type: string)
        - **datas[].userId**: 员工编号 (Type: string)
        - **datas[].userName**: 员工姓名 (Type: string)
        - **datas[].workCode**: 工作地点编码 (Type: string)
        - **datas[].workbegintime**: 公司规定上班时间 (Type: string)
        - **datas[].workendtime**: 公司规定下班时间 (Type: string)
      - **message**: 响应消息 (Type: string)

      ## Original Response
```

   插件2：认证插件，用于接口调用时或者获取token或者身份认证

```yaml
gateway_service:
  endpoint:
    service_name: "bst-oa-test"
    service_port: 80
    service_source: "ip"
token_service:
  endpoint:
    service_name: "bst-oa-test"
    service_port: 80
    service_source: "ip"  
token_config:
  credential:
    form_fields:
      appKey: "AgUiMnIUrF2s4b6Y"
      appSecret: "AlW9GWKuPdLqkqKknoh8hSzmTR9917KD"
  enabled: true
  invalid_token_condition: "code==1"
  retry_send_times: 2
  timeout: 3000
  token_extraction:
    response_path: "datas"
  token_injection:
  - format: "{token}"
    key: "token"
    type: "header"
  token_path: "/bst/oa/auth/getToken"

```

## 生成提示词工具使用
```
go install github.com/higress-group/openapi-to-mcpserver/cmd/openapi-to-mcp@latest
openapi-to-mcp --input ./openapi.json --output ./mcp-server.yaml
```
q-mcp-server.yaml就是前面贴给mcp server插件的内容
openapi.json，对应一个openapi描述文件，可以通过接口调用的具体传参和返回值,丢给ai让它帮你生成，例子如下所示

```json
{
  "openapi": "3.0.3",
  "info": {
    "title": "考勤数据查询 API",
    "description": "通过员工邮箱和日期查询考勤打卡记录",
    "version": "1.0.0",
    "contact": {
      "name": "API Support",
      "email": "support@company.com"
    }
  },
  "servers": [
    {
      "url": "http://oa.bst-agent.com/",
      "description": "生产环境"
    }
  ],
  "paths": {
    "/bst/common/attendance/queryDailyDetialInfo": {
      "post": {
        "summary": "查询员工考勤记录",
        "description": "根据指定日期和员工邮箱查询当天的打卡详情",
        "requestBody": {
          "required": true,
          "content": {
            "application/x-www-form-urlencoded": {
              "schema": {
                "type": "object",
                "properties": {
                  "kqdata": {
                    "type": "string",
                    "format": "date",
                    "pattern": "^\\d{4}-\\d{2}-\\d{2}$",
                    "example": "2025-02-21",
                    "description": "考勤日期，格式为 yyyy-MM-dd"
                  },
                  "email": {
                    "type": "string",
                    "format": "email",
                    "example": "richer.yang@bst.ai",
                    "description": "员工邮箱地址，用于唯一标识员工"
                  }
                },
                "required": ["kqdata", "email"],
                "additionalProperties": false
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "成功返回考勤数据",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "code": {
                      "type": "integer",
                      "example": 0,
                      "description": "状态码，0 表示成功"
                    },
                    "message": {
                      "type": "string",
                      "example": "成功",
                      "description": "响应消息"
                    },
                    "datas": {
                      "type": "array",
                      "items": {
                        "type": "object",
                        "properties": {
                          "signintime": {
                            "type": "string",
                            "example": "09:13:59",
                            "description": "打卡签到时间（时分秒）"
                          },
                          "signouttime": {
                            "type": "string",
                            "example": "19:32:47",
                            "description": "打卡签退时间（时分秒）"
                          },
                          "kqdate": {
                            "type": "string",
                            "format": "date",
                            "example": "2025-02-21",
                            "description": "考勤日期"
                          },
                          "userName": {
                            "type": "string",
                            "example": "杨成",
                            "description": "员工姓名"
                          },
                          "userId": {
                            "type": "string",
                            "example": "1332",
                            "description": "员工编号"
                          },
                          "signindate": {
                            "type": "string",
                            "format": "date",
                            "example": "2025-02-21"
                          },
                          "signoutdate": {
                            "type": "string",
                            "format": "date",
                            "example": "2025-02-21"
                          },
                          "workbegintime": {
                            "type": "string",
                            "example": "09:00",
                            "description": "公司规定上班时间"
                          },
                          "workendtime": {
                            "type": "string",
                            "example": "18:00",
                            "description": "公司规定下班时间"
                          },
                          "workCode": {
                            "type": "string",
                            "example": "CN-WH-0218",
                            "description": "工作地点编码"
                          },
                          "signinAddress": {
                            "type": "string",
                            "example": "武汉考勤机 3101",
                            "description": "签到地点"
                          },
                          "signoutAddress": {
                            "type": "string",
                            "example": "武汉考勤机 3101",
                            "description": "签退地点"
                          },
                          "subcompanyName": {
                            "type": "string",
                            "example": "武汉",
                            "description": "所属分公司"
                          },
                          "onedeptName": {
                            "type": "string",
                            "example": "IT",
                            "description": "所属部门"
                          },
                          "email": {
                            "type": "string",
                            "format": "email",
                            "example": "richer.yang@bst.ai",
                            "description": "员工邮箱"
                          },
                          "status": {
                            "type": "string",
                            "example": "1",
                            "description": "状态（1: 正常, 2: 迟到, 3: 早退等）"
                          }
                        },
                        "required": [
                          "signintime",
                          "signouttime",
                          "kqdate",
                          "userName",
                          "userId",
                          "signindate",
                          "signoutdate",
                          "workbegintime",
                          "workendtime",
                          "workCode",
                          "signinAddress",
                          "signoutAddress",
                          "subcompanyName",
                          "onedeptName",
                          "email",
                          "status"
                        ]
                      }
                    }
                  }
                }
              }
            }
          },
          "400": {
            "description": "参数错误（如格式不正确）",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "code": {
                      "type": "integer",
                      "example": 400
                    },
                    "message": {
                      "type": "string",
                      "example": "参数格式错误"
                    }
                  }
                }
              }
            }
          },
          "404": {
            "description": "未找到数据",
            "content": {
              "application/json": {
                "schema": {
                  "type": "object",
                  "properties": {
                    "code": {
                      "type": "integer",
                      "example": 404
                    },
                    "message": {
                      "type": "string",
                      "example": "未查询到该员工的考勤记录"
                    }
                  }
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {}
  }
}

```
