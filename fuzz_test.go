// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/tools/txtar"
)

func FuzzGoldmark(f *testing.F) {
	if !*goldmarkFlag {
		f.Skip("-goldmark not set")
	}
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		f.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "to_markdown.txt") {
			continue
		}
		a, err := txtar.ParseFile(file)
		if err != nil {
			f.Fatal(err)
		}
		for i := 0; i+2 <= len(a.Files); {
			if a.Files[i].Name == "parser.json" {
				i++
				continue
			}
			md := a.Files[i]
			html := a.Files[i+1]
			i += 2
			name := strings.TrimSuffix(md.Name, ".md")
			if name != strings.TrimSuffix(html.Name, ".html") {
				f.Fatalf("mismatched file pair: %s and %s", md.Name, html.Name)
			}
			f.Add(decode(string(md.Data)))
		}
	}
	f.Fuzz(func(t *testing.T, s string) {
		// Too many corner cases involving non-terminated lines.
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		// Goldmark does not convert \r to \n.
		s = strings.ReplaceAll(s, "\r", "\n")
		// Goldmark treats \v as isUnicodeSpace for deciding emphasis.
		// Not unreasonable, but not what the spec says.
		s = strings.ReplaceAll(s, "\v", "\f")
		if !utf8.ValidString(s) {
			s = string([]rune(s)) // coerce to valid UTF8
		}
		var parsers = []Parser{
			{},
			{HeadingID: true},
			{Strikethrough: true},
			{TaskList: true},
			{HeadingID: true, Strikethrough: true, TaskList: true},
		}
		for i, p := range parsers {
			if t.Failed() {
				break
			}
			t.Run(fmt.Sprintf("p%d", i), func(t *testing.T) {
				doc, corner := p.parse(s)
				if corner {
					return
				}
				out := ToHTML(doc)

				gm := goldmarkParser(&p)
				var buf bytes.Buffer
				if err := gm.Convert([]byte(s), &buf); err != nil {
					t.Fatal(err)
				}
				if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
					buf.WriteByte('\n')
				}
				gout := buf.String()

				// Goldmark uses <br />, <hr />, and <img />.
				// Goldmark also escapes | as %7C.
				// Apply rewrites to out as well as gout to handle these appearing
				// as literals in the input.
				canon := func(s string) string {
					s = strings.ReplaceAll(s, " />", ">")
					s = strings.ReplaceAll(s, "%7C", "|")
					return s
				}
				out = canon(out)
				gout = canon(gout)

				if out != gout {
					q := strings.ReplaceAll(url.QueryEscape(s), "+", "%20")
					t.Fatalf("in: %q\nparse:\n%s\nout: %q\ngout: %q\ndingus: (https://spec.commonmark.org/dingus/?text=%s)\ngithub: (https://github.com/rsc/tmp/issues/new?body=%s)", s, dump(doc), out, gout, q, q)
				}
			})
		}
	})
}
