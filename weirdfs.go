package main

import (
	"flag"
	"fmt"
	"github.com/AlekSi/xattr"
	"os"
	"path/filepath"
)

var defaultIgnoredFiles = []string{
	".DS_Store",
}

var defaultIgnoredXattrs = []string{
	"com.apple.FinderInfo",
	"com.apple.TextEncoding",
	"com.apple.diskimages.recentcksum",
	"com.apple.metadata:_kTimeMachineNewestSnapshot",
	"com.apple.metadata:_kTimeMachineOldestSnapshot",
	"com.apple.metadata:com_apple_backup_excludeItem",
	"com.apple.metadata:kMDItemFinderComment",
	"com.apple.metadata:kMDItemIsScreenCapture",
	"com.apple.metadata:kMDItemScreenCaptureType",
	"com.apple.metadata:kMDItemWhereFroms",
	"com.apple.quarantine",
	"com.dropbox.attributes",
	"com.macromates.bookmarked_lines",
	"com.macromates.caret",
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func debugMsg(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func isIgnoredFile(basename string) bool {
	for _, f := range defaultIgnoredFiles {
		if basename == f {
			return true
		}
	}
	return false
}

func removeIgnoredXattrs(attrs []string) []string {
	filtered := []string{}
	for _, attr := range attrs {
		isIgnored := false
		for _, ignored := range defaultIgnoredXattrs {
			if attr == ignored {
				isIgnored = true
				break
			}
		}
		if !isIgnored {
			filtered = append(filtered, attr)
		}
	}
	return filtered
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

		if isIgnoredFile(filepath.Base(path)) {
			return nil
		}

		if info.Mode().IsRegular() || info.Mode().IsDir() {
			names, err := xattr.List(path)
			check(err)
			names = removeIgnoredXattrs(names)

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
