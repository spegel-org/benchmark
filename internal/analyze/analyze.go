package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"

	"github.com/spegel-org/benchmark/internal/measure"
)

func Analyze(ctx context.Context, baselineDir, variantDir, outputDir string) error {
	entries, err := os.ReadDir(baselineDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(baselineDir, entry.Name()))
		if err != nil {
			return err
		}
		baseline := measure.Result{}
		err = json.Unmarshal(b, &baseline)
		if err != nil {
			return err
		}
		b, err = os.ReadFile(filepath.Join(variantDir, entry.Name()))
		if err != nil {
			return err
		}
		variant := measure.Result{}
		err = json.Unmarshal(b, &variant)
		if err != nil {
			return err
		}
		ext := filepath.Ext(entry.Name())
		outputPath := strings.TrimSuffix(entry.Name(), ext)
		outputPath = filepath.Join(outputDir, outputPath)
		err = createBoxPlot(baseline, variant, outputPath)
		if err != nil {
			return err
		}
	}
	return nil
}

func createBoxPlot(baseline, variant measure.Result, outputPath string) error {
	if len(baseline.Benchmarks) != len(variant.Benchmarks) {
		return errors.New("results cant have different benchmark counts")
	}

	bp := charts.NewBoxPlot()
	bp.SetGlobalOptions(
		charts.WithYAxisOpts(opts.YAxis{Name: "Duration (seconds)", NameLocation: "middle", NameGap: 40}),
		charts.WithLegendOpts(opts.Legend{Left: "70%"}),
		charts.WithAnimation(false),
	)

	boxData := map[string][]opts.BoxPlotData{}
	xAxis := []string{}
	for i := range len(baseline.Benchmarks) {
		if len(baseline.Benchmarks[i].Measurements) != len(variant.Benchmarks[i].Measurements) {
			return errors.New("benchmarks cant have different measurement counts")
		}
		if baseline.Benchmarks[i].Image != variant.Benchmarks[i].Image {
			return errors.New("benchmark images are not the same")
		}

		xAxis = append(xAxis, baseline.Benchmarks[i].Image)

		durations := []float64{}
		for _, meas := range baseline.Benchmarks[i].Measurements {
			durations = append(durations, meas.Duration.Seconds())
		}
		boxData["Baseline"] = append(boxData["Baseline"], opts.BoxPlotData{Value: createBoxPlotData(durations)})

		durations = []float64{}
		for _, meas := range variant.Benchmarks[i].Measurements {
			durations = append(durations, meas.Duration.Seconds())
		}
		boxData["Spegel"] = append(boxData["Spegel"], opts.BoxPlotData{Value: createBoxPlotData(durations)})
	}
	bp.SetXAxis(xAxis)
	itemStyles := []opts.ItemStyle{
		{BorderColor: "#164577", Color: "#9CC1E3"},
		{BorderColor: "#FAA93B", Color: "#FAEAD4"},
	}
	for i, v := range []string{"Baseline", "Spegel"} {
		bp.AddSeries(v, boxData[v], charts.WithItemStyleOpts(itemStyles[i]))
	}

	snippet := bp.RenderSnippet()
	err := os.WriteFile(outputPath+".json", []byte(snippet.Option), 0o644)
	if err != nil {
		return err
	}
	file, err := os.Create(outputPath + ".html")
	if err != nil {
		return err
	}
	err = bp.Render(file)
	if err != nil {
		return err
	}
	return nil
}

func createBoxPlotData(data []float64) []float64 {
	if len(data) == 0 {
		return nil
	}
	sort.Float64s(data)
	return []float64{
		floats.Min(data),
		stat.Quantile(0.25, stat.Empirical, data, nil),
		stat.Mean(data, nil),
		stat.Quantile(0.75, stat.Empirical, data, nil),
		floats.Max(data),
	}
}
