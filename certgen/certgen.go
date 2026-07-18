package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	caCert   *x509.Certificate
	caKey    *ecdsa.PrivateKey
	certMu   sync.RWMutex
	certCache = make(map[string]*tls.Certificate)
	caOnce   sync.Once
)

// getCADir 返回 CA 存储目录
func getCADir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/hproxy/ca"
	}
	return filepath.Join(home, ".hproxy", "ca")
}

// loadOrCreateCA 加载或创建 CA 证书
func loadOrCreateCA() error {
	var err error
	caDir := getCADir()
	os.MkdirAll(caDir, 0700)

	certPath := filepath.Join(caDir, "ca.crt")
	keyPath := filepath.Join(caDir, "ca.key")

	// 尝试加载已有 CA
	if caData, err := os.ReadFile(certPath); err == nil {
		if keyData, err := os.ReadFile(keyPath); err == nil {
			if caKey, err = parseECKey(keyData); err == nil {
				if caCert, err = parseCert(caData); err == nil {
					log.Printf("[CertGen] 使用已有 CA 证书: %s", certPath)
					return nil
				}
			}
		}
	}

	// 创建新 CA
	log.Printf("[CertGen] 生成新的 CA 证书...")
	caKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("生成 CA 密钥失败: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	caTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "hproxy CA",
			Organization: []string{"hproxy"},
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour), // 10年
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	caCertBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("创建 CA 证书失败: %w", err)
	}

	caCert, err = x509.ParseCertificate(caCertBytes)
	if err != nil {
		return fmt.Errorf("解析 CA 证书失败: %w", err)
	}

	// 保存
	caKeyBytes, _ := x509.MarshalECPrivateKey(caKey)
	os.WriteFile(certPath, caCertBytes, 0644)
	os.WriteFile(keyPath, caKeyBytes, 0600)

	log.Printf("[CertGen] CA 证书已生成: %s", certPath)
	log.Printf("[CertGen] 请在浏览器中信任此 CA 证书以实现无警告访问")
	return nil
}

func parseECKey(data []byte) (*ecdsa.PrivateKey, error) {
	return x509.ParseECPrivateKey(data)
}

func parseCert(data []byte) (*x509.Certificate, error) {
	return x509.ParseCertificate(data)
}

// GetCertificate 根据 SNI 返回匹配的证书
// 优先使用已配置的证书，没有则动态生成
func GetCertificate(cfgCerts []tls.Certificate, hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	sni := hello.ServerName
	if sni == "" {
		// 没有 SNI，返回第一个配置的证书
		if len(cfgCerts) > 0 {
			return &cfgCerts[0], nil
		}
		return nil, fmt.Errorf("没有可用证书 (SNI 为空)")
	}

	// 检查已配置证书是否匹配 SNI
	for i, cert := range cfgCerts {
		if matchCertificate(cert, sni) {
			return &cfgCerts[i], nil
		}
	}

	log.Printf("[CertGen] 没有为 %s 配置证书，生成临时证书", sni)

	// 尝试从缓存获取
	sni = strings.ToLower(sni)
	certMu.RLock()
	if cached, ok := certCache[sni]; ok {
		certMu.RUnlock()
		return cached, nil
	}
	certMu.RUnlock()

	// 生成临时证书
	cert, err := generateCert(sni)
	if err != nil {
		log.Printf("[CertGen] 生成 %s 的临时证书失败: %v", sni, err)
		// 回退到第一个证书
		if len(cfgCerts) > 0 {
			return &cfgCerts[0], nil
		}
		return nil, err
	}

	certMu.Lock()
	certCache[sni] = cert
	certMu.Unlock()

	log.Printf("[CertGen] ✅ 已生成 %s 的临时证书", sni)
	return cert, nil
}

// matchCertificate 检查证书是否匹配给定的 SNI
func matchCertificate(cert tls.Certificate, sni string) bool {
	if len(cert.Certificate) == 0 {
		return false
	}
	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return false
	}
	// 检查 SAN
	for _, name := range x509Cert.DNSNames {
		if matchDomain(name, sni) {
			return true
		}
	}
	// 检查 CN
	if matchDomain(x509Cert.Subject.CommonName, sni) {
		return true
	}
	return false
}

// matchDomain 域名匹配（支持通配符）
func matchDomain(pattern, domain string) bool {
	if pattern == domain {
		return true
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := pattern[1:] // 去掉 *
		return strings.HasSuffix(domain, suffix)
	}
	return false
}

// generateCert 为指定域名生成临时证书
func generateCert(domain string) (*tls.Certificate, error) {
	caOnce.Do(func() {
		err := loadOrCreateCA()
		if err != nil {
			log.Printf("[CertGen] CA 初始化失败: %v", err)
		}
	})

	if caCert == nil || caKey == nil {
		return nil, fmt.Errorf("CA 未初始化")
	}

	// 生成域名密钥
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %w", err)
	}

	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: domain,
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(90 * 24 * time.Hour), // 90天
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames: []string{domain},
	}

	// 如果是 IP 地址，添加到 IP 列表
	if ip := net.ParseIP(domain); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, template, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("创建证书失败: %w", err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certBytes, caCert.Raw},
		PrivateKey:  key,
	}

	return tlsCert, nil
}

// CACertPath 返回 CA 证书路径（供用户安装信任）
func CACertPath() string {
	return filepath.Join(getCADir(), "ca.crt")
}
