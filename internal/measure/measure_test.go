package measure

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParsePullMessage(t *testing.T) {
	t.Parallel()

	s := "Successfully pulled image \"docker.io/library/nginx:mainline-alpine\" in 873.420598ms (873.428863ms including waiting)"
	d, err := parsePullMessage(s)
	require.NoError(t, err)
	require.Equal(t, 873420598*time.Nanosecond, d)
}
