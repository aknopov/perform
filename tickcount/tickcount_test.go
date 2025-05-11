package tickcount

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTickCount(t *testing.T) {
	assertT := assert.New(t)

	tc1 := TickCount()
	assertT.GreaterOrEqual(tc1, uint64(0))
	tc2 := TickCount()
	assertT.GreaterOrEqual(tc2, tc1)
}
