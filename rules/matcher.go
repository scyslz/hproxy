package rules

import (
	"strings"

	"hproxy/config"
)

// MatchRule 匹配域名对应的规则
// 返回目标地址，空字符串表示走 DIRECT 模式
func MatchRule(host string, clientHTTPS bool, cfg *config.Config) string {
	host = strings.Split(host, ":")[0]
	host = strings.TrimSpace(host)

	// 用读锁保护对全局规则的访问
	rulesMutex.RLock()
	defer rulesMutex.RUnlock()

	// 1. 检查完整域名规则
	if rule, ok := Rules[host]; ok {
		target := ResolveTarget(rule, clientHTTPS)
		if target != "" {
			return target
		}
	}

	// 2. 检查通配符规则（最长匹配优先）
	var bestTarget string
	bestLen := 0
	for d, rule := range WildRules {
		// d 是通配符后缀，如 ".domain.com" 或 "aa.domain.com"
		// host 以 d 结尾就匹配
		if strings.HasSuffix(host, d) {
			if len(d) > bestLen {
				bestLen = len(d)
				bestTarget = ResolveTarget(rule, clientHTTPS)
			}
		}
	}
	if bestTarget != "" {
		return bestTarget
	}

	// 3. 没有匹配规则，走 DIRECT 模式
	return ""
}

// ResolveTarget 根据规则和目标协议解析实际目标地址
func ResolveTarget(rule config.Rule, clientHTTPS bool) string {
	if len(rule.Target) == 0 {
		return ""
	}

	if rule.Proto == "" || rule.Proto == "strict" {
		for _, t := range rule.Target {
			if matchProto(t, clientHTTPS) {
				return t
			}
		}
		return ""
	}

	if rule.Proto == "prefer-https" {
		for _, t := range rule.Target {
			if hasScheme(t) && strings.HasPrefix(t, "https://") {
				return t
			}
		}
		return fallbackMatch(rule.Target, clientHTTPS)
	}

	if rule.Proto == "prefer-http" {
		for _, t := range rule.Target {
			if hasScheme(t) && strings.HasPrefix(t, "http://") {
				return t
			}
		}
		return fallbackMatch(rule.Target, clientHTTPS)
	}

	if rule.Proto == "force-https" {
		for _, t := range rule.Target {
			if hasScheme(t) && strings.HasPrefix(t, "https://") {
				return t
			}
		}
		for _, t := range rule.Target {
			if !hasScheme(t) {
				return "https://" + t
			}
		}
		return ""
	}

	if rule.Proto == "force-http" {
		for _, t := range rule.Target {
			if hasScheme(t) && strings.HasPrefix(t, "http://") {
				return t
			}
		}
		for _, t := range rule.Target {
			if !hasScheme(t) {
				return "http://" + t
			}
		}
		return ""
	}

	return fallbackMatch(rule.Target, clientHTTPS)
}

func hasScheme(target string) bool {
	return strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://")
}

func matchProto(target string, clientHTTPS bool) bool {
	if hasScheme(target) {
		return (clientHTTPS && strings.HasPrefix(target, "https://")) ||
			(!clientHTTPS && strings.HasPrefix(target, "http://"))
	}
	return false
}

func fallbackMatch(targets []string, clientHTTPS bool) string {
	for _, t := range targets {
		if matchProto(t, clientHTTPS) {
			return t
		}
	}
	if len(targets) > 0 {
		return targets[0]
	}
	return ""
}
