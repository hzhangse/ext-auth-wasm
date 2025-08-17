package config

import (
	"github.com/higress-group/wasm-go/pkg/wrapper"
	"github.com/tidwall/gjson"
)

type TokenConfig struct {
	Enabled               bool             `json:"enabled"`
	Credential            Credential       `json:"credential"`
	TokenPath             string           `json:"token_path"`
	Timeout               uint32           `json:"timeout"`
	TokenExtraction       TokenExtraction  `json:"token_extraction"`
	TokenInjection        []TokenInjection `json:"token_injection"`
	InvalidTokenCondition string           `json:"invalid_token_condition"`
	RetrySendTimes        int              `json:"retry_send_times"`
}

type Credential struct {
	FormFields map[string]string `json:"form_fields"`
	HeadFields map[string]string `json:"head_fields"`
}

type TokenExtraction struct {
	ResponsePath string `json:"response_path"`
}

type TokenInjection struct {
	Type   string `json:"type"` // header, form_body
	Key    string `json:"key"`
	Format string `json:"format"`
}

type SimpleConfig struct {
	TokenConfig  TokenConfig `json:"token_config"`
	TokenService HttpService `json:"token_service"`
	GwService    HttpService `json:"gateway_service"`
}

type HttpService struct {
	Client wrapper.HttpClient
	// IP       string
	// Port     int
	// Service  string
	// Protocol string
}

// CreateClusterClient 根据服务来源创建集群客户端
func CreateClusterClient(json gjson.Result) (wrapper.HttpClient, error) {
	serviceName := json.Get("service_name").String()
	servicePort := int64(80)
	serviceSource := ""
	if json.Get("service_port").Exists() {
		servicePort = json.Get("service_port").Int()
	}
	if json.Get("service_source").Exists() {
		serviceSource = json.Get("service_source").String()
	}

	switch serviceSource {
	case "k8s":
		namespace := json.Get("namespace").String()
		return wrapper.NewClusterClient(wrapper.K8sCluster{
			ServiceName: serviceName,
			Namespace:   namespace,
			Port:        servicePort,
		}), nil
	case "nacos":
		namespace := json.Get("namespace").String()
		return wrapper.NewClusterClient(wrapper.NacosCluster{
			ServiceName: serviceName,
			NamespaceID: namespace,
			Port:        servicePort,
		}), nil
	case "ip":
		return wrapper.NewClusterClient(wrapper.StaticIpCluster{
			ServiceName: serviceName,
			Port:        servicePort,
		}), nil
	case "dns":
		domain := json.Get("domain").String()
		return wrapper.NewClusterClient(wrapper.DnsCluster{
			ServiceName: serviceName,
			Port:        servicePort,
			Domain:      domain,
		}), nil
	case "fqdn":
		return wrapper.NewClusterClient(wrapper.FQDNCluster{
			FQDN: serviceName,
			Port: servicePort,
		}), nil
	default:
		return wrapper.NewClusterClient(wrapper.TargetCluster{
			Host:    serviceName,
			Cluster: serviceName,
		}), nil
	}
}

// ... existing code ...
func ParseConfig(json gjson.Result, config *SimpleConfig) error {
	// Parse token config
	tokenConfig := json.Get("token_config")
	if tokenConfig.Exists() {
		config.TokenConfig.Enabled = tokenConfig.Get("enabled").Bool()
		config.TokenConfig.TokenPath = tokenConfig.Get("token_path").String()
		config.TokenConfig.Timeout = uint32(tokenConfig.Get("timeout").Uint())

		// Parse credential
		credential := tokenConfig.Get("credential")
		if credential.Exists() {
			// Parse form fields
			formFields := credential.Get("form_fields")
			if formFields.Exists() {
				config.TokenConfig.Credential.FormFields = make(map[string]string)
				formFields.ForEach(func(key, value gjson.Result) bool {
					config.TokenConfig.Credential.FormFields[key.String()] = value.String()
					return true
				})
			}

			// Parse head fields
			headFields := credential.Get("head_fields")
			if headFields.Exists() {
				config.TokenConfig.Credential.HeadFields = make(map[string]string)
				headFields.ForEach(func(key, value gjson.Result) bool {
					config.TokenConfig.Credential.HeadFields[key.String()] = value.String()
					return true
				})
			}
		}

		// Parse token extraction
		tokenExtraction := tokenConfig.Get("token_extraction")
		if tokenExtraction.Exists() {
			config.TokenConfig.TokenExtraction.ResponsePath = tokenExtraction.Get("response_path").String()
		}

		// Parse token injection
		tokenInjection := tokenConfig.Get("token_injection")
		if tokenInjection.Exists() && tokenInjection.IsArray() {
			config.TokenConfig.TokenInjection = make([]TokenInjection, 0)
			tokenInjection.ForEach(func(_, value gjson.Result) bool {
				injection := TokenInjection{
					Type:   value.Get("type").String(),
					Key:    value.Get("key").String(),
					Format: value.Get("format").String(),
				}
				config.TokenConfig.TokenInjection = append(config.TokenConfig.TokenInjection, injection)
				return true
			})
		}
	}

	invalidTokenCondition := tokenConfig.Get("invalid_token_condition")
	if invalidTokenCondition.Exists() {
		config.TokenConfig.InvalidTokenCondition = invalidTokenCondition.String()
	}

	retry_send_times := tokenConfig.Get("retry_send_times")
	if retry_send_times.Exists() {
		config.TokenConfig.RetrySendTimes = int(tokenConfig.Get("retry_send_times").Int())
	}
	// config.HttpService.Client = wrapper.NewClusterClient(wrapper.StaticIpCluster{
	// 	ServiceName: "token-service",
	// 	Host:        "10.27.11.211",
	// 	Port:        80,
	// })
	// Parse http service
	token_service := json.Get("token_service")
	if token_service.Exists() {
		// Create HTTP client
		endpoint := token_service.Get("endpoint")
		if endpoint.Exists() {
			config.TokenService.Client, _ = CreateClusterClient(endpoint)
			// serviceName := endpoint.Get("service_name").String()
			// servicePort := (endpoint.Get("service_port").Int())
			// serviceHost := endpoint.Get("service_host").String()
			// protocol := endpoint.Get("protocol").String()
			// config.TokenService.Client, _ = CreateClusterClient(endpoint, "")
			// config.TokenService.IP = serviceHost
			// config.TokenService.Port = int(servicePort)
			// config.TokenService.Service = serviceName
			// config.TokenService.Protocol = protocol

		}
	}

	// gw_service := json.Get("gateway_service")
	// if gw_service.Exists() {
	// 	// Create HTTP client
	// 	endpoint := gw_service.Get("endpoint")

	// 	if endpoint.Exists() {
	// 		config.GwService.Client, _ = CreateClusterClient(endpoint)

	// 	}
	// }
	return nil
}
