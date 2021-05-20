package main

import (
	"os"

	"tutor4"
)

func main() {
	os.Exit(tutor4.RunApp(os.Args[1:]))
}
