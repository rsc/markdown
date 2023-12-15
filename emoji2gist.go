// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
)

var outfile = flag.String("o", "", "write output to `file`")

func main() {
	log.SetFlags(0)
	log.SetPrefix("emoji2gist: ")
	flag.Parse()

	resp, err := http.Get("https://api.github.com/emojis")
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

	list := make(map[string]string)
	err = json.Unmarshal(data, &list)
	if err != nil {
		log.Fatal(err)
	}

	var names []string
	for name := range list {
		names = append(names, name)
	}
	sort.Strings(names)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "code | emoji\n-|-\n")
	for _, name := range names {
		fmt.Fprintf(&buf, "`%s` | :%s:\n", name, name)
	}

	if *outfile != "" {
		if err := os.WriteFile(*outfile, buf.Bytes(), 0666); err != nil {
			log.Fatal(err)
		}
	} else {
		os.Stdout.Write(buf.Bytes())
	}
}
