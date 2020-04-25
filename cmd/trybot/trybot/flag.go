package trybot

import (
	"flag"
	"fmt"
	"os"
)

var flagSet = flag.NewFlagSet("trybot", flag.ContinueOnError)

func init() { flagSet.Usage = usage }

func usage() {
	fmt.Fprintf(os.Stderr, `
Usage of cueckoobot:

	cueckoobot (start|end)

`[1:])
	flagSet.PrintDefaults()
}

type usageErr string

func (u usageErr) Error() string { return string(u) }

type flagErr string

func (f flagErr) Error() string { return string(f) }

func Main() { os.Exit(main1()) }

func main1() int {
	err := mainerr()
	if err == nil {
		return 0
	}
	switch err.(type) {
	case usageErr:
		fmt.Fprintln(os.Stderr, err)
		flagSet.Usage()
		return 2
	case flagErr:
		return 2
	}
	fmt.Fprintln(os.Stderr, err)
	return 1
}
