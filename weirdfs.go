package main

import (
	"flag"
	"fmt"
	"github.com/AlekSi/xattr"
	"os"
	"path/filepath"
)

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func debugMsg(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func main() {
	debug := flag.Bool("debug", false, "debug")
	flag.Parse()

	dir := flag.Arg(0)
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		check(err)
	}
	if *debug {
		debugMsg("Scanning %s", dir)
	}

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.Mode().IsRegular() {
			names, err := xattr.List(path)
			check(err)

			if len(names) > 0 {
				fmt.Println(path)
				for _, name := range names {
					fmt.Printf("    %s\n", name)
				}
			} else if *debug {
				debugMsg("%s", path)
			}
		}

		return nil
	})
}
