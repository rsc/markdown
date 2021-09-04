// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"strings"
)

type CodeBlock struct {
	Position
	Fence string
	Info  string
	Text  []string
}

func (b *CodeBlock) PrintHTML(buf *bytes.Buffer) {
	if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
		buf.WriteString("\n")
	}
	buf.WriteString("<pre><code")
	if b.Info != "" {
		fmt.Fprintf(buf, " class=\"language-%s\"", htmlQuoteEscaper.Replace(b.Info))
	}
	buf.WriteString(">")
	if b.Fence == "" { // TODO move
		for len(b.Text) > 0 && strings.Trim(b.Text[len(b.Text)-1], " \t") == "" {
			b.Text = b.Text[:len(b.Text)-1]
		}
	}
	for _, s := range b.Text {
		buf.WriteString(htmlEscaper.Replace(s))
		buf.WriteString("\n")
	}
	buf.WriteString("</code></pre>\n")
}

func newPre(p *parser, line Line) (Line, bool) {
	peek2 := line
	if p.para() == nil && peek2.trimSpace(4, 4, false) && !peek2.isBlank() {
		b := &preBuilder{}
		p.addBlock(b)
		b.text = append(b.text, peek2.string())
		return Line{}, true
	}
	return line, false
}

func newFence(p *parser, line Line) (Line, bool) {
	var fence, info string
	var n int
	peek := line
	if peek.trimFence(&fence, &info, &n) {
		p.addBlock(&fenceBuilder{fence, info, n, nil})
		return Line{}, true
	}
	return line, false
}

func (s *Line) trimFence(fence, info *string, n *int) bool {
	t := *s
	*n = 0
	for *n < 3 && t.trimSpace(1, 1, false) {
		*n++
	}
	switch c := t.peek(); c {
	case '`', '~':
		f := t.string()
		n := 0
		for i := 0; ; i++ {
			if !t.trim(c) {
				if i >= 3 {
					break
				}
				return false
			}
			n++
		}
		txt := mdUnescaper.Replace(t.trimString())
		if c == '`' && strings.Contains(txt, "`") {
			return false
		}
		i := strings.IndexAny(txt, " \t")
		if i >= 0 {
			txt = txt[:i]
		}
		*info = txt

		*fence = f[:n]
		*s = Line{}
		return true
	}
	return false
}

type preBuilder struct {
	text []string
}

func (c *preBuilder) Extend(p *parser, line Line) (Line, bool) {
	if !line.trimSpace(4, 4, true) {
		return line, false
	}
	c.text = append(c.text, line.string())
	return Line{}, true
}

func (b *preBuilder) Build(p BuildState) Block {
	return &CodeBlock{p.Pos(), "", "", b.text}
}

type fenceBuilder struct {
	fence string
	info  string
	n     int
	text  []string
}

func (c *fenceBuilder) Extend(p *parser, line Line) (Line, bool) {
	var fence, info string
	var n int
	if t := line; t.trimFence(&fence, &info, &n) && strings.HasPrefix(fence, c.fence) && info == "" {
		return Line{}, false
	}
	line.trimSpace(0, c.n, false)
	c.text = append(c.text, line.string())
	return Line{}, true
}

func (c *fenceBuilder) Build(p BuildState) Block {
	return &CodeBlock{
		p.Pos(),
		c.fence,
		c.info,
		c.text,
	}
}
