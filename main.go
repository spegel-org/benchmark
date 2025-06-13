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
	OutputDir      string   `arg:"--output-dir,required"`
	KubeconfigPath string   `arg:"--kubeconfig,env:KUBECONFIG"`
	Namespace      string   `arg:"--namespace" default:"spegel-benchmark"`
	Images         []string `arg:"--images,required"`
}

type SuiteCmd struct {
	OutputDir      string `arg:"--output-dir,required"`
	KubeconfigPath string `arg:"--kubeconfig,env:KUBECONFIG"`
	Namespace      string `arg:"--namespace" default:"spegel-benchmark"`
	Name           string `arg:"--name,required"`
}

type AnalyzeCmd struct {
	OutputDir  string   `arg:"--output-dir,required"`
	SuitePaths []string `arg:"--suite-paths,required"`
}

type Arguments struct {
	Generate *GenerateCmd `arg:"subcommand:generate" help:"Generate images for benchmarking."`
	Measure  *MeasureCmd  `arg:"subcommand:measure" help:"Run benchmark measurement."`
	Suite    *SuiteCmd    `arg:"subcommand:suite" help:"Run the full suite of measurements."`
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
		return measure.RunMeasure(ctx, args.Measure.KubeconfigPath, args.Measure.Namespace, args.Measure.OutputDir, args.Measure.Images)
	case args.Suite != nil:
		if args.Suite.KubeconfigPath == "" {
			return errors.New("kubeconfig path cannot be empty")
		}
		return measure.RunSuite(ctx, args.Suite.KubeconfigPath, args.Suite.Namespace, args.Suite.OutputDir, args.Suite.Name)
	case args.Analyze != nil:
		return analyze.Analyze(ctx, args.Analyze.SuitePaths, args.Analyze.OutputDir)
	default:
		return errors.New("unknown command")
	}
}
