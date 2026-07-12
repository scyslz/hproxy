package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"hproxy/config"
	"hproxy/rules"
)

var cfg *config.Config

// Handler 返回管理接口 handler
func Handler(c *config.Config) http.Handler {
	cfg = c  // 保存配置
	mux := http.NewServeMux()

	mux.HandleFunc("/config", configHandler(cfg))
	mux.HandleFunc("/rules", rulesHandler)
	mux.HandleFunc("/logs", logsHandler)
	mux.HandleFunc("/logs/start", logsStartHandler)
	mux.HandleFunc("/reload", reloadHandler(cfg))

	return mux
}

func configHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cfg)
	}
}

func rulesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	result := struct {
		Rules []map[string]interface{} `json:"rules"`
		Wild  []map[string]interface{} `json:"wild"`
	}{}
	
	// 精确规则
	for host, rule := range rules.Rules {
		result.Rules = append(result.Rules, map[string]interface{}{
			"source": rule.Source,
			"host":   host,
			"rule":   rule,
		})
	}
	
	// 通配符规则
	for host, rule := range rules.WildRules {
		result.Wild = append(result.Wild, map[string]interface{}{
			"source": rule.Source,
			"host":   host,
			"rule":   rule,
		})
	}
	
	json.NewEncoder(w).Encode(result)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	
	logDir := "logs"
	if cfg != nil && cfg.LogFile != "" {
		logDir = filepath.Dir(cfg.LogFile) + "/logs"
	}
	
	if info, err := os.Stat(logDir); err != nil || !info.IsDir() {
		http.Error(w, "日志目录不存在", http.StatusInternalServerError)
		return
	}
	
	// 找最新的日志文件（按修改时间排序）
	files, err := os.ReadDir(logDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	type fileInfo struct {
		name    string
		modTime time.Time
	}
	
	var logFiles []fileInfo
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".log") {
			info, _ := f.Info()
			logFiles = append(logFiles, fileInfo{name: f.Name(), modTime: info.ModTime()})
		}
	}
	
	if len(logFiles) == 0 {
		http.Error(w, "没有日志文件", http.StatusNotFound)
		return
	}
	
	// 按修改时间降序排序
	sort.Slice(logFiles, func(i, j int) bool {
		return logFiles[i].modTime.After(logFiles[j].modTime)
	})
	
	latestFile := logFiles[0].name
	
	// 读取最新日志文件内容
	content, err := os.ReadFile(filepath.Join(logDir, latestFile))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	w.Write(content)
}

func logsStartHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	
	logDir := "logs"
	if cfg != nil && cfg.LogFile != "" {
		logDir = filepath.Dir(cfg.LogFile) + "/logs"
	}
	os.MkdirAll(logDir, 0755)
	
	// 找当前最新的日志文件（YYYY-MM-DD.log）
	files, err := os.ReadDir(logDir)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"msg":    err.Error(),
		})
		return
	}
	
	// 找当前日志文件（不含 _ 的 .log 文件）
	var currentFile string
	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".log") && !strings.Contains(f.Name(), "_") {
			currentFile = f.Name()
			break
		}
	}
	
	if currentFile == "" {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"msg":    "没有当前日志文件",
		})
		return
	}
	
	// 归档当前文件（加序号 001）
	baseName := strings.TrimSuffix(currentFile, ".log")
	archiveFile := fmt.Sprintf("%s_001.log", baseName)
	
	// 如果 001 存在，递增序号
	for i := 1; i <= 999; i++ {
		testFile := fmt.Sprintf("%s_%03d.log", baseName, i)
		if _, err := os.Stat(filepath.Join(logDir, testFile)); os.IsNotExist(err) {
			archiveFile = testFile
			break
		}
	}
	
	// 重命名当前文件
	oldPath := filepath.Join(logDir, currentFile)
	newPath := filepath.Join(logDir, archiveFile)
	if err := os.Rename(oldPath, newPath); err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"msg":    fmt.Sprintf("归档失败: %v", err),
		})
		return
	}
	
	log.Printf("[Admin] 日志已归档: %s → %s", currentFile, archiveFile)
	
	// 创建新的日志文件（原文件名）
	newLogFile := filepath.Join(logDir, currentFile)
	f, err := os.OpenFile(newLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"msg":    err.Error(),
		})
		return
	}
	
	log.SetOutput(f)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.Printf("[Admin] 新日志文件: %s", currentFile)
	
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"archived": archiveFile,
		"new_file": currentFile,
	})
}

func reloadHandler(cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[Admin] 手动触发重载")
		// 重新加载规则
		chain := rules.NewRuleChain(cfg)
		chain.LoadAll()
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"rules":  len(rules.Rules),
			"wild":   len(rules.WildRules),
		})
	}
}
