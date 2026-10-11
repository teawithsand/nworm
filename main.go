package main

import (
	"os"

	"github.com/teawithsand/nworm/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
