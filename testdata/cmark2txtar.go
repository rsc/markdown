// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/tools/txtar"
	"rsc.io/markdown"
)

var parsers = map[string]string{
	"example autolink":      `{"AutoLinkText": true, "AutoLinkAssumeHTTP": true}`,
	"example disabled":      `{"TaskListItems": true}`,
	"example strikethrough": `{"Strikethrough": true}`,
	"example table":         `{"Table": true}`,
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("cmark2txtar: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: cmark2txtar file\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	file := flag.Arg(0)

	data, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	a := &txtar.Archive{
		Comment: []byte("// go run cmark2txtar.go " + file + "\n"),
	}

	var p markdown.Parser
	doc := p.Parse(string(data))
	n := 0
	for _, b := range doc.Blocks {
		var in, out []string
		b, ok := b.(*markdown.CodeBlock)
		if !ok || !strings.HasPrefix(b.Info, "example") {
			continue
		}
		for i := 0; i < len(b.Text); i++ {
			if b.Text[i] == "." {
				in, out = b.Text[:i], b.Text[i+1:]
				goto Found
			}
		}
		log.Fatalf("did not find . in pre block:\n%s", strings.Join(b.Text, "\n"))
	Found:
		parserChange := false
		if b.Info != "example" {
			js, ok := parsers[b.Info]
			if !ok {
				log.Printf("skipping %s", b.Info)
				continue
			}
			parserChange = true
			a.Files = append(a.Files, txtar.File{Name: "parser.json", Data: []byte(js)})
		}
		n++
		name := fmt.Sprintf("%d", n)
		a.Files = append(a.Files,
			txtar.File{
				Name: name + ".md",
				Data: []byte(encode(join(in))),
			},
			txtar.File{
				Name: name + ".html",
				Data: []byte(encode(join(out))),
			},
		)
		if parserChange {
			a.Files = append(a.Files, txtar.File{Name: "parser.json", Data: []byte(`{}`)})
		}
	}

	os.Stdout.Write(txtar.Format(a))
}

func encode(s string) string {
	s = strings.ReplaceAll(s, " \n", " ^J\n")
	s = strings.ReplaceAll(s, "\t\n", "\t^J\n")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "^D\n"
	}
	return s
}

func join(s []string) string {
	if len(s) == 0 {
		return ""
	}
	x := strings.Join(s, "\n") + "\n"
	x = strings.ReplaceAll(x, "→", "\t")
	return x
}
