package ptr_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LoneWolfPR/MedMarket/backend/internal/ptr"
)

func TestTo(t *testing.T) {
	p := ptr.To(42)

	require.NotNil(t, p)
	assert.Equal(t, 42, *p)
}

func TestDeref(t *testing.T) {
	t.Run("non-nil returns the pointed-to value", func(t *testing.T) {
		assert.Equal(t, "hello", ptr.Deref(ptr.To("hello")))
	})

	t.Run("nil returns the zero value", func(t *testing.T) {
		var s *string
		assert.Equal(t, "", ptr.Deref(s))

		var n *int
		assert.Equal(t, 0, ptr.Deref(n))
	})
}
