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
		fmt.Fprintln(os.Stderr, "[!]", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	opts, cmds := parseGlobalOptions(args)

	cfg, resolvedPath, err := config.LoadWithDecryptedTokens(opts.configPath)
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "", 0)

	if len(cmds) > 0 {
		switch cmds[0] {
		case "serve":
			return serve(cfg, resolvedPath, logger)
		case "login":
			return login(cfg, logger)
		case "logout":
			return logout(cfg, resolvedPath, logger)
		case "config":
			return configCommand(cmds[1:], cfg, resolvedPath)
		case "help", "-h", "--help":
			printUsage()
			return nil
		default:
			return fmt.Errorf("unknown command %q", cmds[0])
		}
	}
	return serve(cfg, resolvedPath, logger)
}

type globalOptions struct {
	configPath string
}

func parseGlobalOptions(args []string) (globalOptions, []string) {
	var opts globalOptions
	var filtered []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--config" && i+1 < len(args):
			i++
			opts.configPath = args[i]
		default:
			filtered = append(filtered, args[i])
		}
	}
	return opts, filtered
}

func serve(cfg config.Config, configPath string, logger *log.Logger) error {
	printBanner()

	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	authManager := auth.NewManager(&cfg, configPath, client)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if authManager.HasGitHubToken() {
		logger.Println("[~] Found saved GitHub token, verifying...")
		if err := authManager.RefreshCopilotToken(ctx); err != nil {
			logger.Printf("[!] Token invalid or cannot exchange for Copilot token: %v", err)
			_ = authManager.RemoveAccount(ctx, authManager.ActiveAccountID())
		} else {
			logger.Println("[+] Copilot token refreshed successfully")
		}
	}

	if !authManager.HasCopilotToken() {
		logger.Println("[~] No active Copilot session. Use WebUI to authorize at", cfg.PublicBaseURL()+"/ui/")
		logger.Println("[~] Or use: copilot-proxy login")
	}

	authManager.StartRefreshLoop(ctx, 25*time.Minute, logger.Printf)

	fallbackSelector := proxy.NewFallbackSelector(cfg, client)
	if authManager.HasCopilotToken() {
		headers := cfg.DefaultHeaders()
		headers["Authorization"] = "Bearer " + authManager.CopilotToken()
		if model, err := fallbackSelector.Choose(ctx, strings.TrimRight(cfg.Copilot.APIBase, "/")+"/models", headers); err == nil && model != "" {
			logger.Printf("[~] Fallback model selected: %s", model)
		} else if err != nil {
			logger.Printf("[!] Fallback model selection failed: %v", err)
		}
	}

	stats := proxy.NewStats(500)
	proxyHandler := proxy.NewHandler(cfg, authManager, fallbackSelector, client, logger, stats)
	app := server.NewApp(&cfg, configPath, authManager, fallbackSelector, proxyHandler, logger)
	httpServer := app.HTTPServer()

	logger.Printf("")
	logger.Printf("  Copilot Proxy started!")
	logger.Printf("  API:      %s", cfg.PublicBaseURL())
	logger.Printf("  WebUI:    %s/ui/", cfg.PublicBaseURL())
	logger.Printf("  Config:   %s", configPath)
	logger.Printf("  Ctrl+C to stop")
	logger.Printf("")

	go func() {
		<-ctx.Done()
		logger.Println("[~] Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	if authManager.HasCopilotToken() {
		go func() {
			time.Sleep(500 * time.Millisecond)
			if err := auth.OpenBrowser(cfg.PublicBaseURL() + "/ui/"); err != nil {
				logger.Printf("[~] %v", err)
			}
		}()
	}

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return err
	}
	logger.Println("[~] Proxy stopped")
	return nil
}

func login(cfg config.Config, logger *log.Logger) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	mgr := auth.NewManager(&cfg, "", client)

	flow, err := mgr.StartDeviceFlow(context.Background())
	if err != nil {
		return fmt.Errorf("start device flow: %w", err)
	}

	fmt.Println()
	fmt.Println("  GitHub Copilot Authorization")
	fmt.Println("  ============================")
	fmt.Println("  Open:", flow.VerificationURI)
	fmt.Println("  Code:", flow.UserCode)
	fmt.Println("  ============================")
	fmt.Println()

	if err := auth.OpenBrowser(flow.VerificationURI); err != nil {
		logger.Printf("[~] %v", err)
	}

	token, err := mgr.WaitForAccessToken(context.Background(), flow, func(remaining time.Duration) {
		logger.Printf("[~] Waiting for authorization... %v remaining", remaining.Round(time.Second))
	})
	if err != nil {
		return fmt.Errorf("wait for token: %w", err)
	}

	// 保存 token 到配置
	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg2, _, err := config.LoadWithDecryptedTokens(configPath)
	if err != nil {
		return err
	}
	mgr2 := auth.NewManager(&cfg2, configPath, client)
	if _, err := mgr2.AddAccount(context.Background(), token); err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	if err := mgr2.RefreshCopilotToken(context.Background()); err != nil {
		return fmt.Errorf("refresh copilot token: %w", err)
	}
	fmt.Println("[+] Authorization successful!")
	return nil
}

func logout(cfg config.Config, configPath string, logger *log.Logger) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	mgr := auth.NewManager(&cfg, configPath, client)
	account := mgr.ActiveAccount()
	if account == nil {
		fmt.Println("[~] No active account to logout")
		return nil
	}
	if err := mgr.RemoveAccount(context.Background(), account.ID); err != nil {
		return fmt.Errorf("remove account: %w", err)
	}
	fmt.Println("[+] Logged out successfully")
	return nil
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
  copilot-proxy                    Start proxy server (no forced login)
  copilot-proxy serve              Start proxy server
  copilot-proxy [--config path]    Start with custom config path
  copilot-proxy login              Run GitHub device authorization
  copilot-proxy logout             Remove current account
  copilot-proxy config show        Print effective config
  copilot-proxy config path        Print config file path

Options:
  --config <path>                  Config file path

The first run creates a default config at <exe_dir>/config/config.json.
WebUI is available at http://localhost:15432/ui/ after starting the server.`)
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
