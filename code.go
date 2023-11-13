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

// func initialSpaces(s string) int {
// 	for i := 0; i < len(s); i++ {
// 		if s[i] != ' ' {
// 			return i
// 		}
// 	}
// 	return len(s)
// }

func (b *CodeBlock) printMarkdown(buf *bytes.Buffer, s mdState) {
	prefix1 := s.prefix1
	if prefix1 == "" {
		prefix1 = s.prefix
	}
	if b.Fence == "" {
		for i, line := range b.Text {
			// Ignore final empty line (why is it even there?).
			if i == len(b.Text)-1 && len(line) == 0 {
				break
			}
			// var iline string
			// is := initialSpaces(line)
			// if is < 4 {
			// 	iline = "    " + line
			// } else {
			// 	iline = "\t" + line[4:]
			// }
			// Indent by 4 spaces.
			pre := s.prefix
			if i == 0 {
				pre = prefix1
			}
			fmt.Fprintf(buf, "%s%s%s\n", pre, "    ", line)
		}
	} else {
		fmt.Fprintf(buf, "%s%s\n", prefix1, b.Fence)
		for _, line := range b.Text {
			fmt.Fprintf(buf, "%s%s\n", s.prefix, line)
		}
		fmt.Fprintf(buf, "%s%s\n", s.prefix, b.Fence)
	}
}

func newPre(p *Parser, s line) (line, bool) {
	peek2 := s
	if p.para() == nil && peek2.trimSpace(4, 4, false) && !peek2.isBlank() {
		b := &preBuilder{ /*indent: strings.TrimSuffix(s.string(), peek2.string())*/ }
		p.addBlock(b)
		b.text = append(b.text, peek2.string())
		return line{}, true
	}
	return s, false
}

func newFence(p *Parser, s line) (line, bool) {
	var fence, info string
	var n int
	peek := s
	if peek.trimFence(&fence, &info, &n) {
		p.addBlock(&fenceBuilder{fence, info, n, nil})
		return line{}, true
	}
	return s, false
}

func (s *line) trimFence(fence, info *string, n *int) bool {
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
		*s = line{}
		return true
	}
	return false
}

// For indented code blocks.
type preBuilder struct {
	indent string
	text   []string
}

func (c *preBuilder) extend(p *Parser, s line) (line, bool) {
	if !s.trimSpace(4, 4, true) {
		return s, false
	}
	c.text = append(c.text, s.string())
	return line{}, true
}

func (b *preBuilder) build(p buildState) Block {
	return &CodeBlock{p.pos(), "", "", b.text}
}

type fenceBuilder struct {
	fence string
	info  string
	n     int
	text  []string
}

func (c *fenceBuilder) extend(p *Parser, s line) (line, bool) {
	var fence, info string
	var n int
	if t := s; t.trimFence(&fence, &info, &n) && strings.HasPrefix(fence, c.fence) && info == "" {
		return line{}, false
	}
	s.trimSpace(0, c.n, false)
	c.text = append(c.text, s.string())
	return line{}, true
}

func (c *fenceBuilder) build(p buildState) Block {
	return &CodeBlock{
		p.pos(),
		c.fence,
		c.info,
		c.text,
	}
}
