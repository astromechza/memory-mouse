package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
)

func main() {
	if err := mainInner(); err != nil {
		slog.Default().Error(err.Error())
		os.Exit(1)
	}
}

type mainOptions struct {
	port int
}

func parseFlags(args []string) (*mainOptions, error) {
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	opts := new(mainOptions)
	fs.IntVar(&opts.port, "port", 8080, "port to listen on")
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
	// TODO: manipulate logger via flag
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	slog.Debug("parsed options", slog.Any("opts", opts))
	return nil
}
