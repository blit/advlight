package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTemplates(t *testing.T) {
	assert.NotNil(t, templates["index.html"])
}
