package rules

import (
	"log"
	"time"

	"hproxy/config"
	"hproxy/dns"
)

// Scheduler 管理定时任务
type Scheduler struct {
	cfg         *config.Config
	dnsProvider dns.DNSProvider
	chain       *RuleChain
	interval    time.Duration
	stop        chan struct{}
}

// StartScheduler 启动定时更新规则
// dnsProvider 可选，如果提供则会自动更新 DNS
func StartScheduler(cfg *config.Config, dnsProvider dns.DNSProvider) *Scheduler {
	interval, err := time.ParseDuration(cfg.Interval)
	if err != nil || interval < time.Second {
		interval = 3 * time.Minute // 默认 3 分钟
	}

	s := &Scheduler{
		cfg:         cfg,
		dnsProvider: dnsProvider,
		chain:       NewRuleChain(cfg),
		interval:    interval,
		stop:        make(chan struct{}),
	}

	go s.run()
	log.Printf("[Scheduler] 定时更新已启动，间隔: %v", interval)
	return s
}

func (s *Scheduler) run() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// 立即执行一次
	s.update()

	for {
		select {
		case <-ticker.C:
			s.update()
		case <-s.stop:
			log.Printf("[Scheduler] 定时更新已停止")
			return
		}
	}
}

func (s *Scheduler) update() {
	// 加载所有规则（链式处理）
	s.chain.LoadAll()

	// 触发 DNS 更新
	if s.dnsProvider != nil {
		domains := make(map[string]string)
		ip := s.cfg.LanIP
		if ip == "" {
			ip = "192.168.100.1"  // 默认值
		}
		for d := range Rules {
			domains[d] = ip
		}
		if err := s.dnsProvider.UpdateDomains(domains); err != nil {
			log.Printf("[Scheduler] DNS 更新失败: %v", err)
		}
		s.dnsProvider.Reload()
	}
}

// Stop 停止定时更新
func (s *Scheduler) Stop() {
	close(s.stop)
}
