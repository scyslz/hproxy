package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
	config.SetDebug(cfg.Debug)
	dnsProvider, _ := initDNSProvider(cfg)

	// 启动定时更新
	if dnsProvider != nil {
		rules.StartScheduler(cfg, dnsProvider)
	}

	// 启动管理接口（单独端口）
	if cfg.AdminPort != "" {
		go func() {
			adminHandler := admin.Handler(cfg)
			log.Printf("[Admin] 管理接口监听 http://%s:%s", cfg.LanIP, cfg.AdminPort)
			if err := http.ListenAndServe(cfg.LanIP+":"+cfg.AdminPort, adminHandler); err != nil {
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

// loadAllCerts 加载所有证书
func loadAllCerts(cfg *config.Config) ([]tls.Certificate, error) {
	var certs []tls.Certificate
	
	certConfigs := cfg.ListCerts()
	if len(certConfigs) == 0 {
		return nil, fmt.Errorf("未配置任何证书")
	}
	
	for i, certCfg := range certConfigs {
		tlsCert, err := tls.LoadX509KeyPair(certCfg.Cert, certCfg.Key)
		if err != nil {
			log.Printf("[Cert] 加载证书 %s[%d] 失败: %v", certCfg.Name, i, err)
			continue
		}
		certs = append(certs, tlsCert)
	}
	
	if len(certs) == 0 {
		return nil, fmt.Errorf("未能加载任何证书")
	}
	
	return certs, nil
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
	// 验证所有证书文件存在
	for i, cert := range cfg.Certs {
		if _, err := os.Stat(cert.Cert); err != nil {
			return fmt.Errorf("证书 %s[%d] 不存在: %s", cert.Name, i, cert.Cert)
		}
		if _, err := os.Stat(cert.Key); err != nil {
			return fmt.Errorf("密钥 %s[%d] 不存在: %s", cert.Name, i, cert.Key)
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
		log.SetFlags(log.LstdFlags)
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

	// 包装 handler，注入 server_addr 到 context
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), "server_addr", realAddr))
		handler(w, r)
	})

	switch scheme {
	case "http":
		go func() {
			log.Printf("[Server] HTTP %s", realAddr)
			if err := http.ListenAndServe(realAddr, wrappedHandler); err != nil {
				log.Printf("[Server] %s 监听失败: %v", realAddr, err)
			}
		}()
	case "https":
		if len(cfg.Certs) == 0 {
			log.Printf("[Server] %s HTTPS 需要证书", realAddr)
			return
		}
		
		go func() {
			// 加载所有证书
			tlsCerts, err := loadAllCerts(cfg)
			if err != nil {
				log.Printf("[Server] %s TLS 证书加载失败: %v", realAddr, err)
				return
			}
			
			// 创建 TCP 监听器
			ln, err := net.Listen("tcp", realAddr)
			if err != nil {
				log.Printf("[Server] %s 监听失败: %v", realAddr, err)
				return
			}
			defer ln.Close()
			
			// 创建 TLS 配置
			tlsConfig := &tls.Config{
				Certificates: tlsCerts,
			}
			
			// 创建 TLS 监听器
			tlsListener := tls.NewListener(ln, tlsConfig)
			defer tlsListener.Close()
			
			log.Printf("[Server] HTTPS %s (多证书支持, %d个证书)", realAddr, len(tlsCerts))
			
			// 启动 HTTP 服务器
			server := &http.Server{
				Handler: wrappedHandler,
			}
			if err := server.Serve(tlsListener); err != nil {
				log.Printf("[Server] %s 服务失败: %v", realAddr, err)
			}
		}()
	case "auto":
		// 同一个端口同时处理 HTTP 和 HTTPS
		if len(cfg.Certs) == 0 {
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
			
			// 加载所有证书
			tlsCerts, err := loadAllCerts(cfg)
			if err != nil {
				log.Printf("[Server] %s 加载证书失败: %v", realAddr, err)
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
					go handleConnection(conn, handler, tlsCerts, realAddr)
			}
		}()
	}
}

// handleConnection 处理连接，根据第一个字节判断是 HTTP 还是 HTTPS
func handleConnection(conn net.Conn, handler http.HandlerFunc, tlsCerts []tls.Certificate, serverAddr string) {
	defer conn.Close()

	// 读取前几个字节用于判断协议
	peek := make([]byte, 5)

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, err := io.ReadAtLeast(conn, peek, 1)
	if err != nil {
		return
	}

	// 清除探测超时
	conn.SetReadDeadline(time.Time{})

	// 把已经读取的数据放回读取流
	bufferedConn := &readConn{
		Conn: conn,
		reader: io.MultiReader(
			bytes.NewReader(peek[:n]),
			conn,
		),
	}

	var serveConn net.Conn = bufferedConn

	// 判断 TLS
	if isTLS(peek[:n]) {
		serveConn = tls.Server(bufferedConn, &tls.Config{
			Certificates: tlsCerts,
		})
	}

	// 包装 handler，注入 server_addr
	wrappedHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), "server_addr", serverAddr))
		handler(w, r)
	})

	server := &http.Server{
		Handler: wrappedHandler,
	}

	server.Serve(&singleConnListener{
		conn: serveConn,
	})
}
func isTLS(buf []byte) bool {
	return len(buf) >= 3 &&
		buf[0] == 0x16 &&
		buf[1] == 0x03 &&
		buf[2] >= 0x01 &&
		buf[2] <= 0x04
}
// singleConnListener 包装单个连接为 Listener
type singleConnListener struct {
	conn net.Conn
	once sync.Once
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	var c net.Conn
	l.once.Do(func() {
		c = l.conn
	})
	if c == nil {
		// 连接已返回过一次，阻塞等待（server 会在处理完后关闭连接）
		select {}
	}
	return c, nil
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
	reader io.Reader
}

func (c *readConn) Read(p []byte) (int, error) {
	return c.reader.Read(p)
}
