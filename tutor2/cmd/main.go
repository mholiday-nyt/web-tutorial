package main

import (
	"os"
	"tutor2"
)

func main() {
	os.Exit(tutor2.RunApp(os.Args[1:]))
}
