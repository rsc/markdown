// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"flag"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/tools/txtar"
)

var goldmarkFlag = flag.Bool("goldmark", false, "run goldmark tests")

func Test(t *testing.T) {
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".txt"), func(t *testing.T) {
			a, err := txtar.ParseFile(file)
			if err != nil {
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
					doc := Parse(decode(string(md.Data)))
					h := encode(ToHTML(doc))
					if h != string(html.Data) {
						t.Fatalf("input %q\nparse:\n%s\nhave %q\nwant %q\ndingus: (https://spec.commonmark.org/dingus/?text=%s)", md.Data, dump(doc), h, html.Data, strings.ReplaceAll(url.QueryEscape(decode(string(md.Data))), "+", "%20"))
					}
					npass++
				})

				if !*goldmarkFlag {
					continue
				}
				t.Run("goldmark/"+name, func(t *testing.T) {
					gm := goldmark.New(
						goldmark.WithRendererOptions(
							ghtml.WithUnsafe(),
						),
					)
					var buf bytes.Buffer
					if err := gm.Convert([]byte(decode(string(md.Data))), &buf); err != nil {
						t.Fatal(err)
					}
					if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
						buf.WriteByte('\n')
					}
					want := string(html.Data)
					want = strings.ReplaceAll(want, " />", ">")
					out := encode(buf.String())
					out = strings.ReplaceAll(out, " />", ">")
					if out != want {
						t.Fatalf("\n    - input: ``%q``\n    - output: ``%q``\n    - golden: ``%q``\n    - [dingus](https://spec.commonmark.org/dingus/?text=%s)", md.Data, out, want, strings.ReplaceAll(url.QueryEscape(decode(string(md.Data))), "+", "%20"))
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
