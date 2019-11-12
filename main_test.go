package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderAuthorizedKeys(t *testing.T) {
	var TestKeys = []Key{
		Key{
			"comment-1",
			"ssh-rsa aaaabbbbcccc",
		},
		Key{
			"comment-2",
			"ssh-rsa ddddeeeeffffgggg",
		},
	}

	b := RenderAuthorizedKeys(TestKeys)

	expected := "# comment-1\nssh-rsa aaaabbbbcccc\n# comment-2\nssh-rsa ddddeeeeffffgggg\n"
	assert.Equal(t, expected, string(b), "rendered authorized_keys correctly")
}
