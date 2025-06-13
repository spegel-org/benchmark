package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/stat"

	"github.com/spegel-org/benchmark/internal/measure"
)

func Analyze(ctx context.Context, suitePaths []string, outputDir string) error {
	suites := []measure.Suite{}
	for _, path := range suitePaths {
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		suite := measure.Suite{}
		err = json.Unmarshal(b, &suite)
		if err != nil {
			return err
		}
		suites = append(suites, suite)
	}
	if len(suites) == 0 {
		return errors.New("suites is empty")
	}

	err := os.MkdirAll(outputDir, 0o755)
	if err != nil {
		return err
	}

	for k := range suites[0].Benchmarks {
		benchmarks := []measure.Benchmark{}
		suiteNames := []string{}
		for j := range suites {
			benchmarks = append(benchmarks, suites[j].Benchmarks[k])
			suiteNames = append(suiteNames, suites[j].Name)
		}
		err := createBoxPlot(benchmarks, suiteNames, outputDir, k)
		if err != nil {
			return err
		}
	}
	return nil
}

func createBoxPlot(benchmarks []measure.Benchmark, suiteNames []string, outputDir, benchmarkName string) error {
	bp := charts.NewBoxPlot()
	bp.SetGlobalOptions(
		charts.WithYAxisOpts(opts.YAxis{Name: "Duration (seconds)", NameLocation: "middle", NameGap: 40}),
		charts.WithLegendOpts(opts.Legend{Left: "70%"}),
		charts.WithAnimation(false),
	)
	bp.SetXAxis([]string{"Create", "Update"})

	itemStyles := []opts.ItemStyle{
		{BorderColor: "#164577", Color: "#9CC1E3"},
		{BorderColor: "#FAA93B", Color: "#FAEAD4"},
	}
	for i, v := range benchmarks {
		initialDurations := []float64{}
		for _, sample := range v.Create.Samples {
			initialDurations = append(initialDurations, sample.Duration.Seconds())
		}

		rollingDurations := []float64{}
		for _, sample := range v.Update.Samples {
			rollingDurations = append(rollingDurations, sample.Duration.Seconds())
		}

		data := []opts.BoxPlotData{
			{Value: createBoxPlotData(initialDurations), Name: suiteNames[i]},
			{Value: createBoxPlotData(rollingDurations), Name: suiteNames[i]},
		}
		bp.AddSeries(suiteNames[i], data, charts.WithItemStyleOpts(itemStyles[i]))
	}

	snippet := bp.RenderSnippet()
	err := os.WriteFile(filepath.Join(outputDir, benchmarkName+".json"), []byte(snippet.Option), 0o644)
	if err != nil {
		return err
	}
	file, err := os.Create(filepath.Join(outputDir, benchmarkName+".html"))
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
