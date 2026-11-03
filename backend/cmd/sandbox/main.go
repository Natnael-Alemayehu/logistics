// Command sandbox runs the provider sandbox: Ethio telecom's SMS gateway,
// telebirr and CBE Birr as Menged reaches them, with the phones and payers
// on the other side (internal/sandbox).
//
//	sandbox        serve on SANDBOX_ADDR (127.0.0.1:8091)
//	sandbox keys   write throwaway telebirr key pairs to SANDBOX_KEYS_DIR
//
// The providers' settings are the variables Menged reads, so the two share
// sandbox.env (make sandbox, make run-sandbox). The sandbox's own:
//
//	SANDBOX_ADDR        where to listen
//	SANDBOX_APP_URL     Menged, which the providers call (PUBLIC_URL, or http://localhost:$PORT)
//	SANDBOX_KEYS_DIR    the key pairs (sandbox-keys)
//	SANDBOX_PIN         every wallet's PIN (1234)
//	SANDBOX_HOST_ALIAS  what localhost is from inside a container (host.docker.internal)
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/natnael-alemayehu/logistics/internal/sandbox"
)

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "keys":
		err = keys()
	case "help", "-h", "--help":
		fmt.Println("usage: sandbox [serve | keys]")
		return
	default:
		err = fmt.Errorf("unknown command %q (want serve or keys)", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sandbox:", err)
		os.Exit(1)
	}
}

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func serve() error {
	cfg, err := sandbox.FromEnv(os.Getenv)
	if err != nil {
		return err
	}
	cfg.Out = os.Stdout
	sb, err := sandbox.New(cfg)
	if err != nil {
		return err
	}
	defer sb.Close()

	addr := env("SANDBOX_ADDR", "127.0.0.1:8091")
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: sb.Handler(), ReadHeaderTimeout: 10 * time.Second}
	fmt.Printf("Provider sandbox on http://%s for %s at %s: no text reaches a phone and no money moves.\n",
		shown(ln.Addr().String()), cfg.AppName, cfg.AppURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdown); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return nil
}

// shown is a listening address as a browser on the machine reaches it.
func shown(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if ip := net.ParseIP(host); host == "" || (ip != nil && (ip.IsUnspecified() || ip.IsLoopback())) {
		host = "localhost"
	}
	return net.JoinHostPort(host, port)
}

func keys() error {
	dir := env("SANDBOX_KEYS_DIR", "sandbox-keys")
	written, err := sandbox.WriteKeys(dir)
	for _, f := range written {
		fmt.Println("wrote", f)
	}
	return err
}
