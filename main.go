package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/bsm/extsort"
	"github.com/fatih/color"
	art "github.com/plar/go-adaptive-radix-tree"
	sortutils "github.com/twotwotwo/sorts"
)

//Version populated upon build by goreleaser or the Makefile
var Version = ""
var linecountoutputat uint64 = 1000000 // every million lines output, print if verbose mode

type filesFlag []string

func (i *filesFlag) String() string {
	return "junkplaceholder"
}

func (i *filesFlag) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
}

func scanRunes(data []byte, atEOF bool) (advance int, token []byte, err error) {
	start := 0
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	end := start

	for width, i := 0, start; i < len(data); i += width {
		var r rune
		r, width = utf8.DecodeRune(data[i:])
		if r == '\n' {
			return i + width, data[start:i], nil
		}
		end = i + 1
	}

	if atEOF && len(data) > start {
		return len(data), data[start:end], nil
	}

	return 0, nil, nil
}

func methodRadix(maxLineLength int, fileList filesFlag, file *os.File, verbose bool) {

	if verbose {
		color.Green("[+] Starting method radix")
	}

	sorter := art.New()
	var linecount uint64 = 0
	var total uint64 = 0

	for _, filename := range fileList {

		filein, err := os.Open(filename)

		if err != nil {
			color.Red(fmt.Sprintf("[!]\tunable to read %s - %v\n", filename, err))
			return
		}
		defer filein.Close()

		var maxlen int = 13000

		if maxLineLength != -1 {
			maxlen = maxLineLength
		}

		buffer := make([]byte, 0, maxlen)
		scanner := bufio.NewScanner(filein)
		scanner.Split(scanRunes)
		scanner.Buffer(buffer, maxlen)

		if verbose {
			color.Green(fmt.Sprintf("[+]\tReading %s with max line length of %d\n", filename, maxlen))
			printMemStats()
		}

		for scanner.Scan() {
			sorter.Insert(scanner.Bytes(), nil)
			linecount++
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
	}

	if verbose {
		color.Green("[+]\tDone reading files - saving output")
		printMemStats()
	}

	total = linecount
	var nextupdate uint64 = linecount

	for it := sorter.Iterator(); it.HasNext(); {
		node, _ := it.Next()

		if _, err := file.Write(append(node.Key(), "\n"...)); err != nil {
			color.Red("[!] Failed to write line to file for key %s\n", node.Key())
		}

		linecount--

		if verbose && linecount < nextupdate {
			color.Green(fmt.Sprintf("[+]\t%d / %d\n", total, linecount))
			nextupdate = nextupdate - linecountoutputat
		}
	}

	if verbose {
		color.Green("[+] Done with method radix")
		printMemStats()
	}
}

func SortByteArrays(src [][]byte) {
	sort.Slice(src, func(i, j int) bool { return bytes.Compare(src[i], src[j]) < 0 })
}

func methodExtsort(tmpdir string, maxLineLength int, fileList filesFlag, file *os.File, verbose bool) {

	if verbose {
		color.Green("[+] Starting method extsort")
	}
	var filecount uint64 = 0
	var linecount uint64 = 0
	var total uint64 = 0

	if tmpdir == "" {
		d, err := os.MkdirTemp("", "fsort_m_extsort_tmp*")

		tmpdir = d

		if err != nil {
			color.Red(fmt.Sprintf("[!] error: %s - mkdir failed %s\n", err.Error(), tmpdir))
			log.Fatal(err)
		}
	} else {
		err := os.MkdirAll(tmpdir, 0777)

		if err != nil {
			color.Red(fmt.Sprintf("[!] error: %s - mkdir failed %s\n", err.Error(), tmpdir))
			log.Fatal(err)
		}
	}

	defer os.RemoveAll(tmpdir) // clean up

	sorter := extsort.New(&extsort.Options{
		Sort:        sortutils.Quicksort,
		BufferSize:  2 * 1024 * 1024, // 2 GiB buffer - enough? or make dynamic?
		Dedupe:      bytes.Equal,
		WorkDir:     tmpdir,
		Compression: extsort.CompressionSnappy,
	})
	defer sorter.Close()

	for _, filename := range fileList {

		filein, err := os.Open(filename)

		if err != nil {
			color.Red(fmt.Sprintf("[!]\tunable to read %s - %v\n", filename, err))
			return
		}

		defer filein.Close()
		var maxlen int = 13000

		if maxLineLength != -1 {
			maxlen = maxLineLength
		}

		buffer := make([]byte, 0, maxlen)
		scanner := bufio.NewScanner(filein)
		scanner.Split(scanRunes)
		scanner.Buffer(buffer, maxlen)

		if verbose {
			color.Green(fmt.Sprintf("[+]\tReading %s with max line length of %d\n", filename, maxlen))
			printMemStats()
		}

		for scanner.Scan() {
			_ = sorter.Put(scanner.Bytes(), nil)
			filecount++
			linecount++
		}

		if err := scanner.Err(); err != nil {
			color.Red(fmt.Sprintf("[!] error: %s - on %s:%d\n", err.Error(), filename, filecount))
			panic(err)
		}

		filecount = 0
	}

	if verbose {
		color.Green("[+]\tDone reading files - sorting data")
		printMemStats()
	}

	iter, err := sorter.Sort()
	if err != nil {
		panic(err)
	}
	defer iter.Close()

	if verbose {
		color.Green("[+]\tDone sorting data - saving output")
		printMemStats()
	}

	total = linecount
	var nextupdate uint64 = linecount

	for iter.Next() {

		if _, err := file.Write(append(iter.Key(), "\n"...)); err != nil {
			color.Red("[!] Failed to write line to file for key %s", iter.Key())
		}
		linecount--

		if verbose && linecount < nextupdate {
			color.Green(fmt.Sprintf("[+]\t%d / %d\n", total, linecount))
			nextupdate = nextupdate - linecountoutputat
		}
	}

	if err := iter.Err(); err != nil {
		panic(err)
	}

	if verbose {
		color.Green("[+] Done with method extsort")
		printMemStats()
	}
}

func main() {

	var fileList filesFlag

	flag.Var(&fileList, "file", "Merge and Sort this file as well")
	maxLineLength := flag.Int("maxlen", -1, "There must not be a line is longer than this")
	method := flag.String("method", "extsort", "The method to use for merge/sort: options are radix,extsort - radix is faster, but uses more memory - extsort is slower but uses less memory (many large files, this is your option)")
	outfile := flag.String("out", "", "The result of merging and sorting the files provided")
	tmpdir := flag.String("tmpdir", "", "Use this location as the base directory (when using extsort)")
	version := flag.Bool("version", false, "Display the version and exit")
	verbose := flag.Bool("verbose", false, "Display memory usage")

	flag.Usage = usage
	flag.Parse()

	if *version {
		color.Blue(fmt.Sprintf("fsort %s", Version))
		os.Exit(0)
	}

	if len(fileList) <= 0 {
		color.Red("[!] You must specify atleast one -file argument")
		usage()
	}

	if *outfile == "" {
		*outfile = *method

		for _, filename := range fileList {
			base := filepath.Base(filename)
			ext := filepath.Ext(filename)

			if ext != "" {
				*outfile = *outfile + "_" + strings.Split(base, ext)[0]
			} else {
				*outfile = *outfile + "_" + base
			}
		}
	}

	color.HiWhite(fmt.Sprintf("[+] Saving merged files to: %s\n", *outfile))

	file, err := os.Create(*outfile)

	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	if *method == "radix" {
		methodRadix(*maxLineLength, fileList, file, *verbose)
	} else if *method == "extsort" {
		methodExtsort(*tmpdir, *maxLineLength, fileList, file, *verbose)
	} else {
		color.Red(fmt.Sprintf("[!] bad method %s\n", *method))
		usage()
	}

	runtime.GC()
}

func usage() {
	color.HiWhite("fsort Usage\r\n")
	flag.PrintDefaults()
	os.Exit(0)
}

func printMemStats() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	color.HiYellow(fmt.Sprintf("[*]\tSys = %v MiB / %v MiB\n", m.Sys/1024/1024, m.Alloc/1024/1024))
}
