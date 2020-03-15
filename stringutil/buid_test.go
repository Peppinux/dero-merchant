package stringutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBuild(t *testing.T) {
	tests := []struct {
		Strings  []string
		Expected string
	}{
		{
			Strings:  []string{"", ""},
			Expected: "",
		},
		{
			Strings:  []string{"foo", "bar"},
			Expected: "foobar"},
		{
			Strings:  []string{"foo ", "bar ", "baz"},
			Expected: "foo bar baz"},
	}

	var actual string
	for _, test := range tests {
		actual = Build(test.Strings...)
		assert.Equal(t, test.Expected, actual)
	}
}
