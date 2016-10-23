package main

import (
	"os"

	"github.com/Songmu/go-memcached-tool"
)

func main() {
	os.Exit((&memdtool.CLI{ErrStream: os.Stderr, OutStream: os.Stdout}).Run(os.Args[1:]))
}
