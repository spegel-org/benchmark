package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexflint/go-arg"

	"github.com/spegel-org/benchmark/internal/analyze"
	"github.com/spegel-org/benchmark/internal/measure"
)

type MeasureCmd struct {
	ResultDir      string   `arg:"--result-dir,required"`
	Name           string   `arg:"--name,required"`
	KubeconfigPath string   `arg:"--kubeconfig,required"`
	Namespace      string   `arg:"--namespace,required"`
	Images         []string `arg:"--images,required"`
}

type AnalyzeCmd struct {
	Path string `args:"--path,required"`
}

type Arguments struct {
	Measure *MeasureCmd `arg:"subcommand:measure"`
	Analyze *AnalyzeCmd `arg:"subcommand:analyze"`
}

func main() {
	args := &Arguments{}
	arg.MustParse(args)
	err := run(*args)
	if err != nil {
		fmt.Println("unexpected error:", err)
		os.Exit(1)
	}
}

func run(args Arguments) error {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	switch {
	case args.Measure != nil:
		return measure.Measure(ctx, args.Measure.KubeconfigPath, args.Measure.Namespace, args.Measure.Name, args.Measure.ResultDir, args.Measure.Images)
	case args.Analyze != nil:
		return analyze.Analyze(ctx, args.Analyze.Path)
	default:
		return fmt.Errorf("unknown command")
	}
}
