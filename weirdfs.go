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

			rsrcPath := filepath.Join(path, "..namedfork", "rsrc")
			rsrcStat, err := os.Stat(rsrcPath)
			check(err)
			rsrcSize := rsrcStat.Size()

			if len(names) > 0 || rsrcSize > 0 {
				fmt.Println(path)
				for _, name := range names {
					fmt.Printf("    %s\n", name)
				}
				if rsrcSize > 0 {
					fmt.Printf("    Resource fork of %dB\n", rsrcSize)
				}
			} else if *debug {
				debugMsg("%s", path)
			}
		}

		return nil
	})
}
