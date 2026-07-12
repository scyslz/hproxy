package rules

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"hproxy/config"
)

// RuleProvider 规则提供者接口
// 每种规则来源实现自己的解析逻辑
type RuleProvider interface {
	Name() string
	Load() (map[string]config.Rule, bool, error) // 返回解析后的规则, changed, error
}

// RuleChain 规则处理链
type RuleChain struct {
	providers []RuleProvider
}

// NewRuleChain 创建规则链
func NewRuleChain(cfg *config.Config) *RuleChain {
	chain := &RuleChain{}
	for _, src := range cfg.RuleSources {
		if !src.Enabled {
			continue
		}
		switch src.Type {
		case "lucky_api":
			chain.Add(&LuckyAPISource{
				URL:       src.URL,
				Target:    src.Target,
				Proto:     src.Proto,
				Filter:    src.Filter,
				CacheFile: src.CacheFile,
			})
		case "local_file":
			chain.Add(&LocalFileSource{
				Path:   src.Path,
				Format: src.Format,
			})
		}
	}
	return chain
}

// Add 添加规则提供者
func (c *RuleChain) Add(provider RuleProvider) {
	c.providers = append(c.providers, provider)
}

// LoadAll 加载所有规则提供者，更新全局 map
func (c *RuleChain) LoadAll() bool {
	changed := false

	// 清空旧规则
	Rules = make(map[string]config.Rule)
	WildRules = make(map[string]config.Rule)

	for _, provider := range c.providers {
		rules, c, err := provider.Load()
		if err != nil {
			log.Printf("[RuleChain] 来源 %s 加载失败: %v", provider.Name(), err)
			continue
		}
		if c {
			changed = true
		}
		// 合并规则（自动处理通配符）
		mergeRules(rules, provider.Name())
	}

	log.Printf("[RuleChain] 加载完成: 精确=%d, 通配=%d",
		len(Rules), len(WildRules))
	return changed
}

// mergeRules 将规则合并到全局 map，自动处理通配符
// 支持格式: *.domain.com 或 *aa.domain.com
func mergeRules(rules map[string]config.Rule, sourceName string) {
	for host, rule := range rules {
		if strings.HasPrefix(host, "*") {
			// 通配符规则：去掉开头的 *，保留后面的部分作为匹配后缀
			wildcard := strings.TrimPrefix(host, "*")
			rule.Source = sourceName
			WildRules[wildcard] = rule
			log.Printf("[RuleChain] 通配符规则: %s (来源: %s)", wildcard, sourceName)
		} else {
			// 普通规则
			rule.Source = sourceName
			Rules[host] = rule
		}
	}
}

// --- LuckyAPISource ---

type LuckyAPISource struct {
	URL       string
	Target    string
	Proto     string
	Filter    *config.APIFilter
	CacheFile string
	lastRules map[string]config.Rule
}

func (s *LuckyAPISource) Name() string {
	return "lucky_api"
}

func (s *LuckyAPISource) Load() (map[string]config.Rule, bool, error) {
	rules, err := fetchDomains(s.URL, s.Target, s.Proto, s.Filter)
	if err != nil {
		log.Printf("[LuckyAPI] 获取失败: %v，尝试缓存", err)
		return s.loadCache()
	}

	if s.CacheFile != "" {
		if data, err := json.MarshalIndent(rules, "", "  "); err == nil {
			os.WriteFile(s.CacheFile, data, 0644)
		}
	}

	changed := s.compareRules(rules)
	s.lastRules = rules
	return rules, changed, nil
}

func (s *LuckyAPISource) loadCache() (map[string]config.Rule, bool, error) {
	if s.CacheFile == "" {
		return nil, false, fmt.Errorf("无缓存文件")
	}
	data, err := os.ReadFile(s.CacheFile)
	if err != nil {
		return nil, false, err
	}
	var rules map[string]config.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, false, err
	}
	changed := s.compareRules(rules)
	s.lastRules = rules
	return rules, changed, nil
}

func (s *LuckyAPISource) compareRules(newRules map[string]config.Rule) bool {
	if len(newRules) != len(s.lastRules) {
		return true
	}
	for host, newRule := range newRules {
		oldRule, ok := s.lastRules[host]
		if !ok || !ruleEqual(oldRule, newRule) {
			return true
		}
	}
	return false
}

