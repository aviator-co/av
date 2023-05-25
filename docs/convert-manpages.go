// Binary convert-manpages converts specified man page Markdown files.
package main

import (
	"bytes"
	"errors"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/cpuguy83/go-md2man/v2/md2man"
	"github.com/spf13/pflag"
)

var (
	preview   = pflag.Bool("preview", false, "Preview the converted man page")
	outputDir = pflag.String("output-dir", "", "Output directory")

	manpageMarkdownPattern = regexp.MustCompile(`[.](\d)[.]md`)
)

func main() {
	pflag.Parse()

	if *outputDir == "" {
		// If the output directory is not specified, assume it's for preview.
		*preview = true
	}

	if *preview {
		args := pflag.Args()
		if len(args) != 1 {
			pflag.Usage()
			os.Exit(1)
			return
		}
		previewMarkdown(args[0])
		return
	}

	wd, err := os.Getwd()
	if err != nil {
		log.Fatalf("Cannot get the current wd: %v", err)
	}
	ents, err := os.ReadDir(wd)
	if err != nil {
		log.Fatal(err)
	}

	for _, ent := range ents {
		matches := manpageMarkdownPattern.FindStringSubmatch(ent.Name())
		if len(matches) == 0 {
			continue
		}
		bs, err := os.ReadFile(ent.Name())
		if err != nil {
			log.Fatalf("Cannot read a file %q: %v", ent.Name(), err)
		}
		roff := convertMarkdown(bs)
		outFilePath := filepath.Join(
			*outputDir,
			"man"+matches[1],
			strings.TrimSuffix(ent.Name(), ".md"),
		)
		if err := os.MkdirAll(filepath.Dir(outFilePath), 0755); err != nil {
			log.Fatalf("Cannot create the output directory: %v", err)
		}
		if err := os.WriteFile(outFilePath, roff, 0644); err != nil {
			log.Fatalf("Cannot write the conversion result to %q: %v", outFilePath, err)
		}
	}
}

func previewMarkdown(fp string) error {
	bs, err := os.ReadFile(fp)
	if err != nil {
		return err
	}

	roff := convertMarkdown(bs)
	_ = roff

	if runtime.GOOS == "darwin" || runtime.GOOS == "freebsd" {
		cmd := exec.Command("mandoc", "-a")
		cmd.Stdin = bytes.NewBuffer(roff)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	} else if runtime.GOOS == "linux" {
		cmd := exec.Command("man", "-l", "-")
		cmd.Stdin = bytes.NewBuffer(roff)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return errors.New("operating system not supported for preview")
}

func convertMarkdown(bs []byte) []byte {
	return md2man.Render(bs)
}
