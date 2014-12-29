package main

import (
	"flag"
	"fmt"
	"github.com/AlekSi/xattr"
	"os"
	"path/filepath"
	"strings"
)

var defaultIgnoredFiles = []string{
	".DS_Store",
}

var defaultIgnoredXattrs = []string{
	"com.apple.FinderInfo",
	"com.apple.Preview.UIstate.v1",
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

func evaluateXattrs(path string, info os.FileInfo, attrs []string) (logs, warns []string) {
	if len(attrs) > 0 {
		logs = append(logs, fmt.Sprintf("xattrs: %s", strings.Join(attrs, ", ")))
	}
	for _, attr := range attrs {
		if attr == "com.apple.ResourceFork" {
			if filepath.Ext(path) == "" {
				warns = append(warns, "No file extension, resource fork may contain file type.")
			}
			rsrc, err := xattr.Get(path, attr)
			check(err)
			if info.Size() == 0 {
				warns = append(warns, fmt.Sprintf("Data fork is empty; resource fork may contain all data (%dB).", len(rsrc)))
			}
		}
	}
	return logs, warns
}

func log(msg, level string) {
	fmt.Printf("    [%s] %s\n", strings.ToUpper(level), msg)
}

func logMany(msgs []string, level string) {
	for _, msg := range msgs {
		log(msg, level)
	}
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

	scannedFiles := 0
	scannedDirs := 0

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if isIgnoredFile(filepath.Base(path)) {
			return nil
		}

		if info.Mode().IsRegular() || info.Mode().IsDir() {
			if info.Mode().IsRegular() {
				scannedFiles++
			} else {
				scannedDirs++
			}

			names, err := xattr.List(path)
			check(err)
			names = removeIgnoredXattrs(names)
			logs, warns := evaluateXattrs(path, info, names)

			if len(warns) > 0 {
				fmt.Println(path)
				logMany(warns, "warn")
				logMany(logs, "info")
			} else if *debug {
				if len(logs) > 0 {
					debugMsg("%s", path)
					logMany(logs, "info")
				}
			}
		}

		return nil
	})

	fmt.Printf("\nScanned %d directories and %d files.\n", scannedDirs, scannedFiles)
}
