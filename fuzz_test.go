// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/renderer/html"
	"golang.org/x/tools/txtar"
)

func FuzzGoldmark(f *testing.F) {
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
		if len(a.Files) > 0 && a.Files[0].Name == "parser.json" {
			a.Files = a.Files[1:]
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
		if strings.Contains(s, "(%)") || strings.Contains(s, "<!") || strings.Contains(s, "\n   1.") || strings.Contains(s, "\n   - ") || !strings.HasSuffix(s, "\n") || strings.Contains(s, "\x00") {
			return
		}
		var p Parser
		doc := p.Parse(s)
		out := ToHTML(doc)
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
