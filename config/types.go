package config

// Rule 定义一条域名规则
type Rule struct {
	Host   string   `json:"host"`
	Target []string `json:"target"` // 支持多个目标，可带协议头
	Proto  string   `json:"proto"`  // strict|prefer-https|prefer-http|force-https|force-http|direct
	Source string   `json:"source,omitempty"` // 来源：lucky_api/local_file/wild
}

// RuleSource 规则来源配置
type RuleSource struct {
	Type    string      `json:"type"`    // lucky_api | local_file
	URL     string      `json:"url,omitempty"`
	Target  string      `json:"target,omitempty"`
	Proto   string      `json:"proto,omitempty"`
	Filter  *APIFilter  `json:"filter,omitempty"`
	Path    string      `json:"path,omitempty"`
	Format  string      `json:"format,omitempty"`
	Enabled bool        `json:"enabled"`
	CacheFile string    `json:"cache_file,omitempty"` // 缓存文件路径
}

// APIFilter Lucky API 过滤条件
type APIFilter struct {
	RuleName   string `json:"rule_name,omitempty"`
	ListenPort int    `json:"listen_port,omitempty"`
	DomainContains string `json:"domain_contains,omitempty"`
}

// DNSProviderConfig DNS 提供商配置
type DNSProviderConfig struct {
	Provider string                 `json:"provider"`
	Config   map[string]interface{} `json:"config"`
}

// Config 主配置
type Config struct {
	ConfigFile   string              `json:"-"`
	LanIP        string              `json:"lan_ip,omitempty"`     // 内网 IP，如 192.168.100.1
	AdminPort    string              `json:"admin_port,omitempty"`   // 管理接口端口
	LogFile      string              `json:"log_file,omitempty"`     // 日志文件路径
	Cert         string              `json:"cert,omitempty"`
	Key          string              `json:"key,omitempty"`
	ProxyServers []string            `json:"proxy_servers,omitempty"` // 代理监听地址
	Interval     string              `json:"interval,omitempty"`     // 定时更新间隔
	DNS          *DNSProviderConfig `json:"dns,omitempty"`
	RuleSources  []RuleSource        `json:"rule_sources,omitempty"`
}
