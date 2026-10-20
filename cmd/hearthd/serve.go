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

	"github.com/rfizzle/hearth/internal/document"
	"github.com/rfizzle/hearth/internal/native"
	"github.com/rfizzle/hearth/internal/provider"
	"github.com/rfizzle/hearth/internal/rpc"
)

// runServe serves the contract until a signal arrives or stdin closes, and
// returns the process exit code.
func runServe(args []string) int {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:0", "TCP address to listen on")
	socket := fs.String("socket", "", "Unix socket path to listen on instead of --listen")
	dataDir := fs.String("data-dir", "", "folder for the database and credentials; default is the Hearth folder in Application Support")
	token := fs.String("token", "", "launch token for hand-runs from a terminal; the app sends it on stdin, and stdin wins")
	nativeAddr := fs.String("native", "", "loopback host:port of the app's native service; unset, pictures in documents are not read")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	listenSet := false
	fs.Visit(func(f *flag.Flag) { listenSet = listenSet || f.Name == "listen" })
	logger := log.New(os.Stderr, "hearthd: ", log.LstdFlags)
	usage := func() int {
		fmt.Fprintln(os.Stderr, "usage: hearthd serve [--listen host:port | --socket <path>] [--data-dir <folder>] [--native 127.0.0.1:<port>]")
		fmt.Fprintln(os.Stderr, "  The launch token is the first line of stdin; with --native, the app's own token is the second;")
		fmt.Fprintln(os.Stderr, "  the rest of stdin is the lifeline.")
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
	var recogniser func() document.Recogniser
	if *nativeAddr != "" {
		r, err := nativeRecogniser(*nativeAddr, stdin)
		if err != nil {
			logger.Print(err)
			return usage()
		}
		recogniser = r
	}

	dir := *dataDir
	if dir == "" {
		d, err := provider.DefaultDir()
		if err != nil {
			logger.Print(err)
			return 1
		}
		dir = d
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

	opts, closeCore, err := openCore(ctx, dir, recogniser)
	if err != nil {
		logger.Print(err)
		return 1
	}
	defer closeCore()
	opts.Version, opts.Token = version, tok

	srv := &http.Server{
		Handler:           rpc.NewHandler(opts),
		BaseContext:       func(net.Listener) context.Context { return ctx },
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
	// A crashed helper leaves its socket file behind and the next bind would
	// fail with "address in use". Remove only a socket, never another file.
	if fi, err := os.Lstat(socket); err == nil && fi.Mode()&os.ModeSocket != 0 {
		if err := os.Remove(socket); err != nil {
			return nil, "", err
		}
	}
	// Bind under a umask that leaves only the owner's bits, so the socket is
	// never group- or world-accessible, not even briefly after creation.
	old := syscall.Umask(0o177)
	l, err := net.Listen("unix", socket)
	syscall.Umask(old)
	if err != nil {
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

// nativeRecogniser is the app's reader of pictures at addr, a loopback
// address that comes on argv because a port is not a secret. Its bearer is
// the next line of r: a token of the app's own for this direction, never the
// launch token, so a process that takes the port after the app lets it go
// learns nothing that controls the helper.
func nativeRecogniser(addr string, r *bufio.Reader) (func() document.Recogniser, error) {
	tok, err := readToken(r)
	if err != nil || tok == "" {
		return nil, errors.New("--native needs the app's native token as the second line of stdin")
	}
	c, err := native.New(addr, tok)
	if err != nil {
		return nil, err
	}
	return c.ForRun, nil
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
