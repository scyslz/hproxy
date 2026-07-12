package dns

import (
	"fmt"
	"strings"
)

// DNSProvider 接口
type DNSProvider interface {
	Name() string
	UpdateDomains(domains map[string]string) error
	Reload() error
}

// DomainEntry 域名记录
type DomainEntry struct {
	Domain string
	IP     string
}

// GenerateConfig 生成配置文件内容
func GenerateConfig(entries []DomainEntry, format, header string) string {
	var sb strings.Builder
	sb.WriteString(header)
	for _, e := range entries {
		line := strings.ReplaceAll(format, "{domain}", e.Domain)
		line = strings.ReplaceAll(line, "{ip}", e.IP)
		sb.WriteString(line + "\n")
	}
	return sb.String()
}

// NewProvider 根据配置创建 DNS provider
func NewProvider(cfg map[string]interface{}) (DNSProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("dns config is nil")
	}
	provider := "smartdns"
	if v, ok := cfg["provider"]; ok {
		provider = v.(string)
	}
	switch provider {
	case "smartdns":
		return NewSmartDNS(cfg)
	case "dnsmasq":
		return NewDnsmasq(cfg)
	default:
		return nil, fmt.Errorf("unsupported dns provider: %s", provider)
	}
}
