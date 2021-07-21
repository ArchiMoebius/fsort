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
	"strings"

	"github.com/bsm/extsort"
	"github.com/fatih/color"
	art "github.com/plar/go-adaptive-radix-tree"
)

//BuildDate populated upon build by goreleaser or the Makefile
var BuildDate = ""
var linecountoutputat uint64 = 100000

type filesFlag []string

func (i *filesFlag) String() string {
	return "junkplaceholder"
}

func (i *filesFlag) Set(value string) error {
	*i = append(*i, strings.TrimSpace(value))
	return nil
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
		buffer := make([]byte, 0, maxLineLength)
		scanner := bufio.NewScanner(filein)
		scanner.Buffer(buffer, maxLineLength)

		if verbose {
			color.Green(fmt.Sprintf("[+]\tReading %s\n", filename))
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

	for it := sorter.Iterator(); it.HasNext(); {
		node, _ := it.Next()
		file.Write(node.Key())
		file.WriteString("\n")
		linecount--

		if verbose && linecount%linecountoutputat == 0 {
			color.Green(fmt.Sprintf("[+]\t%d / %d\n", total, linecount))
		}
	}

	if verbose {
		color.Green("[+] Done with method radix")
		printMemStats()
	}
}

func methodExtsort(maxLineLength int, fileList filesFlag, file *os.File, verbose bool) {

	if verbose {
		color.Green("[+] Starting method extsort")
	}
	var linecount uint64 = 0
	var total uint64 = 0

	sorter := extsort.New(&extsort.Options{
		Dedupe: bytes.Equal,
	})
	defer sorter.Close()

	for _, filename := range fileList {

		filein, err := os.Open(filename)

		if err != nil {
			color.Red(fmt.Sprintf("[!]\tunable to read %s - %v\n", filename, err))
			return
		}

		defer filein.Close()

		buffer := make([]byte, 0, maxLineLength)
		scanner := bufio.NewScanner(filein)
		scanner.Buffer(buffer, maxLineLength)

		if verbose {
			color.Green(fmt.Sprintf("[+]\tReading %s\n", filename))
		}

		for scanner.Scan() {
			_ = sorter.Put(scanner.Bytes(), nil)
			linecount++
		}

		if err := scanner.Err(); err != nil {
			log.Fatal(err)
		}
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

	for iter.Next() {
		file.Write(iter.Key())
		file.WriteString("\n")
		linecount--

		if verbose && linecount%linecountoutputat == 0 {
			color.Green(fmt.Sprintf("[+]\t%d / %d\n", total, linecount))
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
	maxLineLength := flag.Int("maxlen", 350, "There must not be a line is longer than this")
	method := flag.String("method", "extsort", "The method to use for merge/sort: options are radix,extsort - radix is faster, but uses more memory - extsort is slower but uses less memory (many large files, this is your option)")
	outfile := flag.String("out", "", "The result of merging and sorting the files provided")
	version := flag.Bool("version", false, "Display the version and exit")
	verbose := flag.Bool("verbose", false, "Display memory usage")

	flag.Usage = usage
	flag.Parse()

	if *version {
		color.Blue(fmt.Sprintf("fsort %s", BuildDate))
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
		methodExtsort(*maxLineLength, fileList, file, *verbose)
	} else {
		color.Red(fmt.Sprintf("[!] bad method %s\n", *method))
		usage()
	}
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
