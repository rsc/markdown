// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// curl https://spec.commonmark.org/0.30/spec.json | go run spec2txtar.go > spec0.30.txt

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	"golang.org/x/tools/txtar"
)

type specCase struct {
	Name     string
	Markdown string
	HTML     string
	Example  int
}

func main() {
	log.SetFlags(0)
	log.SetPrefix("spec2txtar: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: spec2txtar url\n")
		os.Exit(2)
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
	}
	url := flag.Arg(0)

	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != 200 {
		log.Fatal(resp.Status)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	var spec []specCase
	err = json.Unmarshal(data, &spec)
	if err != nil {
		log.Fatal(err)
	}

	a := &txtar.Archive{
		Comment: []byte("// go run spec2txtar.go " + url + "\n"),
	}
	for _, cas := range spec {
		name := fmt.Sprintf("%d", cas.Example)
		a.Files = append(a.Files,
			txtar.File{
				Name: name + ".md",
				Data: []byte(encode(cas.Markdown)),
			},
			txtar.File{
				Name: name + ".html",
				Data: []byte(encode(cas.HTML)),
			},
		)
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
