package main

import (
	"os"

	"github.com/dariuszporowski/mdfmv/cmd/mdfmv"
)

func main() {
	code := mdfmv.Execute()
	//nolint: revive // exiting in main is correct
	os.Exit(code)
}
