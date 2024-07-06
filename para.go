// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
)

// TODO: unexport Empty?

// An Empty is a [Block] representing no block at all.
// The parser never returns a parse tree containing an Empty,
// but it can be useful during syntax editing.
// It does not render as anything at all.
type Empty struct {
	Position
}

func (*Empty) Block() {}

func (b *Empty) printHTML(p *printer) {}

func (b *Empty) printMarkdown(*printer) {}

type Text struct {
	Position
	Inline Inlines
}

// TODO: This is only a Block for tight lists. Maybe keep the Paragraphs for those?
func (*Text) Block() {}

func (b *Text) printHTML(p *printer) {
	for _, x := range b.Inline {
		x.printHTML(p)
	}
}

func (b *Text) printMarkdown(p *printer) {
	for _, x := range b.Inline {
		x.printMarkdown(p)
	}
}

// A Paragraph is a [Block] representing a [paragraph].
// Except when they appear as top-level blocks in an item of a tight list,
// paragraphs render in <p>...</p> tags.
//
// [paragraph]: https://spec.commonmark.org/0.31.2/#paragraphs
type Paragraph struct {
	Position
	Text *Text
}

func (*Paragraph) Block() {}

func (b *Paragraph) printHTML(p *printer) {
	p.html("<p>")
	b.Text.printHTML(p)
	p.html("</p>\n")
}

func (b *Paragraph) printMarkdown(p *printer) {
	p.maybeNL()
	b.Text.printMarkdown(p)
}

// A paraBuilder is a [blockBuilder] for a [Paragraph].
type paraBuilder struct {
	text  []string // each line of the paragraph
	table *tableBuilder
}

// startParagraph is a [starter] for a [Paragraph].
func startParagraph(p *parser, s line) (line, bool) {
	// Process paragraph continuation text or start new paragraph.
	b := p.para()
	indented := p.lineDepth == len(p.stack)-2 // fully indented, not playing "pargraph continuation text" games
	text := s.trimSpaceString()

	if b != nil && b.table != nil {
		if indented && text != "" && text != "|" {
			// Continue table.
			b.table.addRow(text)
			return line{}, true
		}
		// Blank or unindented line ends table.
		// (So does a new block structure, but the caller has checked that already.)
		// So does a line with just a pipe:
		// https://github.com/github/cmark-gfm/pull/127 and
		// https://github.com/github/cmark-gfm/pull/128
		// fixed a buffer overread by rejecting | by itself as a table line.
		// That seems to violate the spec, but we will play along.
		b = nil
	}

	// If we are looking for tables and this is a table start, start a table.
	if p.Table && b != nil && indented && len(b.text) > 0 && isTableStart(b.text[len(b.text)-1], text) {
		// The current line s is the delimiter line.
		// The previous line in the paragraph is the header line.
		// Take the header line out of the current paragraph and
		// start a new paragraph that will be only the table.
		// Removing the last line from b may result in an empty paragraph.
		// That is handled by [paraBuilder.build].
		//
		// TODO: Why not make tableBuilder its own builder?
		// It seems like that would work (tables don't get paragraph continuation text).
		hdr := b.text[len(b.text)-1]
		b.text = b.text[:len(b.text)-1]
		tb := new(paraBuilder)
		p.addBlock(tb)
		tb.table = new(tableBuilder)
		tb.table.start(hdr, text)
		return line{}, true
	}

	if b != nil {
		for i := p.lineDepth; i < len(p.stack); i++ {
			p.stack[i].pos.EndLine = p.lineno
		}
	} else {
		// Note: Ends anything without a matching prefix.
		b = new(paraBuilder)
		p.addBlock(b)
	}
	b.text = append(b.text, text)
	return line{}, true
}

// extend would normally extend the paragraph with the line s,
// but we return false and let startParagraph handle extension,
// which it must for “paragraph continuation text” anyway.
func (b *paraBuilder) extend(p *parser, s line) (line, bool) {
	return s, false
}

func (b *paraBuilder) build(p *parser) Block {
	// If this paragraph is actually a table, build the table instead.
	if b.table != nil {
		return b.table.build(p)
	}

	// Join all the lines (leading framing already removed)
	// to produce the full string of the paragraph.
	// In theory the join could be avoided by having [parser.inline]
	// handle a slice of lines, but then all the [inlineParser] implementations
	// would need to do that too, which would complicate them.
	// The join is simple.
	s := strings.Join(b.text, "\n")

	// Parse and remove any link reference definitions at the start of s.
	for s != "" {
		end, ok := parseLinkRefDef(p, s)
		if !ok {
			break
		}
		s = s[skipSpace(s, end):]
	}

	// If the paragraph is empty, return an Empty.
	// This can happen if the text was entirely link reference definitions,
	// but it can also happen if there is no paragraph text before a table.
	if s == "" {
		return &Empty{p.pos()}
	}

	// Recompute EndLine because the last line of b.text
	// might have been removed to start a table.
	pos := p.pos()
	pos.EndLine = pos.StartLine + len(b.text) - 1
	return &Paragraph{
		pos,
		p.newText(pos, s),
	}
}
