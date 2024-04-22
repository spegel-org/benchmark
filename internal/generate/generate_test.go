package generate

import (
	"testing"

	"github.com/c2h5oh/datasize"
	"github.com/stretchr/testify/require"
)

func TestLayerSize(t *testing.T) {
	layerSize, err := layerSize(4, 1*datasize.GB)
	require.NoError(t, err)
	require.Equal(t, int64(268435456), layerSize)
}
