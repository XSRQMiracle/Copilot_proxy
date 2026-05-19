package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
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
		// 解密警告是非致命的：WebUI 仍可启动，用户可重刷 token 或重新授权
		fmt.Fprintln(os.Stderr, "[!]", err)
	}
	logger := log.New(os.Stdout, "", 0)

	if len(cmds) > 0 {
		switch cmds[0] {
		case "serve":
			return serve(cfg, resolvedPath, logger)
		case "login":
			return login(cfg, resolvedPath, logger)
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

	transport := &http.Transport{
		// 关闭 HTTP/2 以避免 Clash TUN 模式下返回空响应 (EOF)。
		// 许多代理/TUN 工具对 HTTP/2 支持不完整且无法回退到 HTTP/1.1，
		// 导致 Go 收到空响应体。
		TLSNextProto: make(map[string]func(authority string, c *tls.Conn) http.RoundTripper),
	}
	client := &http.Client{
		Timeout:   cfg.HTTPTimeout(),
		Transport: transport,
	}
	authManager := auth.NewManager(&cfg, configPath, client)

	logger.Printf("[~] 已加载 %d 个账号, activeID=%q, hasGitHubToken=%v, hasCopilotToken=%v",
		len(cfg.Auth.Accounts),
		authManager.ActiveAccountID(),
		authManager.HasGitHubToken(),
		authManager.HasCopilotToken())

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if authManager.HasGitHubToken() {
		logger.Println("[~] Found saved GitHub token, verifying...")
		if err := authManager.RefreshCopilotToken(ctx); err != nil {
			logger.Printf("[!] Token refresh failed: %v", err)
			logger.Println("[~] Use WebUI to re-authorize if the token has expired")
		} else {
			logger.Println("[+] Copilot token refreshed successfully")
		}
	}

	if !cfg.HasAdminPassword() {
		logger.Println("[!] No admin password configured — WebUI and API are accessible to anyone on the LAN")
		logger.Printf("[~] Set security.admin_password in %s or via the WebUI settings page", configPath)
	}

	if !authManager.HasCopilotToken() {
		logger.Println("[~] No active Copilot session. Use WebUI to authorize at", cfg.PublicBaseURL()+"/ui/")
		logger.Println("[~] Or use: copilot-proxy login")
	}

	authManager.StartRefreshLoop(ctx, 25*time.Minute, logger.Printf)

	stats := proxy.NewStats(500)
	proxyHandler := proxy.NewHandler(cfg, authManager, client, logger, stats)
	restartCh := make(chan struct{}, 1)
	app := server.NewApp(&cfg, configPath, authManager, proxyHandler, client, logger, restartCh)
	httpServer := app.HTTPServer()

	lanURL := ""
	if host := cfg.Server.Host; host == "" || host == "0.0.0.0" || host == "127.0.0.1" || host == "::" {
		if lanIP := lanIP(); lanIP != "" {
			lanURL = fmt.Sprintf("http://%s/ui/", net.JoinHostPort(lanIP, strconv.Itoa(cfg.Server.Port)))
		}
	}
	logger.Printf("")
	logger.Printf("  Copilot Proxy started!")
	logger.Printf("  API:      %s", cfg.PublicBaseURL())
	logger.Printf("  WebUI:    %s/ui/", cfg.PublicBaseURL())
	if lanURL != "" {
		logger.Printf("  LAN:      %s", lanURL)
	}
	logger.Printf("  Config:   %s", configPath)
	logger.Printf("  Ctrl+C to stop")
	logger.Printf("")

	// Shutdown on signal
	go func() {
		<-ctx.Done()
		logger.Println("[~] Shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	// Retry listener to handle brief TIME_WAIT / orphan process on restart.
	// When COPILOT_PROXY_RESTARTING=1 (spawned by a restarting instance),
	// retry up to 30 times (15 seconds) to give the old process time to release the port.
	var ln net.Listener
	{
		var err error
		maxAttempts := 3
		if os.Getenv("COPILOT_PROXY_RESTARTING") == "1" {
			maxAttempts = 30
		}
		for i := range maxAttempts {
			ln, err = net.Listen("tcp", httpServer.Addr)
			if err == nil {
				if i > 0 && os.Getenv("COPILOT_PROXY_RESTARTING") == "1" {
					logger.Printf("[~] Port acquired after %d attempts", i+1)
				}
				break
			}
			if i == 0 || i == maxAttempts-1 || (i+1)%10 == 0 {
				logger.Printf("[~] Port %s not available (attempt %d/%d), retrying...", cfg.PublicBaseURL(), i+1, maxAttempts)
			}
			time.Sleep(500 * time.Millisecond)
		}
		if err != nil {
			return fmt.Errorf("listen %s after retries: %w", httpServer.Addr, err)
		}
		defer ln.Close()
	}

	// ListenAndServe in a goroutine so we can also listen for restart signals
	serveErr := make(chan error, 1)
	go func() {
		serveErr <- httpServer.Serve(ln)
	}()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
		logger.Println("[~] Proxy stopped")
		return nil

	case <-restartCh:
		logger.Println("[~] Restarting...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_ = httpServer.Shutdown(shutdownCtx)
		cancel()

		exe, err := os.Executable()
		if err != nil {
			logger.Printf("[!] restart: %v", err)
			return nil
		}
		cmd := exec.Command(exe, os.Args[1:]...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = append(os.Environ(), "COPILOT_PROXY_RESTARTING=1")
		if err := cmd.Start(); err != nil {
			logger.Printf("[!] restart: %v", err)
			return nil
		}

		// Wait for new process to bind the port before exiting the old process.
		// This prevents port from going unclaimed, and ensures Ctrl+C correctly
		// targets the (still-running) parent process.
		logger.Println("[~] Waiting for new process to take over port...")
		if waitErr := waitForPort(httpServer.Addr, 30, 500*time.Millisecond); waitErr != nil {
			logger.Printf("[!] restart: new process may not have bound port: %v", waitErr)
		} else {
			logger.Println("[+] New process is listening, exiting")
		}
		return nil
	}
}

// waitForPort 轮询 TCP 端口，直到成功建立连接（表示新进程已开始监听）或超时。
// 用 Dial 而非 Listen 探测，避免与目标进程竞争端口绑定。
func waitForPort(addr string, attempts int, delay time.Duration) error {
	var lastErr error
	for range attempts {
		conn, err := net.DialTimeout("tcp", addr, delay)
		if err == nil {
			conn.Close()
			return nil
		}
		lastErr = err
		time.Sleep(delay)
	}
	return fmt.Errorf("port %s not accepting connections after %d attempts, last err: %w", addr, attempts, lastErr)
}

func login(cfg config.Config, configPath string, logger *log.Logger) error {
	client := &http.Client{Timeout: cfg.HTTPTimeout()}
	mgr := auth.NewManager(&cfg, configPath, client)

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

	if _, err := mgr.AddAccount(context.Background(), token); err != nil {
		return fmt.Errorf("save account: %w", err)
	}
	if err := mgr.RefreshCopilotToken(context.Background()); err != nil {
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
		for i := range cfg.Auth.Accounts {
			cfg.Auth.Accounts[i].GitHubToken = ""
		}
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

func lanIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}
		name := strings.ToLower(iface.Name)
		if strings.Contains(name, "tailscale") ||
			strings.Contains(name, "docker") ||
			strings.Contains(name, "hyper-v") ||
			strings.Contains(name, "vmware") ||
			strings.Contains(name, "virtualbox") ||
			strings.Contains(name, "vbox") ||
			strings.Contains(name, "meta") ||
			strings.Contains(name, "utun") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ipnet, ok := addr.(*net.IPNet)
			if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
				continue
			}
			if ipnet.IP[0] == 169 && ipnet.IP[1] == 254 {
				continue
			}
			return ipnet.IP.String()
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		ipnet, ok := addr.(*net.IPNet)
		if !ok || ipnet.IP.IsLoopback() || ipnet.IP.To4() == nil {
			continue
		}
		return ipnet.IP.String()
	}
	return ""
}
