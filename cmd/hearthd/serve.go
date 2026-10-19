package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rfizzle/hearth/internal/rpc"
)

// runServe serves the contract until a signal arrives or stdin closes, and
// returns the process exit code.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:0", "TCP address to listen on")
	socket := fs.String("socket", "", "Unix socket path to listen on instead of --listen")
	token := fs.String("token", "", "launch token for hand-runs from a terminal; the app sends it on stdin, and stdin wins")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	listenSet := false
	fs.Visit(func(f *flag.Flag) { listenSet = listenSet || f.Name == "listen" })
	logger := log.New(os.Stderr, "hearthd: ", log.LstdFlags)
	usage := func() int {
		fmt.Fprintln(os.Stderr, "usage: hearthd serve [--listen host:port | --socket <path>]")
		fmt.Fprintln(os.Stderr, "  The launch token is the first line of stdin; the rest of stdin is the lifeline.")
		fmt.Fprintln(os.Stderr, "  By hand from a terminal: hearthd serve --token <t>")
		fs.PrintDefaults()
		return 2
	}
	if *socket != "" && listenSet {
		return usage()
	}

	// The token stays off argv so `ps` never shows it. A hand-run may still
	// pass --token, which every local user can see; a token on stdin wins.
	stdin := bufio.NewReader(os.Stdin)
	tok := *token
	if fi, err := os.Stdin.Stat(); *token == "" || (err == nil && fi.Mode()&os.ModeCharDevice == 0) {
		line, err := readToken(stdin)
		switch {
		case line != "" && *token != "":
			logger.Print("ignoring --token: the token on stdin wins")
			tok = line
		case line != "":
			tok = line
		case err != nil && *token == "":
			return usage()
		}
	}
	if tok == "" {
		return usage()
	}

	l, addr, err := bind(*listen, *socket)
	if err != nil {
		logger.Print(err)
		return 1
	}
	// The app reads the first stdout line as the address to dial.
	fmt.Println(addr)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx = lifeline(ctx, stdin)

	srv := &http.Server{
		Handler:           rpc.NewHandler(rpc.Options{Version: version, Token: tok}),
		ReadHeaderTimeout: 5 * time.Second,
		ErrorLog:          logger,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(l) }()

	select {
	case err := <-errc:
		logger.Print(err)
		return 1
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		logger.Print(err)
	}
	return 0
}

// bind listens on the Unix socket when one is named, else on the TCP address,
// and returns the address the app should dial.
func bind(listen, socket string) (net.Listener, string, error) {
	if socket == "" {
		l, err := net.Listen("tcp", listen)
		if err != nil {
			return nil, "", err
		}
		return l, l.Addr().String(), nil
	}
	l, err := net.Listen("unix", socket)
	if err != nil {
		return nil, "", err
	}
	if err := os.Chmod(socket, 0o600); err != nil {
		_ = l.Close()
		return nil, "", err
	}
	return l, socket, nil
}

// readToken reads the launch token: the first line of r, without its line end.
func readToken(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if errors.Is(err, io.EOF) && line != "" {
		err = nil
	}
	return strings.TrimRight(line, "\r\n"), err
}

// lifeline returns a context cancelled when r reaches EOF or fails. The app
// holds the write end of hearthd's stdin and never writes to it; the kernel
// closes it when the app exits or crashes, which makes it the one exit signal
// that survives a crash. HTTP connections are per request, so no connection
// can play that part.
// See docs/architecture.md#3-the-contract-sketch
func lifeline(parent context.Context, r io.Reader) context.Context {
	ctx, cancel := context.WithCancel(parent)
	go func() {
		defer cancel()
		_, _ = io.Copy(io.Discard, r)
	}()
	return ctx
}
