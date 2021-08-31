// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/tools/txtar"
)

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
					p := parse(decode(string(md.Data)))
					h := encode(toHTML(p))
					if h != string(html.Data) {
						t.Fatalf("input %q\nparse:\n%s\nhave %q\nwant %q", md.Data, dump(p), h, html.Data)
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
	s = strings.ReplaceAll(s, "^M\n", "\r\n")
	s = strings.ReplaceAll(s, "^D\n", "")
	return s
}

func encode(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "^M\n")
	s = strings.ReplaceAll(s, "\r", "^M^D\n")
	s = strings.ReplaceAll(s, " \n", " ^J\n")
	s = strings.ReplaceAll(s, "\t\n", "\t^J\n")
	if s != "" && !strings.HasSuffix(s, "\n") {
		s += "^D\n"
	}
	return s
}
