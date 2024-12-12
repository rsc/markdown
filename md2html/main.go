// Copyright 2017 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Md2html converts Markdown to HTML.
//
// Usage:
//
//	md2html [file...]
//
// Md2html reads the named files, or else standard input, as Markdown documents
// and then prints the corresponding HTML to standard output.
package main

import (
	"bytes"
	"flag"
	"io/ioutil"
	"log"
	"os"
	"unicode/utf8"

	"rsc.io/markdown"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		do(os.Stdin)
	} else {
		for _, arg := range args {
			f, err := os.Open(arg)
			if err != nil {
				log.Fatal(err)
			}
			do(f)
			f.Close()
		}
	}
}

func do(f *os.File) {
	data, err := ioutil.ReadAll(f)
	if err != nil {
		log.Fatal(err)
	}
	os.Stdout.WriteString(toHTML(data))
}

// toHTML converts Markdown to HTML.
func toHTML(md []byte) string {
	var p markdown.Parser
	p.Table = true
	return markdown.ToHTML(p.Parse(string(replaceTabs(md))))
}

// replaceTabs replaces all tabs in text with spaces up to a 4-space tab stop.
//
// In Markdown, tabs used for indentation are required to be interpreted as
// 4-space tab stops. See https://spec.commonmark.org/0.30/#tabs.
// Go also renders nicely and more compactly on the screen with 4-space
// tab stops, while browsers often use 8-space.
// Make the Go code consistently compact across browsers,
// all while staying Markdown-compatible, by expanding to 4-space tab stops.
//
// This function does not handle multi-codepoint Unicode sequences correctly.
func replaceTabs(text []byte) []byte {
	var buf bytes.Buffer
	col := 0
	for len(text) > 0 {
		r, size := utf8.DecodeRune(text)
		text = text[size:]

		switch r {
		case '\n':
			buf.WriteByte('\n')
			col = 0

		case '\t':
			buf.WriteByte(' ')
			col++
			for col%4 != 0 {
				buf.WriteByte(' ')
				col++
			}

		default:
			buf.WriteRune(r)
			col++
		}
	}
	return buf.Bytes()
}
