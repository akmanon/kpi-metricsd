package main

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTailAndRedirect(t *testing.T) {
	srcF := "app_test.log"
	dstF := "app_test_redirect.log"
	_, err := os.Create(srcF)

	assert.NoError(t, err)
	tr := NewTailAndRedirect(srcF, dstF, context.Background())
	err = tr.Start()
	assert.NoError(t, err)
}
