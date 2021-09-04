// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"strings"
)

type Heading struct {
	Position
	Level int
	Text  Block
}

func (b *Heading) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<h%d>", b.Level)
	b.Text.PrintHTML(buf)
	fmt.Fprintf(buf, "</h%d>\n", b.Level)
}

func newATXHeading(p *parser, line Line) (Line, bool) {
	peek := line
	var n int
	if peek.trimHeading(&n) {
		s := peek.string()
		s = strings.TrimRight(s, " \t")
		if t := strings.TrimRight(s, "#"); t != strings.TrimRight(t, " \t") || t == "" {
			s = t
		}
		pos := Position{p.lineno, p.lineno}
		p.doneBlock(&Heading{pos, n, p.NewText(pos, s)})
		return Line{}, true
	}
	return line, false
}

func newSetextHeading(p *parser, line Line) (Line, bool) {
	var n int
	peek := line
	if p.nextB() == p.para() && peek.trimSetext(&n) {
		p.closeBlock()
		para, ok := p.Last().(*Paragraph)
		if !ok {
			return line, false
		}
		p.DeleteLast()
		p.doneBlock(&Heading{Position{para.StartLine, p.lineno}, n, para.Text})
		return Line{}, true
	}
	return line, false
}

func (s *Line) trimHeading(width *int) bool {
	t := *s
	t.trimSpace(0, 3, false)
	if !t.trim('#') {
		return false
	}
	n := 1
	for n < 6 && t.trim('#') {
		n++
	}
	if !t.trimSpace(1, 1, true) {
		return false
	}
	*width = n
	*s = t
	return true
}

func (s *Line) trimSetext(n *int) bool {
	t := *s
	t.trimSpace(0, 3, false)
	c := t.peek()
	if c == '-' || c == '=' {
		for t.trim(c) {
		}
		t.skipSpace()
		if t.eof() {
			if c == '=' {
				*n = 1
			} else {
				*n = 2
			}
			return true
		}
	}
	return false
}
