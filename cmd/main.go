package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hjhsamuel/magent/app"
)

var (
	Version   = "dev"
	CommitID  = ""
	BuildTime = ""
)

func printVersion() {
	fmt.Printf(`MAgent %s 
CommitID: %s
BuildTime: %s\n`, Version, CommitID, BuildTime)
}

func main() {
	var (
		showVersion = flag.Bool("v", false, "show version")
	)
	flag.Parse()

	printVersion()

	if *showVersion {
		os.Exit(0)
	}

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
