package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/alexflint/go-arg"
	"github.com/c2h5oh/datasize"
	"github.com/go-logr/logr"

	"github.com/spegel-org/benchmark/internal/analyze"
	"github.com/spegel-org/benchmark/internal/generate"
	"github.com/spegel-org/benchmark/internal/measure"
)

type GenerateCmd struct {
	ImageName  string            `arg:"--image-name,required"`
	LayerCount int               `arg:"--layer-count,required"`
	ImageSize  datasize.ByteSize `arg:"--image-size,required"`
}

type MeasureCmd struct {
	ResultDir      string   `arg:"--result-dir,required"`
	KubeconfigPath string   `arg:"--kubeconfig,env:KUBECONFIG"`
	Namespace      string   `arg:"--namespace" default:"spegel-benchmark"`
	Images         []string `arg:"--images,required"`
}

type AnalyzeCmd struct {
	Path string `args:"--path,required"`
}

type Arguments struct {
	Generate *GenerateCmd `arg:"subcommand:generate" help:"Generate images for benchmarking."`
	Measure  *MeasureCmd  `arg:"subcommand:measure" help:"Run benchmark measurement."`
	Analyze  *AnalyzeCmd  `arg:"subcommand:analyze" help:"Analyze benchmark results."`
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
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	opts := slog.HandlerOptions{
		AddSource: false,
		Level:     slog.Level(-1),
	}
	handler := slog.NewTextHandler(os.Stderr, &opts)
	log := logr.FromSlogHandler(handler)
	ctx = logr.NewContext(ctx, log)

	switch {
	case args.Generate != nil:
		return generate.Generate(ctx, args.Generate.ImageName, args.Generate.LayerCount, args.Generate.ImageSize)
	case args.Measure != nil:
		if args.Measure.KubeconfigPath == "" {
			return errors.New("kubeconfig path cannot be empty")
		}
		return measure.Measure(ctx, args.Measure.KubeconfigPath, args.Measure.Namespace, args.Measure.ResultDir, args.Measure.Images)
	case args.Analyze != nil:
		return analyze.Analyze(ctx, args.Analyze.Path)
	default:
		return errors.New("unknown command")
	}
}
