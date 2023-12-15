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
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var outfile = flag.String("o", "", "write output to `file`")

func get(url string) []byte {
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
	return data
}

var gemojiRE = regexp.MustCompile(`</?g-emoji[^<>]*>`)

func main() {
	log.SetFlags(0)
	log.SetPrefix("emoji2go: ")
	flag.Parse()

	emojiJSON := get("https://api.github.com/emojis")
	list := make(map[string]string)
	err := json.Unmarshal(emojiJSON, &list)
	if err != nil {
		log.Fatal(err)
	}

	var names []string
	for name := range list {
		names = append(names, name)
	}
	sort.Strings(names)

	emojiHTML := string(get("https://gist.github.com/rsc/316bc98c066ad111973634d435203aac"))

	bad := false
	var buf bytes.Buffer
	buf.WriteString(hdr)
	fmt.Fprintf(&buf, "var emoji = map[string]string{\n")
	n := 0
	for _, name := range names {
		n = max(n, len(name))
		_, val, ok := strings.Cut(emojiHTML, "<td><code>"+name+"</code></td>\n<td>")
		if !ok {
			log.Printf("gist missing :%s:", name)
			bad = true
			continue
		}
		val, _, ok = strings.Cut(val, "</td>")
		if !ok {
			log.Printf("gist missing :%s:", name)
			bad = true
			continue
		}
		val = gemojiRE.ReplaceAllString(val, "")
		if strings.Contains(val, "<") {
			log.Printf("skipping %s: non-unicode: %s", name, val)
			continue
		}
		fmt.Fprintf(&buf, "\t%q: %s,\n", name, strconv.QuoteToASCII(val))
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
		os.Stdout.Write(src)
	}
}

var hdr = `// Copyright 2023 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:generate go run emoji2go.go -o emoji.go

package markdown

// emoji maps known emoji names to their UTF-8 emoji forms.
`
