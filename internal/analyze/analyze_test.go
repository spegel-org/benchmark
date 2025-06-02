package analyze

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateBoxPlotData(t *testing.T) {
	t.Parallel()

	data := createBoxPlotData(nil)
	require.Empty(t, data)
	data = createBoxPlotData([]float64{0, 2, 1, 3})
	require.Equal(t, []float64{0, 0, 1.5, 2, 3}, data)
}
