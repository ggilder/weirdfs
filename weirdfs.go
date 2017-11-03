package main

import (
	"flag"
	"fmt"
	"github.com/AlekSi/xattr"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
	"unsafe"
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
	".fseventsd",
	".Trashes",
	".Spotlight-V100",
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
	"Desktop DB",
	"Desktop DF",
}

var resourceForkRequired = "[WARNING] unreadable without resource fork"
var resourceForkOld = "[WARNING] old files may require resource fork"
var resourceForkTypeWarnings = map[string]string{
	".mov":  resourceForkOld,
	".psd":  resourceForkOld,
	".sd2":  resourceForkRequired,
	".sd2f": resourceForkRequired,
}

var illegalPathnameChars = []rune{
	':',
	'/',
	'\\',
}

var illegalTrailingChars = []rune{
	'.',
	' ',
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

func evaluateXattrs(path string, info os.FileInfo, attrs []string, report *map[string]int) (logs, warns []string) {
	if len(attrs) > 0 {
		logs = append(logs, fmt.Sprintf("xattrs: %s", strings.Join(attrs, ", ")))
	}
	for _, attr := range attrs {
		if attr == "com.apple.ResourceFork" {
			ext := strings.ToLower(filepath.Ext(path))
			if ext == "" {
				ext = "(no extension)"
			}
			(*report)[ext]++
			rsrc, err := xattr.Get(path, attr)
			check(err)
			if info.Size() == 0 {
				warns = append(warns, fmt.Sprintf("Data fork is empty; resource fork may contain all data (%d).", len(rsrc)))
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
	lastRune, _ := utf8.DecodeLastRuneInString(base)
	for _, illegalRune := range illegalTrailingChars {
		if lastRune == illegalRune {
			warns = append(warns, fmt.Sprintf("Name ends with illegal character '%c'.", illegalRune))
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

func copyStrippedFile(path string, info os.FileInfo, attrs []string, dest string, ignoredExtensions []string) ([]string, int) {
	logs := []string{}
	count := 0
	for _, attr := range attrs {
		if attr == "com.apple.ResourceFork" {
			fileExt := strings.ToLower(filepath.Ext(path))
			for _, ext := range ignoredExtensions {
				if fileExt == ext {
					return logs, count
				}
			}
			rsrc, err := xattr.Get(path, attr)
			if err != nil || len(rsrc) == 0 {
				return logs, count
			}

			destPath := filepath.Join(dest, strings.Replace(path, "/", "__", -1))
			data, err := ioutil.ReadFile(path)
			check(err)
			err = ioutil.WriteFile(destPath, data, 0644)
			check(err)
			return append(logs, fmt.Sprintf("Copied data-only version to %s", destPath)), 1
		}
	}
	return logs, count
}

func log(msg, level string) {
	fmt.Printf("    [%s] %s\n", strings.ToUpper(level), msg)
}

func logMany(msgs []string, level string) {
	for _, msg := range msgs {
		log(msg, level)
	}
}

func printStatusLine(msg string) {
	var dimensions [4]uint16

	// probably not very efficient to make this syscall every time but oh well!
	if _, _, err := syscall.Syscall6(
		syscall.SYS_IOCTL,
		uintptr(syscall.Stdin),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(&dimensions)),
		0,
		0,
		0,
	); err != 0 {
		// ignore error
		return
	}

	width := int(dimensions[1])
	// pad to width if shorter, then truncate if longer
	msg = fmt.Sprintf("%- "+strconv.Itoa(width)+"s", msg)
	fmt.Printf("%s\r", msg[:width-1])
}

func main() {
	debug := flag.Bool("debug", false, "Output extra debugging info")
	stripResourceForks := flag.Bool("stripResourceForks", false, "Make a data-only copy of files with resource forks for manual analysis")
	stripResourceSkip := flag.String("stripResourceSkip", "", "Comma-separated list of file extensions to exclude from manual analysis, e.g. 'crw,jpg'")
	warnOnCreationTimes := flag.Bool("warnOnCreationTimes", false, "Print warnings on files with creation times that vary from modification times by more than 1 day")
	flag.Parse()

	dir := flag.Arg(0)
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		check(err)
	}

	var strippedDir string = ""
	stripResourceIgnoredExtensions := []string{}
	strippedFilesCount := 0
	if *stripResourceForks {
		usr, err := user.Current()
		check(err)
		strippedDir, err = ioutil.TempDir(usr.HomeDir, "stripped_files")
		check(err)
		for _, ext := range strings.Split(*stripResourceSkip, ",") {
			ext := strings.TrimSpace(strings.ToLower(ext))
			if ext == "" {
				continue
			}
			if string([]rune(ext)[0]) != "." {
				ext = "." + ext
			}
			stripResourceIgnoredExtensions = append(stripResourceIgnoredExtensions, ext)
		}
	}

	if *debug {
		debugMsg("Scanning %s", dir)
		if *stripResourceForks {
			debugMsg("Copying data forks to %s for analyis", strippedDir)
			if len(stripResourceIgnoredExtensions) > 0 {
				debugMsg("Ignoring extensions: %v", stripResourceIgnoredExtensions)
			}
		}
	}

	scannedFiles := 0
	scannedDirs := 0
	resourceForkTypes := map[string]int{}
	rawScanned := 0
	scanErrors := 0

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		rawScanned++

		// Check ignored list before errors to avoid reporting errors on stuff we would ignore anyway
		if isIgnoredFile(filepath.Base(path)) {
			printStatusLine(fmt.Sprintf("%d: (ignored file)", rawScanned))
			return nil
		}

		if isIgnoredPath(path) {
			printStatusLine(fmt.Sprintf("%d: (ignored path)", rawScanned))
			return nil
		}

		if err != nil {
			scanErrors++
			printStatusLine("")
			fmt.Println(path)
			log(err.Error(), "error")
			return nil
		}

		if info.Mode().IsRegular() || info.Mode().IsDir() {
			printStatusLine(fmt.Sprintf("%d: %s", rawScanned, path))

			if info.Mode().IsRegular() {
				scannedFiles++
			} else {
				scannedDirs++
			}

			logs, warns := checkBasename(path, info)
			errors := []string{}

			names, err := xattr.List(path)
			if err != nil {
				errors = append(errors, err.Error())
			}

			names = removeIgnoredXattrs(names)
			logs2, warns2 := evaluateXattrs(path, info, names, &resourceForkTypes)
			logs = append(logs, logs2...)
			warns = append(warns, warns2...)

			if *stripResourceForks {
				logs2, copied := copyStrippedFile(path, info, names, strippedDir, stripResourceIgnoredExtensions)
				strippedFilesCount += copied
				logs = append(logs, logs2...)
			}

			if *warnOnCreationTimes {
				stat := info.Sys().(*syscall.Stat_t)
				birthtime := time.Unix(stat.Birthtimespec.Sec, stat.Birthtimespec.Nsec)
				if info.ModTime().Sub(birthtime).Hours() > 24 {
					warns = append(warns, fmt.Sprintf("Significant creation time: %v vs. %v", birthtime, info.ModTime()))
				}
			}

			if len(warns) > 0 || len(errors) > 0 {
				printStatusLine("")
				fmt.Println(path)
				logMany(errors, "error")
				logMany(warns, "warn")
				logMany(logs, "info")
			} else if *debug {
				if len(logs) > 0 {
					printStatusLine("")
					debugMsg("%s", path)
					logMany(logs, "info")
				}
			}

		}

		return nil
	})

	check(err)

	// clear status line
	printStatusLine("")
	fmt.Printf("\nScanned %d directories and %d files. %d scan errors.\n", scannedDirs, scannedFiles, scanErrors)
	if len(resourceForkTypes) > 0 {
		fmt.Println("\nTypes with resource forks (lowercased):")
		exts := make([]string, len(resourceForkTypes))
		i := 0
		for ext, _ := range resourceForkTypes {
			exts[i] = ext
			i++
		}
		sort.Strings(exts)
		for _, ext := range exts {
			count := resourceForkTypes[ext]
			warning := resourceForkTypeWarnings[ext]
			fmt.Printf("    %s: %d   %s\n", ext, count, warning)
		}
	}
	if *stripResourceForks {
		fmt.Printf("\nStripped resource forks from %d files in %s for analysis.\n", strippedFilesCount, strippedDir)
	}
}
