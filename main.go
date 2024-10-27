package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := mainInner(); err != nil {
		slog.Default().Error(err.Error())
		os.Exit(1)
	}
}

type mainOptions struct {
	address  string
	logLevel int
}

func parseFlags(args []string) (*mainOptions, error) {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	opts := new(mainOptions)
	fs.StringVar(&opts.address, "address", ":8080", "address to listen on")
	fs.IntVar(&opts.logLevel, "loglevel", 0, "log level (DEBUG=-1,INFO=0,WARN=1,ERROR=2)")
	if err := fs.Parse(args[1:]); err != nil {
		return nil, fmt.Errorf("failed to parse flags: %w", err)
	}
	return opts, nil
}

func mainInner() error {
	opts, err := parseFlags(os.Args)
	if err != nil {
		return err
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.Level(opts.logLevel * 4)})))
	slog.Debug("parsed options", slog.Any("opts", opts))

	listener, err := net.Listen("tcp", opts.address)
	if err != nil {
		return fmt.Errorf("could not listen on %s: %w", opts.address, err)
	}
	defer func() {
		if err := listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Error("failed to close listener", slog.Any("err", err.Error()))
		} else {
			slog.Info("listener closed")
		}
	}()

	mux := http.NewServeMux()
	server := &http.Server{Handler: mux}
	defer func() {
		if err := server.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			slog.Error("failed to close http server", slog.Any("err", err.Error()))
		} else {
			slog.Info("server closed")
		}
	}()

	sigChannel := make(chan os.Signal, 1)
	signal.Notify(sigChannel, syscall.SIGINT, syscall.SIGTERM)
	shutdownFinished := make(chan bool)
	go func() {
		slog.Info("waiting for signal to shutdown")
		sig := <-sigChannel
		slog.Info("signal received - shutting down", slog.Any("signal", sig))
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := server.Shutdown(ctx); err != nil {
				slog.Error("failed to shutdown http server", slog.Any("err", err.Error()))
			} else {
				slog.Info("server shut down")
			}
			close(shutdownFinished)
		}()
		sig = <-sigChannel
		slog.Warn("second signal received - skipping shut down", slog.Any("signal", sig))
		close(shutdownFinished)
	}()

	slog.Info("serving http", slog.String("address", listener.Addr().String()))
	if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("could not serve on %s: %w", listener.Addr(), err)
	}
	<-shutdownFinished
	return nil
}
