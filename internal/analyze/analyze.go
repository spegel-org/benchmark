package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gonum.org/v1/gonum/stat"
	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

	"github.com/spegel-org/benchmark/internal/measure"
)

func Analyze(ctx context.Context, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	result := measure.Result{}
	err = json.Unmarshal(b, &result)
	if err != nil {
		return err
	}
	ext := filepath.Ext(path)
	outPath := strings.TrimSuffix(path, ext)
	outPath = fmt.Sprintf("%s.png", outPath)
	err = createPlot(result, outPath)
	if err != nil {
		return err
	}
	return nil
}

func createPlot(result measure.Result, path string) error {
	plots := []*plot.Plot{}
	for _, bench := range result.Benchmarks {
		p := plot.New()
		p.Title.Text = bench.Image
		p.Title.Padding = vg.Points(10)
		p.Y.Min = 0
		p.Y.Label.Text = "Pod Number"
		p.X.Label.Padding = 3
		slices.SortFunc(bench.Measurements, func(a, b measure.Measurement) int {
			if a.Start.Equal(b.Start) {
				return a.Stop.Compare(b.Stop)
			}
			return a.Start.Compare(b.Start)
		})
		zeroTime := bench.Measurements[0].Start

		sum := int64(0)
		durations := []float64{}
		for i, result := range bench.Measurements {
			sum += result.Duration.Milliseconds()
			durations = append(durations, float64(result.Duration.Milliseconds()))

			start := result.Start.Sub(zeroTime)
			stop := start + result.Duration
			b, err := plotter.NewBoxPlot(4, float64(len(bench.Measurements)-i-1), plotter.Values{float64(start.Milliseconds()), float64(stop.Milliseconds())})
			if err != nil {
				return err
			}
			b.Horizontal = true
			b.FillColor = color.Black
			p.Add(b)
		}
		slices.Sort(durations)
		mean := stat.Mean(durations, nil)
		p90 := stat.Quantile(0.90, stat.Empirical, durations, nil)
		p95 := stat.Quantile(0.95, stat.Empirical, durations, nil)
		p.X.Label.Text = fmt.Sprintf("Time [ms}\n\nMean: %.0f ms | P90: %.0f ms | P95: %.0f ms | Total: %d ms", mean, p90, p95, sum)
		plots = append(plots, p)
	}

	img := vgimg.New(vg.Points(700), vg.Points(350))
	dc := draw.New(img)
	t := draw.Tiles{
		Rows:      1,
		Cols:      len(plots),
		PadX:      vg.Millimeter,
		PadY:      vg.Millimeter,
		PadTop:    vg.Points(10),
		PadBottom: vg.Points(10),
		PadLeft:   vg.Points(10),
		PadRight:  vg.Points(10),
	}
	canv := plot.Align([][]*plot.Plot{plots}, t, dc)
	for i, plot := range plots {
		plot.Draw(canv[0][i])
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	png := vgimg.PngCanvas{Canvas: img}
	if _, err := png.WriteTo(file); err != nil {
		return err
	}
	return nil
}
