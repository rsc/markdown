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
	"go/format"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

var outfile = flag.String("o", "", "write output to `file`")

func main() {
	log.SetFlags(0)
	log.SetPrefix("emoji2go: ")
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

	bad := false
	var buf bytes.Buffer
	buf.WriteString(hdr)
	fmt.Fprintf(&buf, "var emoji = map[string]string{\n")
	n := 0
	for _, name := range names {
		n = max(n, len(name))
		url := list[name]
		_, file, ok := strings.Cut(url, "/emoji/unicode/")
		if !ok {
			// There are a handful of non-Unicode emoji on GitHub, like
			// :accessibility:, :basecamp:, :dependabot:, :electron:.
			// Ignore those.
			continue
		}
		file, _, ok = strings.Cut(file, ".png")
		if !ok {
			log.Printf("bad URL: :%s: => %s", name, url)
			bad = true
			continue
		}
		var runes []rune
		for _, f := range strings.Split(file, "-") {
			r, err := strconv.ParseUint(f, 16, 32)
			if err != nil {
				log.Printf("bad URL: :%s: => %s", name, url)
				bad = true
				continue
			}
			runes = append(runes, rune(r))
		}
		fmt.Fprintf(&buf, "\t%q: %s,\n", name, strconv.QuoteToASCII(string(runes)))
	}
	fmt.Fprintf(&buf, "}\n\n")

	fmt.Fprintf(&buf, "const maxEmojiLen = %d\n", n)

	if bad {
		os.Exit(1)
	}

	src, err := format.Source(buf.Bytes())
	if err != nil {
		log.Fatalf("reformatting output: %v", err)
	}

	if *outfile != "" {
		if err := os.WriteFile(*outfile, src, 0666); err != nil {
			log.Fatal(err)
		}
	} else {
		os.Stdout.Write(buf.Bytes())
	}
}

var hdr = `// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run emoji2go.go -o emoji.go

package markdown

// emoji maps known emoji names to their UTF-8 emoji forms.
`
