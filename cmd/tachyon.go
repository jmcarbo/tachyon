package main

import (
	"github.com/jmcarbo/tachyon"
	_ "github.com/jmcarbo/tachyon/net"
	_ "github.com/jmcarbo/tachyon/package"
	_ "github.com/jmcarbo/tachyon/procmgmt"
	"os"
)

var Release string

func main() {
	if Release != "" {
		tachyon.Release = Release
	}

	os.Exit(tachyon.Main(os.Args))
}
