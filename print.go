// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import "bytes"

const (
	writeMarkdown = iota
	writeHTML
	writeText
)

type printer struct {
	writeMode   int
	buf         bytes.Buffer
	prefix      []byte
	prefixOld   []byte
	prefixOlder []byte
	trimLimit   int
	listOut
	footnotes    map[*Footnote]*printedNote
	footnotelist []*printedNote
}

type listOut struct {
	bullet rune
	num    int
	loose  int
	tight  int
}

func (w *printer) WriteStrings(list ...string) {
	for _, s := range list {
		w.WriteString(s)
	}
}

func cutLastNL(text []byte) (prefix, last []byte) {
	i := bytes.LastIndexByte(text, '\n')
	if i < 0 {
		return nil, text
	}
	return text[:i], text[i+1:]
}

func (b *printer) noTrim() {
	b.trimLimit = len(b.buf.Bytes())
}

func (b *printer) nl() {
	text := b.buf.Bytes()
	for len(text) > b.trimLimit && text[len(text)-1] == ' ' {
		text = text[:len(text)-1]
	}
	b.buf.Truncate(len(text))

	b.buf.WriteByte('\n')
	b.buf.Write(b.prefix)
	b.prefixOlder, b.prefixOld = b.prefixOld, b.prefix
}

func (b *printer) maybeNL() bool {
	// Starting a new block that may need a blank line before it
	// to avoid being mixed into a previous block
	// as paragraph continuation text.
	//
	// If the prefix on the current line (all of cur)
	// is the same as the current continuation prefix
	// (not first line of a list item)
	// and the previous line started with the same prefix,
	// then we need a blank line to avoid looking like
	// paragraph continuation text.
	before, cur := cutLastNL(b.buf.Bytes())
	before, prev := cutLastNL(before)
	if b.buf.Len() > 0 && bytes.Equal(cur, b.prefix) && bytes.HasPrefix(prev, b.prefix) {
		b.nl()
		return true
	}
	return true
}

func ToHTML(b Block) string {
	var p printer
	p.writeMode = writeHTML
	b.printHTML(&p)
	printFootnoteHTML(&p)
	return p.buf.String()
}

func Format(b Block) string {
	var p printer
	b.printMarkdown(&p)
	printFootnoteMarkdown(&p)
	// TODO footnotes?
	return p.buf.String()
}

var closeP = []byte("</p>\n")

func (b *printer) eraseCloseP() bool {
	if bytes.HasSuffix(b.buf.Bytes(), closeP) {
		b.buf.Truncate(b.buf.Len() - len(closeP))
		return true
	}
	return false
}

func (b *printer) maybeQuoteNL(quote byte) bool {
	// Starting a new quote block.
	// Make sure it doesn't look like it is part of a preceding quote block.
	before, cur := cutLastNL(b.buf.Bytes())
	before, prev := cutLastNL(before)
	if len(prev) >= len(cur)+1 && bytes.HasPrefix(prev, cur) && prev[len(cur)] == quote {
		b.nl()
		return true
	}
	return false
}

func (b *printer) WriteByte(c byte) error {
	if c == '\n' {
		panic("Write \\n")
	}
	return b.buf.WriteByte(c)
}

func (p *printer) Write(text []byte) (int, error) {
	if p.writeMode == writeMarkdown {
		for i := range text {
			if text[i] == '\n' {
				panic("Write \\n")
			}
		}
	}
	return p.buf.Write(text)
}

func (p *printer) html(list ...string) {
	if p.writeMode != writeHTML {
		panic("raw HTML in non-HTML output")
	}
	for _, s := range list {
		p.buf.WriteString(s)
	}
}

func (p *printer) text(list ...string) {
	if p.writeMode == writeHTML {
		for _, s := range list {
			htmlEscaper.WriteString(&p.buf, s)
		}
		return
	}
	for _, s := range list {
		p.buf.WriteString(s)
	}

}

func (p *printer) md(list ...string) {
	if p.writeMode != writeMarkdown {
		panic("markdown in non-markdown output")
	}
	for _, s := range list {
		p.buf.WriteString(s)
	}
}

func (b *printer) WriteString(s string) (int, error) {
	if b.writeMode == writeMarkdown {
		for i := 0; i < len(s); i++ {
			if s[i] == '\n' {
				panic("Write \\n")
			}
		}
	}
	return b.buf.WriteString(s)
}

func (b *printer) push(s string) int {
	n := len(b.prefix)
	b.prefix = append(b.prefix, s...)
	return n
}

func (b *printer) pop(n int) {
	b.prefix = b.prefix[:n]
}
