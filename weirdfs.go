package main

import (
	"flag"
	"fmt"
	"github.com/AlekSi/xattr"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
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
	"com.dropbox.attrs",
}

var defaultAllowedNamesWithoutFileExtension = []string{
	"Capfile",
	"Gemfile",
	"Rakefile",
	"Procfile",
	"CHANGELOG",
	"LICENCE",
	"LICENSE",
	"MIT-LICENSE",
	"README",
	"TODO",
	"VERSION",
	"INSTALL",
	"crontab",
	"Desktop DB",
	"Desktop DF",
	// From old DVD Studio Pro projects
	"ModuleDataB",
	"ObjectDataB",
}

var resourceForkRequired = "[WARNING] unreadable without resource fork"
var resourceForkOld = "[WARNING] old files may require resource fork"
var resourceForkTypeWarnings = map[string]string{
	".disc":         resourceForkRequired,
	".mov":          resourceForkOld, // TODO are there really MOV files that require resource fork?
	".psd":          resourceForkOld, // TODO are there really PSD files that require resource fork?
	".sd2":          resourceForkRequired,
	".sd2f":         resourceForkRequired,
	".textclipping": resourceForkRequired,
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

var derezResourceType = regexp.MustCompile("(?m:^data '(.{4})')")

// Added pi symbol for old RealBasic and GoLive files
var validFileExtension = regexp.MustCompile("^\\.[a-z0-9π\\-]+$")

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func debugMsg(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
}

func uniqueStrings(input []string) []string {
	u := make([]string, 0, len(input))
	m := make(map[string]bool)

	for _, val := range input {
		if _, ok := m[val]; !ok {
			m[val] = true
			u = append(u, val)
		}
	}

	return u
}

func strictFileExtension(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if !validFileExtension.MatchString(ext) {
		return ""
	}

	return ext
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

func evaluateXattrs(path string, info os.FileInfo, attrs []string, report *map[string]int, resourceReport *map[string][]string) (logs, warns []string) {
	if len(attrs) > 0 {
		logs = append(logs, fmt.Sprintf("xattrs: %s", strings.Join(attrs, ", ")))
	}
	for _, attr := range attrs {
		if attr == "com.apple.ResourceFork" {
			rsrc, err := xattr.Get(path, attr)
			if err != nil {
				warns = append(warns, fmt.Sprintf("Error: %s", err))
			}
			resourceTypes, err := extractResourceTypes(path)
			if err != nil {
				warns = append(warns, fmt.Sprintf("Error: %s", err))
			}
			if len(resourceTypes) > 0 {
				ext := strictFileExtension(path)
				if ext == "" {
					ext = "(no extension)"
				}
				(*report)[ext]++
				(*resourceReport)[ext] = uniqueStrings(append((*resourceReport)[ext], resourceTypes...))
				if info.Size() == 0 {
					warns = append(warns, fmt.Sprintf("Data fork is empty; resource fork may contain all data (%d).", len(rsrc)))
				}
			}
		}
	}
	return logs, warns
}

func extractResourceTypes(path string) ([]string, error) {
	cmdOut, err := exec.Command("DeRez", path).Output()
	if err != nil {
		return nil, err
	}
	out := string(cmdOut)
	matches := derezResourceType.FindAllStringSubmatch(out, -1)
	resources := make(map[string]struct{})
	for _, match := range matches {
		kind := match[1]
		resources[kind] = struct{}{}
	}
	resourceTypes := make([]string, len(resources))
	i := 0
	for kind := range resources {
		resourceTypes[i] = kind
		i++
	}
	sort.Strings(resourceTypes)
	return resourceTypes, nil
}

func checkBasename(path string, info os.FileInfo, allowTextMissingExtension bool) (logs, warns []string) {
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
	if info.Mode().IsRegular() && strictFileExtension(path) == "" {
		for _, name := range defaultAllowedNamesWithoutFileExtension {
			if base == name {
				return logs, warns
			}
		}
		if allowTextMissingExtension && isPlainTextFile(path) {
			return logs, warns
		}
		warns = append(warns, "Missing file extension.")
	}
	return logs, warns
}

func isPlainTextFile(path string) bool {
	cmdOut, err := exec.Command("file", "-b", path).Output()
	check(err)
	out := string(cmdOut)
	if out == "empty\n" {
		return true
	}
	matched, err := regexp.MatchString("\\b((ASCII|Unicode|ISO-8859) text|very short file)\\b", out)
	check(err)
	return matched
}

func copyStrippedFile(path string, info os.FileInfo, attrs []string, dest string, ignoredExtensions []string) ([]string, int) {
	logs := []string{}
	count := 0
	fileExt := strictFileExtension(path)
	for _, ext := range ignoredExtensions {
		if fileExt == ext {
			return logs, count
		}
	}
	for _, attr := range attrs {
		if attr == "com.apple.ResourceFork" {
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
	fmt.Fprintf(os.Stderr, "%s\r", msg[:width-1])
}

func main() {
	debug := flag.Bool("debug", false, "Output extra debugging info")
	stripResourceForks := flag.Bool("stripResourceForks", false, "Make a data-only copy of files with resource forks for manual analysis")
	stripResourceSkip := flag.String("stripResourceSkip", "", "Comma-separated list of file extensions to exclude from manual analysis, e.g. 'crw,jpg'")
	warnOnCreationTimes := flag.Bool("warnOnCreationTimes", false, "Print warnings on files with creation times that vary from modification times by more than 1 day")
	allowTextMissingExtension := flag.Bool("allowTextMissingExtension", false, "Allow plain text files without file extension")
	flag.Parse()

	dir := flag.Arg(0)
	var err error
	if dir == "" {
		dir, err = os.Getwd()
		check(err)
	}
	dir, err = filepath.Abs(dir)
	check(err)

	fmt.Printf("Scanning %s\n", dir)

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
	resourcesByType := make(map[string][]string)
	fileExtensions := map[string]bool{}
	rawScanned := 0
	scanErrors := 0

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if *debug {
			debugMsg("Scanning %s", path)
		}
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
				fileExtensions[strictFileExtension(path)] = true
			} else {
				scannedDirs++
			}

			logs, warns := checkBasename(path, info, *allowTextMissingExtension)
			errors := []string{}

			xattrNames, err := xattr.List(path)
			if err != nil {
				errors = append(errors, err.Error())
			}

			xattrNames = removeIgnoredXattrs(xattrNames)
			logs2, warns2 := evaluateXattrs(path, info, xattrNames, &resourceForkTypes, &resourcesByType)
			logs = append(logs, logs2...)
			warns = append(warns, warns2...)

			if *stripResourceForks {
				logs2, copied := copyStrippedFile(path, info, xattrNames, strippedDir, stripResourceIgnoredExtensions)
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
			sort.Strings(resourcesByType[ext])
			types := "'" + strings.Join(resourcesByType[ext], "', '") + "'"
			fmt.Printf("    %s: %d (%s)   %s\n", ext, count, types, warning)
		}
	}
	if *stripResourceForks {
		fmt.Printf("\nStripped resource forks from %d files in %s for analysis.\n", strippedFilesCount, strippedDir)
	}
	if len(fileExtensions) > 0 {
		fmt.Println("\nFile extensions encountered (lowercased):")
		exts := make([]string, len(fileExtensions))
		i := 0
		for ext, _ := range fileExtensions {
			exts[i] = ext
			i++
		}
		sort.Strings(exts)
		fmt.Println(strings.TrimSpace(strings.Join(exts, " ")))
	}
}
