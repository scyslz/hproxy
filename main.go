package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"hproxy/admin"
	"hproxy/config"
	"hproxy/dns"
	"hproxy/proxy"
	"hproxy/rules"
)

func main() {
	setFDLimit()

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("[Config] 加载失败: %v", err)
	}

	if err := loadCert(cfg); err != nil {
		log.Printf("[Cert] 警告: %v", err)
	}
	setupLog(cfg)
	dnsProvider, _ := initDNSProvider(cfg)

	// 初始更新 DNS
	if dnsProvider != nil {
		domains := make(map[string]string)
		ip := cfg.LanIP
		if ip == "" {
			ip = "192.168.100.1"  // 默认值
		}
		for d := range rules.Rules {
			domains[d] = ip
		}
		if err := dnsProvider.UpdateDomains(domains); err != nil {
			log.Printf("[DNS] 初始更新失败: %v", err)
		}
		dnsProvider.Reload()
	}

	// 启动定时更新
	if dnsProvider != nil {
		rules.StartScheduler(cfg, dnsProvider)
	}

	// 启动管理接口（单独端口）
	if cfg.AdminPort != "" {
		go func() {
			adminHandler := admin.Handler(cfg)
			log.Printf("[Admin] 管理接口监听 :%s", cfg.AdminPort)
			if err := http.ListenAndServe(":"+cfg.AdminPort, adminHandler); err != nil {
				log.Printf("[Admin] 监听失败: %v", err)
			}
		}()
	}

	// 启动代理服务（根据 proxy_servers 配置）
	handler := proxy.ProxyHandler(cfg)
	if len(cfg.ProxyServers) > 0 {
		for _, addr := range cfg.ProxyServers {
			startServer(addr, handler, cfg)
		}
	}

	log.Printf("=== 全部服务已启动 ===")
	select {}
}

func setFDLimit() {
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
		rlimit.Cur = rlimit.Max
		syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit)
	}
}

func loadConfig() (*config.Config, error) {
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	return config.Load(configPath)
}

func loadCert(cfg *config.Config) error {
	if cfg.Cert != "" && cfg.Key != "" {
		if _, err := os.Stat(cfg.Cert); err != nil {
			return fmt.Errorf("证书文件不存在: %s", cfg.Cert)
		}
		if _, err := os.Stat(cfg.Key); err != nil {
			return fmt.Errorf("密钥文件不存在: %s", cfg.Key)
		}
	}
	return nil
}

func setupLog(cfg *config.Config) {
	// 日志按天归档到 log_file 同目录下的 logs/ 目录
	logDir := "logs"
	if cfg.LogFile != "" {
		logDir = filepath.Dir(cfg.LogFile) + "/logs"
	}
	os.MkdirAll(logDir, 0755)
	
	logFile := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err == nil {
		log.SetOutput(f)
		log.SetFlags(log.LstdFlags | log.Lmicroseconds)
		log.Printf("[Main] 日志文件: %s", logFile)
	} else {
		log.Printf("[Main] 无法打开日志文件 %s: %v", logFile, err)
	}
}

func initDNSProvider(cfg *config.Config) (dns.DNSProvider, error) {
	if cfg.DNS == nil {
		return nil, fmt.Errorf("未配置 DNS")
	}
	return dns.NewProvider(cfg.DNS.Config)
}

// parseAddr 解析地址，返回协议和地址
func parseAddr(addr string) (string, string) {
	if strings.HasPrefix(addr, "http://") {
		return "http", strings.TrimPrefix(addr, "http://")
	}
	if strings.HasPrefix(addr, "https://") {
		return "https", strings.TrimPrefix(addr, "https://")
	}
	return "auto", addr
}

// startServer 启动服务器
func startServer(addr string, handler http.HandlerFunc, cfg *config.Config) {
	scheme, realAddr := parseAddr(addr)

	// 获取证书
	cert := cfg.Cert
	key := cfg.Key

	switch scheme {
	case "http":
		go func() {
			log.Printf("[Server] HTTP %s", realAddr)
			if err := http.ListenAndServe(realAddr, handler); err != nil {
				log.Printf("[Server] %s 监听失败: %v", realAddr, err)
			}
		}()
	case "https":
		if cert == "" || key == "" {
			log.Printf("[Server] %s HTTPS 需要证书", realAddr)
			return
		}
		go func() {
			log.Printf("[Server] HTTPS %s", realAddr)
			if err := http.ListenAndServeTLS(realAddr, cert, key, handler); err != nil {
				log.Printf("[Server] %s 监听失败: %v", realAddr, err)
			}
		}()
	case "auto":
		// 同一个端口同时处理 HTTP 和 HTTPS
		if cert == "" || key == "" {
			log.Printf("[Server] %s auto 需要证书，只启动 HTTP", realAddr)
			go func() {
				log.Printf("[Server] HTTP %s", realAddr)
				if err := http.ListenAndServe(realAddr, handler); err != nil {
					log.Printf("[Server] %s 监听失败: %v", realAddr, err)
				}
			}()
			return
		}
		
		go func() {
			log.Printf("[Server] HTTP+HTTPS %s", realAddr)
			// 监听端口
			ln, err := net.Listen("tcp", realAddr)
			if err != nil {
				log.Printf("[Server] %s 监听失败: %v", realAddr, err)
				return
			}
			defer ln.Close()
			
			// 加载证书
			tlsCert, err := tls.LoadX509KeyPair(cert, key)
			if err != nil {
				log.Printf("[Server] TLS 证书加载失败: %v", err)
				return
			}
			
			// 接受连接
			for {
				conn, err := ln.Accept()
				if err != nil {
					log.Printf("[Server] %s 接受连接失败: %v", realAddr, err)
					continue
				}
				
				// 判断是 HTTP 还是 HTTPS
				go handleConnection(conn, handler, &tlsCert)
			}
		}()
	}
}

// handleConnection 处理连接，根据第一个字节判断是 HTTP 还是 HTTPS
func handleConnection(conn net.Conn, handler http.HandlerFunc, tlsCert *tls.Certificate) {
	defer conn.Close()
	
	// 读取第一个字节（不消耗数据）
	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	n, err := conn.Read(buf)
	if err != nil || n == 0 {
		return
	}
	
	// 判断是否是 TLS（第一个字节是 0x16）
	if buf[0] == 0x16 {
		// HTTPS：TLS 握手
		tlsConn := tls.Server(&readConn{Conn: conn, firstByte: buf[0]}, &tls.Config{
			Certificates: []tls.Certificate{*tlsCert},
		})
		defer tlsConn.Close()
		
		// 处理 HTTPS 请求
		server := &http.Server{Handler: handler}
		server.Serve(&singleConnListener{conn: tlsConn})
	} else {
		// HTTP：直接处理
		// 把第一个字节写回去
		conn.Write(buf)
		server := &http.Server{Handler: handler}
		server.Serve(&singleConnListener{conn: conn})
	}
}

// singleConnListener 包装单个连接为 Listener
type singleConnListener struct {
	conn net.Conn
	done bool
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.done {
		return nil, io.EOF
	}
	l.done = true
	return l.conn, nil
}

func (l *singleConnListener) Close() error {
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

// readConn 包装 net.Conn，支持预读第一个字节
type readConn struct {
	net.Conn
	firstByte byte
	read      bool
}

func (c *readConn) Read(b []byte) (int, error) {
	if !c.read {
		b[0] = c.firstByte
		c.read = true
		return 1, nil
	}
	return c.Conn.Read(b)
}
