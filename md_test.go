// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
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
		if strings.HasSuffix(file, "_fmt.txt") {
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
					p = parseParser(t, a.Files[i].Data)
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

					// Make sure unexported types like emphPlain don't leak into result.
					if x, ok := findUnexported(reflect.ValueOf(doc)); ok {
						t.Fatalf("input %q\nparse:\n%s\nfound parsed value of unexported type %s", md.Data, dump(doc), x.Type())
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

func parseParser(t *testing.T, data []byte) Parser {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var p Parser
	err := d.Decode(&p)
	if err != nil {
		t.Fatalf("reading parser.json: %v", err)
	}
	err = d.Decode(new(json.RawMessage))
	if err != io.EOF {
		t.Fatalf("junk on end of parser.json")
	}
	return p
}

func TestFormat(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*_fmt.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		t.Run(strings.TrimSuffix(filepath.Base(file), ".txt"), func(t *testing.T) {
			a, err := txtar.ParseFile(file)
			if err != nil {
				t.Fatal(err)
			}
			var p Parser
			for i := 0; i < len(a.Files); {
				if a.Files[i].Name == "parser.json" {
					p = parseParser(t, a.Files[i].Data)
					i++
					continue
				}
				// Each test case is a single markdown document that should render either as itself,
				// or if followed by a file named "want", then by that file.
				name := a.Files[i].Name
				in := a.Files[i].Data
				wantb := in
				i++
				if i < len(a.Files) && a.Files[i].Name == "want" {
					wantb = a.Files[i].Data
					i++
				}
				t.Run(name, func(t *testing.T) {
					doc := p.Parse(decode(string(in)))
					want := decode(string(wantb))
					docWant := p.Parse(want)
					if ToHTML(doc) != ToHTML(docWant) {
						t.Errorf("bad testdata: input and want are different markdown documents:\ninput:\n%s\n\nwant:\n%s", dump(doc), dump(docWant))
					}
					h := Format(doc)
					h = encode(h)
					if h != want {
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
			h := Format(doc)
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

func TestInline(t *testing.T) {
	// Test that these don't crash,
	// and also "cover" the bodies.
	new(HardBreak).Inline()
	new(SoftBreak).Inline()
	new(HTMLTag).Inline()
	new(Plain).Inline()
	new(Code).Inline()
	new(Strong).Inline()
	new(Del).Inline()
	new(Emph).Inline()
	new(Emoji).Inline()
	new(AutoLink).Inline()
	new(Link).Inline()
	new(Image).Inline()
	new(Task).Inline()
}

func findUnexported(v reflect.Value) (reflect.Value, bool) {
	if t := v.Type(); t.PkgPath() != "" && !token.IsExported(t.Name()) {
		return v, true
	}
	switch v.Kind() {
	case reflect.Interface, reflect.Pointer:
		if !v.IsNil() {
			if u, ok := findUnexported(v.Elem()); ok {
				return u, true
			}
		}
	case reflect.Struct:
		for i := 0; i < v.Type().NumField(); i++ {
			if !v.Type().Field(i).IsExported() {
				return v, true
			}
			if u, ok := findUnexported(v.Field(i)); ok {
				return u, true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			if u, ok := findUnexported(v.Index(i)); ok {
				return u, true
			}
		}
	}
	return v, false
}

var (
	blockType   = reflect.TypeOf(new(Block)).Elem()
	blocksType  = reflect.TypeOf(new([]Block)).Elem()
	inlinesType = reflect.TypeOf(new([]Inline)).Elem()
)

func printb(buf *bytes.Buffer, b Block, prefix string) {
	fmt.Fprintf(buf, "(%T", b)
	v := reflect.ValueOf(b)
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		fmt.Fprintf(buf, " %v", b)
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		if !tf.IsExported() {
			continue
		}
		if tf.Type == inlinesType {
			printis(buf, v.Field(i).Interface().([]Inline))
		} else if tf.Type.Kind() == reflect.Slice && tf.Type.Elem().Kind() == reflect.String {
			fmt.Fprintf(buf, " %s:%q", tf.Name, v.Field(i))
		} else if tf.Type != blocksType && !tf.Type.Implements(blockType) && tf.Type.Kind() != reflect.Slice {
			fmt.Fprintf(buf, " %s:%v", tf.Name, v.Field(i))
		}
	}

	prefix += "\t"
	for i := 0; i < t.NumField(); i++ {
		tf := t.Field(i)
		if !tf.IsExported() {
			continue
		}
		if tf.Type.Implements(blockType) {
			fmt.Fprintf(buf, "\n%s", prefix)
			printb(buf, v.Field(i).Interface().(Block), prefix)
		} else if tf.Type == blocksType {
			vf := v.Field(i)
			for i := 0; i < vf.Len(); i++ {
				fmt.Fprintf(buf, "\n%s", prefix)
				printb(buf, vf.Index(i).Interface().(Block), prefix)
			}
		} else if tf.Type.Kind() == reflect.Slice && tf.Type != inlinesType && tf.Type.Elem().Kind() != reflect.String {
			fmt.Fprintf(buf, "\n%s%s:", prefix, t.Field(i).Name)
			printslice(buf, v.Field(i), prefix)
		}
	}
	fmt.Fprintf(buf, ")")
}

func printslice(buf *bytes.Buffer, v reflect.Value, prefix string) {
	if v.Type().Elem().Kind() == reflect.Slice {
		for i := 0; i < v.Len(); i++ {
			fmt.Fprintf(buf, "\n%s#%d:", prefix, i)
			printslice(buf, v.Index(i), prefix+"\t")
		}
		return
	}
	for i := 0; i < v.Len(); i++ {
		fmt.Fprintf(buf, " ")
		printb(buf, v.Index(i).Interface().(Block), prefix+"\t")
	}
}

func printi(buf *bytes.Buffer, in Inline) {
	fmt.Fprintf(buf, "%T(", in)
	v := reflect.ValueOf(in).Elem()
	text := v.FieldByName("Text")
	if text.IsValid() {
		fmt.Fprintf(buf, "%q", text)
	}
	inner := v.FieldByName("Inner")
	if inner.IsValid() {
		printis(buf, inner.Interface().([]Inline))
	}
	buf.WriteString(")")
}

func printis(buf *bytes.Buffer, ins []Inline) {
	for _, in := range ins {
		buf.WriteByte(' ')
		printi(buf, in)
	}
}

func dump(b Block) string {
	var buf bytes.Buffer
	printb(&buf, b, "")
	return buf.String()
}
