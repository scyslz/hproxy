package dns

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"hproxy/config"
)

// Dnsmasq 实现 DNSProvider 接口
type Dnsmasq struct {
	ConfFile   string
	OutputFile string
	lastDomains map[string]string
	lastModTime time.Time
}

// NewDnsmasq 创建 Dnsmasq 实例
func NewDnsmasq(config map[string]interface{}) (*Dnsmasq, error) {
	d := &Dnsmasq{
		lastDomains: make(map[string]string),
	}
	if v, ok := config["conf_file"]; ok {
		d.ConfFile = v.(string)
	}
	if v, ok := config["output_file"]; ok {
		d.OutputFile = v.(string)
	}
	return d, nil
}

func (d *Dnsmasq) Name() string {
	return "dnsmasq"
}

func (d *Dnsmasq) UpdateDomains(domains map[string]string) error {
	// 变更检测
	if d.compareDomains(domains) {
		config.DebugLog("[Dnsmasq] 域名无变化，跳过更新")
		return nil
	}

	if d.isConfigModified() {
		config.DebugLog("[Dnsmasq] 配置文件被外部修改，强制更新")
	}

	lines := make([]string, 0, len(domains))
	for domain, ip := range domains {
		lines = append(lines, fmt.Sprintf("address=/%s/%s", domain, ip))
	}

	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(d.OutputFile, []byte(content), 0644); err != nil {
		return err
	}

	d.lastDomains = make(map[string]string)
	for k, v := range domains {
		d.lastDomains[k] = v
	}
	if info, err := os.Stat(d.OutputFile); err == nil {
		d.lastModTime = info.ModTime()
	}

	config.DebugLog("[Dnsmasq] 已更新 %d 条域名到 %s", len(domains), d.OutputFile)
	return d.ensureConfig()
}

func (d *Dnsmasq) compareDomains(domains map[string]string) bool {
	if len(domains) != len(d.lastDomains) {
		return false
	}
	for domain, ip := range domains {
		lastIP, ok := d.lastDomains[domain]
		if !ok || lastIP != ip {
			return false
		}
	}
	return true
}

func (d *Dnsmasq) isConfigModified() bool {
	info, err := os.Stat(d.OutputFile)
	if err != nil {
		return true
	}
	return info.ModTime().After(d.lastModTime)
}

func (d *Dnsmasq) ensureConfig() error {
	if d.ConfFile == "" {
		return nil
	}
	data, err := os.ReadFile(d.ConfFile)
	if err != nil {
		return err
	}
	content := string(data)
	if strings.Contains(content, d.OutputFile) {
		return nil
	}
	f, err := os.OpenFile(d.ConfFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\nconf-file=" + d.OutputFile + "\n")
	return err
}

func (d *Dnsmasq) Reload() error {
	if d.ConfFile == "" {
		return nil
	}
	cmd := exec.Command("systemctl", "reload", "dnsmasq")
	cmd.Run()
	config.DebugLog("[Dnsmasq] 已重载")
	return nil
}
