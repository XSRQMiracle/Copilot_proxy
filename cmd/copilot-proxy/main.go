package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/auth"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/config"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/proxy"
	"github.com/Open-Copilot-Proxy/Copilot_Proxy/internal/server"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "[✗]", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, args := parseGlobalOptions(args)
	cfg, resolvedPath, err := config.Load(opts.configPath)
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "", 0)

	if len(args) > 0 {
		switch args[0] {
		case "serve":
			return serve(cfg, resolvedPath, logger, !opts.noLogin)
		case "login":
			return login(cfg, logger)
		case "logout":
			return logout(cfg)
		case "config":
			return configCommand(args[1:], cfg, resolvedPath)
		case "help", "-h", "--help":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown command %q", args[0])
		}
	}
	return serve(cfg, resolvedPath, logger, !opts.noLogin)
}

type globalOptions struct {
	configPath string
	noLogin    bool
}

func parseGlobalOptions(args []string) (globalOptions, []string) {
	var opts globalOptions
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			i++
			opts.configPath = args[i]
		case args[i] == "--no-login":
			opts.noLogin = true
		default:
			filtered = append(filtered, args[i])
		}
	}
	return opts, filtered
}

func serve(cfg config.Config, configPath string, logger *log.Logger, interactiveLogin bool) error {
	printBanner()
	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	authManager := auth.NewManagerForActiveAccount(cfg, client)
	if err := authManager.LoadGitHubToken(); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if authManager.HasGitHubToken() {
		logger.Println("[~] 发现系统 keyring 中已保存的 GitHub Token，正在验证...")
		if err := authManager.RefreshCopilotToken(ctx); err != nil {
			logger.Printf("[!] Token 已失效或无法换取 Copilot Token: %v", err)
			_ = authManager.Logout()
		} else {
			logger.Printf("[✓] Copilot Token 刷新成功")
		}
	}

	if !authManager.HasGitHubToken() && interactiveLogin {
		if err := authManager.InteractiveLogin(ctx, logger.Printf); err != nil {
			return err
		}
		if err := authManager.RefreshCopilotToken(ctx); err != nil {
			_ = authManager.Logout()
			return fmt.Errorf("无法获取 Copilot Token: %w", err)
		}
	}
	if !authManager.HasGitHubToken() && !interactiveLogin {
		logger.Printf("[~] 未发现 GitHub Token，已跳过 CLI 授权流程；请在图形界面完成授权")
	}
	authManager.StartRefreshLoop(ctx, 25*time.Minute, logger.Printf)

	fallbackSelector := proxy.NewFallbackSelector(cfg, client)
	if authManager.HasCopilotToken() {
		headers := cfg.DefaultHeaders()
		headers["Authorization"] = "Bearer " + authManager.CopilotToken()
		if model, err := fallbackSelector.Choose(ctx, strings.TrimRight(cfg.Copilot.APIBase, "/")+"/models", headers); err == nil && model != "" {
			logger.Printf("[~] 已选择回退模型: %s", model)
		} else if err != nil {
			logger.Printf("[!] 回退模型选择失败: %v", err)
		}
	}

	stats := proxy.NewStats(500)
	proxyHandler := proxy.NewHandler(cfg, authManager, fallbackSelector, client, logger, stats)
	app := server.NewApp(cfg, configPath, authManager, fallbackSelector, proxyHandler, logger)
	httpServer := app.HTTPServer()

	logger.Printf("\n==================================================")
	logger.Printf("  Copilot Proxy 已启动!")
	logger.Printf("  API 地址: %s", cfg.PublicBaseURL())
	logger.Printf("  图形界面: %s/ui/", cfg.PublicBaseURL())
	logger.Printf("  配置文件: %s", configPath)
	logger.Printf("==================================================")
	logger.Printf("按 Ctrl+C 停止代理")
	logger.Printf("==================================================\n")

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	logger.Println("\n[~] 代理已停止，下次运行会自动使用系统 keyring 中保存的 Token")
	return nil
}

func login(cfg config.Config, logger *log.Logger) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	manager := auth.NewManagerForActiveAccount(cfg, client)
	if err := manager.InteractiveLogin(context.Background(), logger.Printf); err != nil {
		return err
	}
	return manager.RefreshCopilotToken(context.Background())
}

func logout(cfg config.Config) error {
	return auth.NewManagerForActiveAccount(cfg, nil).Logout()
}

func configCommand(args []string, cfg config.Config, path string) error {
	if len(args) == 0 || args[0] == "show" {
		raw, _ := json.MarshalIndent(cfg, "", "  ")
		fmt.Println(string(raw))
		return nil
	}
	if args[0] == "path" {
		fmt.Println(path)
		return nil
	}
	return fmt.Errorf("unknown config command %q", args[0])
}

func printUsage() {
	fmt.Println(`Usage:
  copilot-proxy [--config path]             Start proxy CLI
  copilot-proxy [--config path] serve       Start proxy CLI
  copilot-proxy [--config path] --no-login serve
                                           Start server without blocking for CLI login
  copilot-proxy [--config path] login       Run GitHub device login and store token in system keyring
  copilot-proxy [--config path] logout      Remove token from system keyring
  copilot-proxy [--config path] config show Print effective config
  copilot-proxy [--config path] config path Print config file path`)
}

func printBanner() {
	fmt.Print(`
   ____            _ _       _     ____
  / ___|___  _ __ (_) | ___ | |_  |  _ \ _ __ _____  ___   _
 | |   / _ \| '_ \| | |/ _ \| __| | |_) | '__/ _ \ \/ / | | |
 | |__| (_) | |_) | | | (_) | |_  |  __/| | | (_) >  <| |_| |
  \____\___/| .__/|_|_|\___/ \__| |_|   |_|  \___/_/\_\\__, |
            |_|                                         |___/
`)
}
