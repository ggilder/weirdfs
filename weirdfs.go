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
	// Garageband files
	"PkgInfo",
	"projectData",
	// Logic files
	"displayState",
	"documentData",
	// Icon with ^M at the end
	string([]byte{0x49, 0x63, 0x6f, 0x6e, 0x0d}),
}

var defaultIgnoredPaths = []string{
	".git",
	".svn",
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

var defaultAllowedNamesWithoutFileExtension = []string{
	"CHANGELOG",
	"Capfile",
	"Gemfile",
	"LICENCE",
	"LICENSE",
	"MIT-LICENSE",
	"README",
	"TODO",
	"crontab",
}

var illegalPathnameChars = []rune{
	':',
	'/',
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

func isIgnoredPath(path string) bool {
	parts := strings.Split(path, "/")
	for _, part := range parts {
		for _, ignored := range defaultIgnoredPaths {
			if part == ignored {
				return true
			}
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
			rsrc, err := xattr.Get(path, attr)
			check(err)
			if info.Size() == 0 {
				warns = append(warns, fmt.Sprintf("Data fork is empty; resource fork may contain all data (%d).", len(rsrc)))
				// } else if info.Size() < int64(len(rsrc)) {
				// 	warns = append(warns, fmt.Sprintf("Resource fork is larger than data fork (%d vs. %d).", len(rsrc), info.Size()))
			}
		}
	}
	return logs, warns
}

func checkBasename(path string, info os.FileInfo) (logs, warns []string) {
	base := filepath.Base(path)
	for _, char := range illegalPathnameChars {
		if strings.IndexRune(base, char) > -1 {
			warns = append(warns, fmt.Sprintf("Name contains illegal character '%c'.", char))
		}
	}
	if info.Mode().IsRegular() && filepath.Ext(path) == "" {
		allowed := false
		for _, name := range defaultAllowedNamesWithoutFileExtension {
			if base == name {
				allowed = true
				break
			}
		}
		if !allowed {
			warns = append(warns, "Missing file extension.")
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

		if isIgnoredPath(path) {
			return nil
		}

		if info.Mode().IsRegular() || info.Mode().IsDir() {
			if info.Mode().IsRegular() {
				scannedFiles++
			} else {
				scannedDirs++
			}

			logs, warns := checkBasename(path, info)

			names, err := xattr.List(path)
			check(err)
			names = removeIgnoredXattrs(names)
			logs2, warns2 := evaluateXattrs(path, info, names)
			logs = append(logs, logs2...)
			warns = append(warns, warns2...)

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
