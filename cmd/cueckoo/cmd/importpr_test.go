package cmd

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestAddCloses(t *testing.T) {
	cases := []struct {
		name string
		in   string
		out  string
		pr   int
	}{
		{
			name: "no trailer",
			in: `first line

My commit message with no trailer

`,
			out: "first line\n\nMy commit message with no trailer\n\nCloses #0 as merged.\n",
		},
		{
			name: "signed-off-by trailer",
			in: `first line

My commit message with no trailer

Signed-off-by: Paul

`,
			out: "first line\n\nMy commit message with no trailer\n\nCloses #0 as merged.\n\nSigned-off-by: Paul\n",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := addClosesMsg(c.in, c.pr)
			if err != nil {
				t.Fatalf("got error when none expected: %v", err)
			}
			t.Log("\n" + got)
			if got != c.out {
				t.Logf("got: %q", got)
				t.Error(cmp.Diff(c.out, got))
			}
		})
	}
}
