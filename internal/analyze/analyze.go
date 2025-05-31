package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"gonum.org/v1/plot/vg/draw"
	"gonum.org/v1/plot/vg/vgimg"

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
		outputPath = fmt.Sprintf("%s.png", outputPath)
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

	plots := []*plot.Plot{}
	w := vg.Points(30)
	for i := range len(baseline.Benchmarks) {
		if len(baseline.Benchmarks[i].Measurements) != len(variant.Benchmarks[i].Measurements) {
			return errors.New("benchmarks cant have different measurement counts")
		}
		if baseline.Benchmarks[i].Image != variant.Benchmarks[i].Image {
			return errors.New("benchmark images are not the same")
		}

		p := plot.New()
		p.Title.Text = baseline.Benchmarks[i].Image
		p.Title.Padding = vg.Points(10)
		p.Y.Label.Text = "Pull Time (seconds)"

		durations := plotter.Values{}
		for _, meas := range baseline.Benchmarks[i].Measurements {
			durations = append(durations, float64(meas.Duration.Seconds()))
		}
		boxPlot, err := plotter.NewBoxPlot(w, float64(0), durations)
		if err != nil {
			return err
		}
		boxPlot.FillColor = color.RGBA{R: 30, G: 144, B: 255, A: 255}
		p.Add(boxPlot)

		durations = plotter.Values{}
		for _, meas := range variant.Benchmarks[i].Measurements {
			durations = append(durations, float64(meas.Duration.Seconds()))
		}
		boxPlot, err = plotter.NewBoxPlot(w, float64(1), durations)
		if err != nil {
			return err
		}
		boxPlot.FillColor = color.RGBA{R: 220, G: 20, B: 60, A: 255}
		p.Add(boxPlot)

		p.NominalX("Baseline", "Spegel")
		plots = append(plots, p)
	}

	width := vg.Points(500)
	height := vg.Points(500)
	img := vgimg.New(width, height)
	dc := draw.New(img)

	// Draw shared title.
	titleStyle := draw.TextStyle{
		Font:    plot.DefaultFont,
		Color:   color.Black,
		XAlign:  draw.XCenter,
		YAlign:  draw.YTop,
		Handler: plot.DefaultTextHandler,
	}
	titleStyle.Font.Size = vg.Points(12)
	titlePoint := vg.Point{X: width / 2, Y: height - vg.Millimeter}
	dc.FillText(titleStyle, titlePoint, "Image Pull Duration")

	// Draw plots.
	t := draw.Tiles{
		Rows:      1,
		Cols:      len(plots),
		PadX:      vg.Points(40),
		PadY:      vg.Millimeter,
		PadTop:    vg.Points(35),
		PadBottom: vg.Points(5),
		PadLeft:   vg.Points(20),
		PadRight:  vg.Points(20),
	}
	canv := plot.Align([][]*plot.Plot{plots}, t, dc)
	for i, plot := range plots {
		plot.Draw(canv[0][i])
	}

	file, err := os.Create(outputPath)
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
