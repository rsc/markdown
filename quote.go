// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import "bytes"

type Quote struct {
	Position
	Blocks []Block
}

func (b *Quote) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<blockquote>\n")
	for _, c := range b.Blocks {
		c.PrintHTML(buf)
	}
	buf.WriteString("</blockquote>\n")
}

func trimQuote(s Line) (Line, bool) {
	t := s
	t.trimSpace(0, 3, false)
	if !t.trim('>') {
		return s, false
	}
	t.trimSpace(0, 1, true)
	return t, true
}

type quoteBuilder struct{}

func newQuote(p *parser, line Line) (Line, bool) {
	if line, ok := trimQuote(line); ok {
		p.addBlock(new(quoteBuilder))
		return line, true
	}
	return line, false
}

func (b *quoteBuilder) Extend(p *parser, line Line) (Line, bool) {
	return trimQuote(line)
}

func (b *quoteBuilder) Build(p BuildState) Block {
	return &Quote{p.Pos(), p.Blocks()}
}
