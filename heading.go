// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"strings"
)

// A Heading is a [Block] representing an [ATX heading] or
// [Setext heading], usually displayed with the <h1> through <h6> tags.
//
// [ATX heading]: https://spec.commonmark.org/0.31.2/#atx-headings
// [Setext heading]: https://spec.commonmark.org/0.31.2/#setext-headings
type Heading struct {
	Position

	// Level is the heading level: 1 through 6.
	// Other values are clamped to the valid range.
	Level int

	// Text is the text of the heading.
	Text *Text

	// ID is the HTML id attribute.
	// The parser populates this field if [Parser.HeadingID] is true
	// and the heading ends with text like "{#id}".
	ID string
}

func (*Heading) Block() {}

// level returns the effective level, clamping Level to the range [1, 6].
func (h *Heading) level() int {
	return max(1, min(6, h.Level))
}

func (b *Heading) printHTML(p *printer) {
	fmt.Fprintf(p, "<h%d", b.level())
	if b.ID != "" {
		fmt.Fprintf(p, ` id="%s"`, htmlEscaper.Replace(b.ID))
	}
	p.WriteByte('>')
	b.Text.printHTML(p)
	fmt.Fprintf(p, "</h%d>\n", b.level())
}

func (b *Heading) printMarkdown(p *printer) {
	p.maybeNL()

	// TODO: handle setext headings properly.
	for i := b.level(); i > 0; i-- {
		p.WriteByte('#')
	}
	p.WriteByte(' ')
	b.Text.printMarkdown(p)
	if b.ID != "" {
		fmt.Fprintf(p, " {#%s}", b.ID)
	}
}

// startATXHeading is a [starter] for an ATX [Heading], like "## Heading".
//
// See https://spec.commonmark.org/0.31.2/#atx-headings.
func startATXHeading(p *parser, s line) (line, bool) {
	n, ok := trimATX(&s)
	if !ok {
		return s, false
	}
	text := trimRightSpaceTab(s.string())

	// Remove any number of trailing '#'s if preceded by a space or tab.
	if inner := strings.TrimRight(text, "#"); inner != trimRightSpaceTab(inner) || inner == "" {
		text = inner
	}

	// Extract id if extension is enabled.
	var id string
	if p.HeadingID {
		// Extension: Parse and remove ID attribute.
		// It must come before trailing '#'s to more closely follow the spec:
		//    The optional closing sequence of #s must be preceded by spaces or tabs
		//    and may be followed by spaces or tabs only.
		// But Goldmark allows it to come after.
		text, id = trimHeadingID(p, text)
	}

	pos := Position{p.lineno, p.lineno}
	p.doneBlock(&Heading{pos, n, p.newText(pos, text), id}) // TODO rename doneBlock?
	return line{}, true
}

// trimHeadingID trims an {#id} suffix from s if one is present,
// returning the prefix before the {#id} and the id.
// If there is no {#id} suffix, trimID returns s, "".
// The {#id} suffix can be followed by spaces, which are
// ignored and discarded.
func trimHeadingID(p *parser, s string) (text, id string) {
	text = s // failure result
	i := strings.LastIndexByte(s, '{')
	if i < 0 {
		return
	}
	j := i + strings.IndexByte(s[i:], '}')
	if j < i || trimRightSpaceTab(s[j+1:]) != "" {
		return
	}
	if j == i+1 || j == i+2 && s[i+1] == '#' {
		p.corner = true // goldmark accepts {} and {#}
		return
	}
	if s[i+1] != '#' {
		return
	}
	text, id = s[:i], strings.TrimSpace(s[i+2:j]) // TODO maybe trimSpace?

	// Goldmark is strict about the id syntax.
	for i := range len(id) {
		if c := id[i]; c >= 0x80 || !isLetterDigit(byte(c)) {
			p.corner = true
		}
	}

	return
}

// startSetextHeading is a [starter] for a Setext [Heading], which is an
// underlined paragraph of text. The parargraph is assumed to have
// been parsed already; startSetextHeading looks for the underline.
//
// See https://spec.commonmark.org/0.31.2/#setext-headings.
func startSetextHeading(p *parser, s line) (line, bool) {
	// Topmost block must be a paragraph.
	if p.nextB() != p.para() {
		return s, false
	}

	// Need Setext underline.
	t := s
	level, ok := trimSetext(&t)
	if !ok {
		return s, false
	}

	// The Setext heading forces an end-of-paragraph,
	// but this still may not be a Setext heading if the paragraph
	// closer decides this wasn't a paragraph after all.
	// Might turn out to be a link reference, for example.
	// Close active paragraph to find out.
	p.closeBlock()
	para, ok := p.last().(*Paragraph)
	if !ok {
		// Paragraph text didn't end in a pargraph after all.
		// Leave underline text for processing by something else.
		return s, false
	}

	p.deleteLast()
	p.doneBlock(&Heading{Position{para.StartLine, p.lineno}, level, para.Text, ""})
	return line{}, true
}

// trimATX trims an ATX heading prefix
// (optional spaces and then 1-6 #s followd by a space) from s.
// reporting the heading level and whether it was successful.
// If trimATX is unsuccessful, it leaves s unmodified.
func trimATX(s *line) (level int, ok bool) {
	t := *s
	t.trimSpace(0, 3, false)
	if !t.trim('#') {
		return
	}
	n := 1
	for n < 6 && t.trim('#') {
		n++
	}
	if !t.trimSpace(1, 1, true) {
		return
	}
	*s = t
	return n, true
}

// trimSetext trims a Setext heading underline
// (optional spaces and then only -'s or ='s
// followed by optional spaces and EOL) from s,
// reporting the leading level and whether it was successful.
// If trimSetext is unsuccessful, it leaves s unmodiifed.
func trimSetext(s *line) (level int, ok bool) {
	t := *s
	t.trimSpace(0, 3, false)
	c := t.peek()
	if c != '-' && c != '=' {
		return
	}
	for t.trim(c) {
	}
	t.skipSpace()
	if !t.eof() {
		return
	}
	level = 1
	if c == '-' {
		level = 2
	}
	*s = line{}
	return level, true
}
