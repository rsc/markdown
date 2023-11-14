// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

func TestToHTML(t *testing.T) {
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "to_markdown.txt") {
			continue
		}
		t.Run(strings.TrimSuffix(filepath.Base(file), ".txt"), func(t *testing.T) {
			a, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}

			var p Parser
			if err := setParserOptions(&p, a.Comment); err != nil {
				t.Fatal(err)
			}

			var ncase, npass int
			for i := 0; i+2 <= len(a.Files); i += 2 {
				ncase++
				md := a.Files[i]
				html := a.Files[i+1]
				name := strings.TrimSuffix(md.Name, ".md")
				if name != strings.TrimSuffix(html.Name, ".html") {
					t.Fatalf("mismatched file pair: %s and %s", md.Name, html.Name)
				}

				t.Run(name, func(t *testing.T) {
					doc := p.Parse(decode(string(md.Data)))
					h := encode(ToHTML(doc))
					if h != string(html.Data) {
						t.Fatalf("input %q\nparse:\n%s\nhave %q\nwant %q\ndingus: (https://spec.commonmark.org/dingus/?text=%s)", md.Data, dump(doc), h, html.Data, strings.ReplaceAll(url.QueryEscape(decode(string(md.Data))), "+", "%20"))
					}
					npass++
				})
			}
			t.Logf("%d/%d pass", npass, ncase)
		})
	}
}

func decode(s string) string {
	s = strings.ReplaceAll(s, "^J\n", "\n")
	s = strings.ReplaceAll(s, "^M", "\r")
	s = strings.ReplaceAll(s, "^D\n", "")
	s = strings.ReplaceAll(s, "^@", "\x00")
	return s
}

func encode(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "^M\n")
	s = strings.ReplaceAll(s, "\r", "^M^D\n")
	s = strings.ReplaceAll(s, " \n", " ^J\n")
	s = strings.ReplaceAll(s, "\t\n", "\t^J\n")
	s = strings.ReplaceAll(s, "\x00", "^@")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "^D\n"
	}
	return s
}

// setParserOptions extracts lines of the form
//
//	key: value
//
// from data and sets the corresponding options on the Parser.
func setParserOptions(p *Parser, data []byte) error {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "//") {
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		value = strings.TrimSpace(value)
		switch key {
		case "HeadingIDs":
			b, err := strconv.ParseBool(value)
			if err != nil {
				return err
			}
			p.HeadingIDs = b
		default:
			return fmt.Errorf("unknown option: %q", key)
		}
	}
	return nil
}

func TestToMarkdown(t *testing.T) {
	// txtar files end in "_to_markdown.txt" and use the same encoding as
	// for the HTML tests.
	const suffix = "_to_markdown.txt"
	files, err := filepath.Glob(filepath.Join("testdata", "*"+suffix))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), suffix), func(t *testing.T) {
			a, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var p Parser
			for i := 0; i < len(a.Files); {
				// Each test case is a single markdown document that should render either as itself,
				// or if followed by a file named "want", then by that file.
				name := a.Files[i].Name
				in := a.Files[i].Data
				want := in
				i++
				if i < len(a.Files) && a.Files[i].Name == "want" {
					want = a.Files[i].Data
					i++
				}
				t.Run(name, func(t *testing.T) {
					doc := p.Parse(decode(string(in)))
					h := ToMarkdown(doc)
					h = encode(h)
					if h != string(want) {
						t.Errorf("input %q\nparse: \n%s\nhave %q\nwant %q", in, dump(doc), h, want)
					}
				})
			}
		})
	}

	// Files ending in ".md" should render as themselves.
	files, err = filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".md"), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			w := string(data)
			var p Parser
			doc := p.Parse(w)
			h := ToMarkdown(doc)
			if h != w {
				t.Errorf("have:\n%s\nwant:\n%s", h, w)
				outfile := file + ".have"
				t.Logf("writing have to %s", outfile)
				if err := os.WriteFile(outfile, []byte(h), 0400); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
