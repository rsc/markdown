// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"encoding/json"
	"flag"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
	gext "github.com/yuin/goldmark/extension"
	gparser "github.com/yuin/goldmark/parser"
	ghtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/tools/txtar"
)

var goldmarkFlag = flag.Bool("goldmark", false, "run goldmark tests")

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
			var ncase, npass int
			for i := 0; i+2 <= len(a.Files); {
				if a.Files[i].Name == "parser.json" {
					p = Parser{}
					if err := json.Unmarshal(a.Files[i].Data, &p); err != nil {
						t.Fatal(err)
					}
					i++
					continue
				}
				ncase++
				md := a.Files[i]
				html := a.Files[i+1]
				i += 2
				name := strings.TrimSuffix(md.Name, ".md")
				if name != strings.TrimSuffix(html.Name, ".html") {
					t.Fatalf("mismatched file pair: %s and %s", md.Name, html.Name)
				}

				t.Run(name, func(t *testing.T) {
					doc := p.Parse(decode(string(md.Data)))
					h := encode(ToHTML(doc))
					if h != string(html.Data) {
						q := strings.ReplaceAll(url.QueryEscape(decode(string(md.Data))), "+", "%20")
						t.Fatalf("input %q\nparse:\n%s\nhave %q\nwant %q\ndingus: (https://spec.commonmark.org/dingus/?text=%s)\ngithub: (https://github.com/rsc/tmp/issues/new?body=%s)", md.Data, dump(doc), h, html.Data, q, q)
					}
					npass++
				})

				if !*goldmarkFlag {
					continue
				}
				t.Run("goldmark/"+name, func(t *testing.T) {
					in := decode(string(md.Data))
					_, corner := p.parse(in)
					if corner {
						t.Skip("known corner case")
					}
					gm := goldmarkParser(&p)
					var buf bytes.Buffer
					if err := gm.Convert([]byte(in), &buf); err != nil {
						t.Fatal(err)
					}
					if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
						buf.WriteByte('\n')
					}
					want := decode(string(html.Data))
					want = strings.ReplaceAll(want, " />", ">")
					out := buf.String()
					out = strings.ReplaceAll(out, " />", ">")
					q := strings.ReplaceAll(url.QueryEscape(decode(string(md.Data))), "+", "%20")
					if out != want {
						t.Fatalf("\n    - input: ``%q``\n    - output: ``%q``\n    - golden: ``%q``\n    - [dingus](https://spec.commonmark.org/dingus/?text=%s)\n    - [github](https://github.com/rsc/tmp/issues/new?body=%s)", in, out, want, q, q)
					}
					npass++

				})
			}
			t.Logf("%d/%d pass", npass, ncase)
		})
	}
}

func goldmarkParser(p *Parser) goldmark.Markdown {
	opts := []goldmark.Option{
		goldmark.WithRendererOptions(ghtml.WithUnsafe()),
	}
	if p.HeadingIDs {
		opts = append(opts, goldmark.WithParserOptions(gparser.WithHeadingAttribute()))
	}
	if p.Strikethrough {
		opts = append(opts, goldmark.WithExtensions(gext.Strikethrough))
	}
	if p.TaskListItems {
		opts = append(opts, goldmark.WithExtensions(gext.TaskList))
	}
	if p.AutoLinkText {
		opts = append(opts, goldmark.WithExtensions(gext.Linkify))
	}
	if p.Table {
		opts = append(opts, goldmark.WithExtensions(gext.Table))
	}
	return goldmark.New(opts...)
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
			i := 0

			if a.Files[i].Name == "parser.json" {
				if err := json.Unmarshal(a.Files[i].Data, &p); err != nil {
					t.Fatal(err)
				}
				i++
			}

			for i < len(a.Files) {
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

func TestHeadingIDToMarkdown(t *testing.T) {
	p := Parser{HeadingIDs: true}
	text := `# H {#id}`
	doc := p.Parse(text)
	got := ToMarkdown(doc)
	want := text + "\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
