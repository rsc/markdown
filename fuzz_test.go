// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build gofuzzbeta

package markdown

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/tools/txtar"
)

func Fuzz(f *testing.F) {
	files, err := filepath.Glob("testdata/*.txt")
	if err != nil {
		f.Fatal(err)
	}
	for _, file := range files {
		a, err := txtar.ParseFile(file)
		if err != nil {
			f.Fatal(err)
		}
		for i := 0; i+2 <= len(a.Files); i += 2 {
			md := a.Files[i]
			html := a.Files[i+1]
			name := strings.TrimSuffix(md.Name, ".md")
			if name != strings.TrimSuffix(html.Name, ".html") {
				f.Fatalf("mismatched file pair: %s and %s", md.Name, html.Name)
			}
			f.Add(decode(string(md.Data)))
		}
	}
	f.Fuzz(func(t *testing.T, s string) {
		if strings.Contains(s, "\r") || strings.Contains(s, "\x00") || !utf8.ValidString(s) || !strings.HasSuffix(s, "\n") || strings.Contains(s, "<>") || strings.Contains(s, "\v") || strings.Contains(s, "\t") || strings.Contains(s, "<1") || strings.Contains(s, "<2") || strings.Contains(s, "<3") || strings.Contains(s, "<4") || strings.Contains(s, "<5") || strings.Contains(s, "<6") || strings.Contains(s, "<7") || strings.Contains(s, "<8") || strings.Contains(s, "<9") || strings.Contains(s, "<0") || strings.Contains(s, "-\n") || strings.Contains(s, "*\n") || strings.Contains(s, "]\n:") || strings.Contains(s, "\f") || strings.Contains(s, "\n'") || strings.Contains(s, "_]") || strings.Contains(s, "(%)") || strings.Contains(s, "<!") || strings.Contains("\n"+s, "\n[") || strings.Contains(s, "< ") || strings.Contains(s, "<\n") || strings.Contains(s, " [") || strings.Contains(s, "]:") || strings.Contains(s, "<") || strings.Contains(s, "&#") || strings.Contains(s, "\n*") || strings.Contains(s, "\n-") || strings.Contains(s, " \\") || strings.Contains(s, "\n -") || strings.Contains(s, "\n  -") || strings.Contains(s, "\n   -") || strings.Contains(s, "%") || strings.Contains(s, "![") || strings.Contains(s, "\\\\") || strings.Contains(s, "[_") || strings.Contains(s, "[*") || strings.Contains(s, "*]") || strings.Contains(s, "-") || strings.Contains(s, "+") || strings.Contains(s, "*") {
			return
		}

		doc := parse(s)
		out := toHTML(doc)
		out = strings.ReplaceAll(out, " />", ">")

		gm := goldmark.New(
			goldmark.WithRendererOptions(
				html.WithUnsafe(),
			),
		)
		var buf bytes.Buffer
		if err := gm.Convert([]byte(s), &buf); err != nil {
			t.Fatal(err)
		}
		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteByte('\n')
		}
		gout := buf.String()
		gout = strings.ReplaceAll(gout, " />", ">")
		gout = strings.ReplaceAll(gout, ` title=""`, ``)

		if out != gout {
			t.Fatalf("in: %q\nparse:\n%s\nout: %q\ngout: %q\ndingus: (https://spec.commonmark.org/dingus/?text=%s)", s, dump(doc), out, gout, strings.ReplaceAll(url.QueryEscape(s), "+", "%20"))
		}
	})
}

func FuzzPassword(f *testing.F) {
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 0 && s[0] == 'p' {
			if len(s) > 1 && s[1] == 'a' {
				if len(s) > 2 && s[2] == 's' {
					if len(s) > 3 && s[3] == 's' {
						if len(s) > 4 && s[4] == 'w' {
							if len(s) > 5 && s[5] == 'o' {
								if len(s) > 6 && s[6] == 'r' {
									if len(s) > 7 && s[7] == 'd' {
										if len(s) > 8 && s[8] == '!' {
											if len(s) == 9 {
												panic("password!")
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	})
}
