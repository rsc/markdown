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
	// The HTML id attribute. The parser populates this field if
	// [Parser.HeadingIDs] is true and the heading ends with text like "{#id}".
	ID string
}

func (b *Heading) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<h%d", b.Level)
	if b.ID != "" {
		fmt.Fprintf(buf, ` id="%s"`, htmlQuoteEscaper.Replace(b.ID))
	}
	buf.WriteByte('>')
	b.Text.PrintHTML(buf)
	fmt.Fprintf(buf, "</h%d>\n", b.Level)
}

func newATXHeading(p *Parser, s line) (line, bool) {
	peek := s
	var n int
	if peek.trimHeading(&n) {
		s := peek.string()
		s = strings.TrimRight(s, " \t")
		// Remove trailing '#'s.
		if t := strings.TrimRight(s, "#"); t != strings.TrimRight(t, " \t") || t == "" {
			s = t
		}
		var id string
		if p.HeadingIDs {
			// Parse and remove ID attribute.
			// It must come before trailing '#'s to more closely follow the spec:
			//    The optional closing sequence of #s must be preceded by spaces or tabs
			//    and may be followed by spaces or tabs only.
			// But Goldmark allows it to come after.
			id, s = extractID(s)
		}
		pos := Position{p.lineno, p.lineno}
		p.doneBlock(&Heading{pos, n, p.newText(pos, s), id})
		return line{}, true
	}
	return s, false
}

// extractID removes an ID attribute from s if one is present.
// It returns the attribute value and the resulting string.
// The attribute has the form "{#...}", where the "..." can contain
// any character other than '}'.
// The attribute must be followed only by whitespace.
func extractID(s string) (id, s2 string) {
	i := strings.LastIndexByte(s, '{')
	if i < 0 || i == len(s)-1 {
		return "", s
	}
	if s[i+1] != '#' {
		return "", s
	}
	j := i + strings.IndexByte(s[i:], '}')
	if j < 0 || strings.TrimRight(s[j+1:], " \t") != "" {
		return "", s
	}
	return s[i+2 : j], s[:i]
}

func newSetextHeading(p *Parser, s line) (line, bool) {
	var n int
	peek := s
	if p.nextB() == p.para() && peek.trimSetext(&n) {
		p.closeBlock()
		para, ok := p.last().(*Paragraph)
		if !ok {
			return s, false
		}
		p.deleteLast()
		p.doneBlock(&Heading{Position{para.StartLine, p.lineno}, n, para.Text, ""})
		return line{}, true
	}
	return s, false
}

func (s *line) trimHeading(width *int) bool {
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

func (s *line) trimSetext(n *int) bool {
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