// --- LocalFileSource ---

type LocalFileSource struct {
	Path       string
	Format     string
	lastModTime time.Time
	lastRules   map[string]config.Rule
}

func (s *LocalFileSource) Name() string {
	return "local_file:" + s.Path
}

func (s *LocalFileSource) Load() (map[string]config.Rule, bool, error) {
	info, err := os.Stat(s.Path)
	if err != nil {
		return nil, false, err
	}
	if !info.ModTime().After(s.lastModTime) && s.lastRules != nil {
		return s.lastRules, false, nil
	}
	s.lastModTime = time.Now()

	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, false, err
	}

	var rules []config.Rule
	if err := json.Unmarshal(data, &rules); err != nil {
		return nil, false, err
	}

	result := make(map[string]config.Rule)
	for _, r := range rules {
		result[r.Host] = r
	}

	changed := s.compareRules(result)
	s.lastRules = result
	return result, changed, nil
}

func (s *LocalFileSource) compareRules(newRules map[string]config.Rule) bool {
	if len(newRules) != len(s.lastRules) {
		return true
	}
	for host, newRule := range newRules {
		oldRule, ok := s.lastRules[host]
		if !ok || !ruleEqual(oldRule, newRule) {
			return true
		}
	}
	return false
}

// --- 全局变量 ---

var (
	Rules     map[string]config.Rule  // 精确域名规则
	WildRules map[string]config.Rule  // 通配符规则
)

func init() {
	Rules = make(map[string]config.Rule)
	WildRules = make(map[string]config.Rule)
}

// UpdateRules 更新所有规则（兼容旧调用）
func UpdateRules(cfg *config.Config) {
	chain := NewRuleChain(cfg)
	chain.LoadAll()
}

// LoadAll 导出函数：加载所有规则
func LoadAll() {
	// 这里需要访问 DefaultChain，但 DefaultChain 没定义
	// 暂时用 UpdateRules 代替
}

// fetchDomains 从 Lucky API 获取域名
func fetchDomains(apiURL, target, proto string, filter *config.APIFilter) (map[string]config.Rule, error) {
	result := make(map[string]config.Rule)

	if apiURL == "" {
		return result, fmt.Errorf("LuckyAPI not configured")
	}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return result, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second, Transport: GlobalTransport}
	resp, err := client.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return result, fmt.Errorf("API 返回状态码 %d", resp.StatusCode)
	}

	var data struct {
		Ret      int `json:"ret"`
		RuleList []struct {
			RuleName   string `json:"RuleName"`
			ListenPort int    `json:"ListenPort"`
			ProxyList  []struct {
				Domains []string `json:"Domains"`
			} `json:"ProxyList"`
		} `json:"ruleList"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return result, err
	}

	ruleName := "Lucky_https"
	listenPort := 2053
	domainContains := "46."
	if filter != nil {
		if filter.RuleName != "" {
			ruleName = filter.RuleName
		}
		if filter.ListenPort != 0 {
			listenPort = filter.ListenPort
		}
		if filter.DomainContains != "" {
			domainContains = filter.DomainContains
		}
	}

	for _, rule := range data.RuleList {
		if rule.RuleName != ruleName || rule.ListenPort != listenPort {
			continue
		}
		for _, proxy := range rule.ProxyList {
			for _, domain := range proxy.Domains {
				domain = strings.Split(domain, "/")[0]
				if strings.Contains(domain, domainContains) {
					result[domain] = config.Rule{
						Host:   domain,
						Target: []string{target},
						Proto:  proto,
					}
				}
			}
		}
	}

	log.Printf("[Lucky API] 获取到 %d 个域名", len(result))
	return result, nil
}

func ruleEqual(a, b config.Rule) bool {
	if a.Host != b.Host || a.Proto != b.Proto {
		return false
	}
	if len(a.Target) != len(b.Target) {
		return false
	}
	for i := range a.Target {
		if a.Target[i] != b.Target[i] {
			return false
		}
	}
	return true
}

// 全局复用的 Transport 和 Resolver
var (
	GlobalTransport = &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:          50,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       30 * time.Second,
		DisableKeepAlives:     false,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	GlobalResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "udp", "223.5.5.5:53")
		},
	}
)
