package context

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestViewSizeEnv(t *testing.T) {
	assert.Equal(t, []string{
		"DFT_WIDTH=120",
		"COLUMNS=120",
		"LINES=40",
	}, ViewSizeEnv(120, 40))
}

func TestViewSizeEnvIsNilWhenSizeIsUnknown(t *testing.T) {
	assert.Nil(t, ViewSizeEnv(0, 40))
	assert.Nil(t, ViewSizeEnv(120, 0))
	assert.Nil(t, ViewSizeEnv(-1, -1))
}

func TestFullViewSizeEnvReservesStatusBarRow(t *testing.T) {
	ctx := &MainContext{TerminalWidth: 100, TerminalHeight: 30}
	assert.Equal(t, []string{
		"DFT_WIDTH=100",
		"COLUMNS=100",
		"LINES=29",
	}, ctx.FullViewSizeEnv())
}

func TestFullViewSizeEnvIsNilBeforeFirstWindowSize(t *testing.T) {
	assert.Nil(t, (&MainContext{}).FullViewSizeEnv())
}
