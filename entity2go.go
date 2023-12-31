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
	"strings"
)

var outfile = flag.String("o", "", "write output to `file`")

func main() {
	log.SetFlags(0)
	log.SetPrefix("entity2go: ")
	flag.Parse()

	resp, err := http.Get("https://html.spec.whatwg.org/entities.json")
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

	list := make(map[string]struct {
		Codepoints []rune
	})
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
	buf.WriteString(hdr)
	fmt.Fprintf(&buf, "var htmlEntity = map[string]string{\n")
	for _, name := range names {
		if !strings.HasSuffix(name, ";") {
			continue
		}
		fmt.Fprintf(&buf, "\t%q: \"", name)
		for _, r := range list[name].Codepoints {
			if r <= 0xFFFF {
				fmt.Fprintf(&buf, "\\u%04x", r)
			} else {
				fmt.Fprintf(&buf, "\\U%08x", r)
			}
		}
		fmt.Fprintf(&buf, "\",\n")
	}
	fmt.Fprintf(&buf, "}\n")

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

//go:generate go run entity2go.go -o entity.go

package markdown

// htmlEntity maps known HTML entity sequences to their meanings.
`
