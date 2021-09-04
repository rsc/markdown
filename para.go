// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"strings"
)

type Empty struct {
	Position
}

func (b *Empty) PrintHTML(buf *bytes.Buffer) {}

type Paragraph struct {
	Position
	Text Block
}

func (b *Paragraph) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<p>")
	b.Text.PrintHTML(buf)
	buf.WriteString("</p>\n")
}

type paraBuilder struct {
	text []string
}

func (c *paraBuilder) Extend(p *parser, line Line) (Line, bool) {
	return line, false
}

func (b *paraBuilder) Build(p BuildState) Block {
	s := strings.Join(b.text, "\n")
	for s != "" {
		end, ok := parseLinkRefDef(p, s)
		if !ok {
			break
		}
		s = s[skipSpace(s, end):]
	}

	if s == "" {
		return &Empty{p.Pos()}
	}

	return &Paragraph{
		p.Pos(),
		p.NewText(p.Pos(), s),
	}
}

func newPara(p *parser, line Line) (Line, bool) {
	// Process paragraph continuation text or start new paragraph.
	b := p.para()
	if b != nil {
		for i := p.lineDepth; i < len(p.stack); i++ {
			p.stack[i].pos.EndLine = p.lineno
		}
	} else {
		// Note: Ends anything without a matching prefix.
		b = new(paraBuilder)
		p.addBlock(b)
	}
	b.text = append(b.text, line.trimSpaceString())
	return Line{}, true
}
