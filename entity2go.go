// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
)

type entityList map[string]struct {
	Codepoints []int
}

func main() {
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

	fmt.Printf("var htmlEntity = map[string]string{\n")
	for _, name := range names {
		if !strings.HasSuffix(name, ";") {
			continue
		}
		fmt.Printf("\t%q: \"", name)
		for _, r := range list[name].Codepoints {
			if r <= 0xFFFF {
				fmt.Printf("\\u%04x", r)
			} else {
				fmt.Printf("\\U%08x", r)
			}
		}
		fmt.Printf("\",\n")
	}
	fmt.Printf("}\n")
}
