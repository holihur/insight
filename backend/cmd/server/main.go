package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/codeinsight/code-insight/internal/agent"
	"github.com/codeinsight/code-insight/internal/api"
	"github.com/codeinsight/code-insight/internal/config"
	"github.com/codeinsight/code-insight/internal/coverage"
	"github.com/codeinsight/code-insight/internal/llm"
	"github.com/codeinsight/code-insight/internal/store"
	webassets "github.com/codeinsight/code-insight/web"
)

func main() {
	cfg := config.Load()

	st, err := store.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("open data store: %v", err)
	}

	model, err := llm.New(context.Background(), cfg.Model)
	if err != nil {
		if cfg.Model.Provider == "mock" {
			log.Printf("running in demo mode: %v", err)
		} else {
			log.Printf("model unavailable (%v); falling back to local demo mode", err)
			cfg.Model.Provider = "mock"
			cfg.Model.Model = ""
			model = nil
		}
	}
	if model == nil {
		log.Printf("no model configured — wiki pages and chat answers will be generated locally")
	} else {
		log.Printf("model: %s", llm.Label(cfg.Model))
	}

	gen := agent.NewGenerator(cfg, st, model)
	chat := agent.NewChatter(cfg, st, model)
	cov := coverage.New(cfg, st)
	srv := api.New(cfg, st, gen, chat, cov)

	if cfg.WebDir != "" {
		log.Printf("serving frontend from %s", cfg.WebDir)
	} else if webassets.Available() {
		log.Printf("serving embedded frontend")
	} else {
		log.Printf("warning: no frontend build embedded; run `mise run build`")
	}

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		log.Printf("Code Insight listening on http://localhost%s", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("shutdown: %v", err)
	}
	log.Println("bye")
}
