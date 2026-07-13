package dns

import (
	"fmt"
	"log"
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
	ReloadCmd  string // 重载命令（为空则跳过重载）
	lastDomains map[string]string
	lastModTime time.Time
}

// NewDnsmasq 创建 Dnsmasq 实例
func NewDnsmasq(cfg map[string]interface{}) (*Dnsmasq, error) {
	d := &Dnsmasq{
		lastDomains: make(map[string]string),
	}
	if v, ok := cfg["conf_file"]; ok {
		d.ConfFile = v.(string)
	}
	if v, ok := cfg["output_file"]; ok {
		d.OutputFile = v.(string)
	}
	if v, ok := cfg["reload_cmd"]; ok {
		d.ReloadCmd = v.(string)
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
		log.Printf("[Dnsmasq] 写文件失败 %s: %v", d.OutputFile, err)
		return err
	}

	d.lastDomains = make(map[string]string)
	for k, v := range domains {
		d.lastDomains[k] = v
	}
	if info, err := os.Stat(d.OutputFile); err == nil {
		d.lastModTime = info.ModTime()
	}

	log.Printf("[Dnsmasq] 已更新 %d 条域名到 %s", len(domains), d.OutputFile)
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
		log.Printf("[Dnsmasq] 读取主配置失败 %s: %v", d.ConfFile, err)
		return err
	}
	content := string(data)
	if strings.Contains(content, d.OutputFile) {
		return nil
	}
	f, err := os.OpenFile(d.ConfFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[Dnsmasq] 打开主配置失败 %s: %v", d.ConfFile, err)
		return err
	}
	defer f.Close()
	_, err = f.WriteString("\nconf-file=" + d.OutputFile + "\n")
	if err != nil {
		log.Printf("[Dnsmasq] 写入主配置失败 %s: %v", d.ConfFile, err)
		return err
	}
	return nil
}

func (d *Dnsmasq) Reload() error {
	if d.ReloadCmd == "" {
		config.DebugLog("[Dnsmasq] 未配置 reload_cmd，跳过重载")
		return nil
	}
	cmd := exec.Command("sh", "-c", d.ReloadCmd)
	if err := cmd.Run(); err != nil {
		config.DebugLog("[Dnsmasq] 重载失败（dnsmasq 可能未运行）: %v", err)
	}
	config.DebugLog("[Dnsmasq] 已重载")
	return nil
}
