package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"
)

type config struct {
	port          string
	botBin        string
	runSecret     string
	runTimeout    time.Duration
	gitlabHost    string
	gitlabToken   string
	gitlabRepo    string
	reviewersPath string
	templatePath  string
	webhookURL    string
	webhookAuth   string
	channel       string
}

func loadConfig() config {
	c := config{
		port:          getenv("PORT", "8080"),
		botBin:        getenv("BOT_BIN", "/app/kc-review-bot"),
		runSecret:     getenv("RUN_SECRET", ""),
		gitlabHost:    getenv("GITLAB_HOST", ""),
		gitlabToken:   os.Getenv("GITLAB_TOKEN"),
		gitlabRepo:    getenv("GITLAB_REPO", ""),
		reviewersPath: getenv("REVIEWERS_PATH", "/app/reviewers.json"),
		templatePath:  getenv("TEMPLATE_PATH", ""),
		webhookURL:    getenv("WEBHOOK_URL", ""),
		webhookAuth:   os.Getenv("WEBHOOK_AUTH"),
		channel:       getenv("CHANNEL", ""),
	}

	timeout := getenv("RUN_TIMEOUT", "300")
	seconds, err := strconv.Atoi(timeout)
	if err != nil || seconds <= 0 {
		log.Fatalf("invalid RUN_TIMEOUT %q", timeout)
	}
	c.runTimeout = time.Duration(seconds) * time.Second

	return c
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type runResponse struct {
	Status string `json:"status"`
	Output string `json:"output,omitempty"`
}

var running atomic.Bool

func (c config) botArgs() []string {
	args := []string{
		"-host", c.gitlabHost,
		"-repo", c.gitlabRepo,
		"-reviewers", c.reviewersPath,
	}
	if c.gitlabToken != "" {
		args = append(args, "-token", c.gitlabToken)
	}
	if c.templatePath != "" {
		args = append(args, "-template", c.templatePath)
	}
	if c.webhookURL != "" {
		args = append(args, "-webhook", c.webhookURL)
	}
	if c.webhookAuth != "" {
		args = append(args, "-webhook-auth", c.webhookAuth)
	}
	if c.channel != "" {
		args = append(args, "-channel", c.channel)
	}
	return args
}

func (c config) authorized(r *http.Request) bool {
	if c.runSecret == "" {
		return true
	}

	candidates := []string{}
	if auth := r.Header.Get("Authorization"); len(auth) > 7 && auth[:7] == "Bearer " {
		candidates = append(candidates, auth[7:])
	}
	if t := r.Header.Get("X-Run-Token"); t != "" {
		candidates = append(candidates, t)
	}
	if k := r.URL.Query().Get("key"); k != "" {
		candidates = append(candidates, k)
	}

	for _, candidate := range candidates {
		if subtle.ConstantTimeCompare([]byte(candidate), []byte(c.runSecret)) == 1 {
			return true
		}
	}
	return false
}

func (c config) execute() (int, string) {
	ctx, cancel := context.WithTimeout(context.Background(), c.runTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, c.botBin, c.botArgs()...)
	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return http.StatusGatewayTimeout, fmt.Sprintf("timed out after %s\n%s", c.runTimeout, output)
	}
	if err != nil {
		log.Printf("bot failed: %v", err)
		return http.StatusInternalServerError, string(output)
	}
	return http.StatusOK, string(output)
}

func (c config) handleRun(w http.ResponseWriter, r *http.Request) {
	if !c.authorized(r) {
		w.Header().Set("WWW-Authenticate", `Bearer realm="kc-review-bot"`)
		writeJSON(w, http.StatusUnauthorized, runResponse{Status: "unauthorized"})
		return
	}

	if r.URL.Query().Get("async") == "1" || r.Header.Get("X-Run-Async") == "1" {
		if !running.CompareAndSwap(false, true) {
			writeJSON(w, http.StatusConflict, runResponse{Status: "already running"})
			return
		}
		go func() {
			defer running.Store(false)
			status, output := c.execute()
			log.Printf("async run finished: status=%d output=%q", status, truncate(output, 500))
		}()
		writeJSON(w, http.StatusAccepted, runResponse{Status: "started"})
		return
	}

	if !running.CompareAndSwap(false, true) {
		writeJSON(w, http.StatusConflict, runResponse{Status: "already running"})
		return
	}
	defer running.Store(false)

	started := time.Now()
	status, output := c.execute()
	log.Printf("run finished in %s: status=%d", time.Since(started).Round(time.Second), status)
	writeJSON(w, status, runResponse{Status: http.StatusText(status), Output: output})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "...(truncated)"
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func main() {
	cfg := loadConfig()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/run", cfg.handleRun)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"service":   "kc-review-bot",
			"endpoints": "GET /health, GET /run",
		})
	})

	addr := ":" + cfg.port
	log.Printf("listening on %s", addr)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: cfg.runTimeout + 30*time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
