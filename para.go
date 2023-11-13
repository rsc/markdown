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

func (b *Empty) printMarkdown(*bytes.Buffer, mdState) {}

type Paragraph struct {
	Position
	Text *Text
}

func (b *Paragraph) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<p>")
	b.Text.PrintHTML(buf)
	buf.WriteString("</p>\n")
}

func (b *Paragraph) printMarkdown(buf *bytes.Buffer, s mdState) {
	// // Ignore prefix when in a list.
	// if s.bullet == 0 {
	// 	buf.WriteString(s.prefix)
	// }
	b.Text.printMarkdown(buf, s)
}

type paraBuilder struct {
	text []string
}

func (c *paraBuilder) extend(p *Parser, s line) (line, bool) {
	return s, false
}

func (b *paraBuilder) build(p buildState) Block {
	s := strings.Join(b.text, "\n")
	for s != "" {
		end, ok := parseLinkRefDef(p, s)
		if !ok {
			break
		}
		s = s[skipSpace(s, end):]
	}

	if s == "" {
		return &Empty{p.pos()}
	}

	return &Paragraph{
		p.pos(),
		p.newText(p.pos(), s),
	}
}

func newPara(p *Parser, s line) (line, bool) {
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
	b.text = append(b.text, s.trimSpaceString())
	return line{}, true
}
