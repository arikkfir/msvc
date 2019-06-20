package msvc

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestAdapter(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		require.Panics(t, func() { _ = NewAdapter(nil) })
	})
}
