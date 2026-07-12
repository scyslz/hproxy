package proxy

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"hproxy/config"
	"hproxy/rules"
)

// ProxyHandler 只处理代理逻辑
func ProxyHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从 context 取 server 地址（由 startServer 注入）
		serverAddr := "unknown"
		if addr, ok := r.Context().Value("server_addr").(string); ok {
			serverAddr = addr
		}

		host := strings.Split(r.Host, ":")[0]
		host = strings.TrimSpace(host)
		target := rules.MatchRule(host, r.TLS != nil, cfg)

		if target == "" {
			// DIRECT 模式：查真实 IP
			log.Printf("[Proxy] [%s] DIRECT %s %s (TLS=%v)", serverAddr, r.Method, r.URL.Path, r.TLS != nil)

			hostOnly := strings.Split(host, ":")[0]
			isLocal := false
			if hostOnly == "127.0.0.1" || hostOnly == "::1" || hostOnly == "localhost" ||
				strings.HasPrefix(hostOnly, cfg.LanIP) ||
				strings.HasPrefix(hostOnly, "10.0.") ||
				strings.HasPrefix(hostOnly, "172.17.") {
				isLocal = true
			}

			if isLocal {
				w.Header().Set("Connection", "close")
				http.Error(w, "503 Loop detected (local address)", http.StatusServiceUnavailable)
				log.Printf("[Proxy] [%s] ❌ 检测到本地地址: %s %s %s", serverAddr, r.Method, r.URL.Path, host)
				return
			}

			addrs, err := rules.GlobalResolver.LookupHost(r.Context(), host)
			if err != nil || len(addrs) == 0 {
				http.Error(w, "503 DNS lookup failed", http.StatusServiceUnavailable)
				log.Printf("[Proxy] ❌ DNS 查询失败: %s → %v", host, err)
				return
			}

			realIP := ""
			for _, addr := range addrs {
				if !strings.Contains(addr, ":") {
					realIP = addr
					break
				}
			}
			if realIP == "" {
				realIP = addrs[0]
			}

			// 从原始请求取端口
			_, port, _ := net.SplitHostPort(r.Host)
			if port == "" {
				if r.TLS != nil {
					port = "443"
				} else {
					port = "80"
				}
			}
			if strings.Contains(realIP, ":") {
				realIP = "[" + realIP + "]"
			}
			target = fmt.Sprintf("http://%s:%s", realIP, port)
			if r.TLS != nil {
				target = fmt.Sprintf("https://%s:%s", realIP, port)
			}
			log.Printf("[Proxy] [%s] ✅ DIRECT %s %s %s → %s", serverAddr, r.Method, r.URL.Path, host, target)
			} else {
			log.Printf("[Proxy] [%s] ✅ 规则匹配 %s %s %s → %s (客户端%s)", serverAddr, r.Method, r.URL.Path, host, target, map[bool]string{true: "HTTPS", false: "HTTP"}[r.TLS != nil])
		}

		proxy := &httputil.ReverseProxy{
			Director: func(req *http.Request) {
				u, _ := url.Parse(target)
				req.URL.Scheme = u.Scheme
				req.URL.Host = u.Host
				req.Host = r.Host
				req.Header.Set("Host", r.Host)
				req.Header.Set("X-Forwarded-For", r.RemoteAddr)
			},
			Transport: rules.GlobalTransport,
		}
		proxy.ServeHTTP(w, r)
	}
}
