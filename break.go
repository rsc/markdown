// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

// A ThematicBreak is a [Block] representing a [thematic break],
// usually displayed as a horizontal rule (<hr> tag).
//
// [thematic break]: https://spec.commonmark.org/0.31.2/#thematic-breaks
type ThematicBreak struct {
	Position
}

func (*ThematicBreak) Block() {}

func (b *ThematicBreak) printHTML(p *printer) {
	p.html("<hr />\n")
}

func (b *ThematicBreak) printMarkdown(p *printer) {
	p.maybeNL()
	p.md("***")
}

// startThematicBreak is a [starter] for a [ThematicBreak].
func startThematicBreak(p *parser, s line) (line, bool) {
	if !trimThematicBreak(&s) {
		return s, false
	}
	p.doneBlock(&ThematicBreak{Position{p.lineno, p.lineno}})
	return line{}, true
}

// trimThematicBreak attempts to trim a thematic break from s,
// reporting whether it was successful.
// See https://spec.commonmark.org/0.31.2/#thematic-breaks.
func trimThematicBreak(s *line) bool {
	t := s
	t.trimSpace(0, 3, false)
	c := t.peek()
	if c != '-' && c != '_' && c != '*' {
		return false
	}
	for i := 0; ; i++ {
		if !t.trim(c) {
			if i < 3 {
				return false
			}
			break
		}
		t.skipSpace()
	}
	if !t.eof() {
		return false
	}
	*s = line{}
	return true
}

// A HardBreak is an Inline representing a hard line break (<br> tag).
type HardBreak struct{}

func (*HardBreak) Inline() {}

func (x *HardBreak) printHTML(p *printer) {
	p.html("<br />\n")
}

func (x *HardBreak) printMarkdown(p *printer) {
	p.md(`\`)
	p.nl()
}

func (x *HardBreak) printText(p *printer) {
	p.text("\n")
}

// A SoftBreak is an Inline representing a soft line break (newline character).
type SoftBreak struct{}

func (*SoftBreak) Inline() {}

func (x *SoftBreak) printHTML(p *printer) {
	// TODO: If printer config says to, print <br> instead.
	p.html("\n")
}

func (x *SoftBreak) printMarkdown(p *printer) {
	p.nl()
}

func (x *SoftBreak) printText(p *printer) {
	p.text("\n")
}

// parseBreak is an [inlineParser] for a [SoftBreak] or [HardBreak].
// The caller has checked that s[start] is a newline.
func parseBreak(p *parser, s string, start int) (x Inline, end int, ok bool) {
	// Back up to remove trailing spaces and tabs.
	i := start
	for i > 0 && (s[i-1] == ' ' || s[i-1] == '\t') {
		i--
	}
	if i < start {
		// The caller will do p.emit(start), but we want to skip
		// the spaces and tabs between i and start, so do the
		// emit ourselves followed by skipping to start.
		p.emit(i)
		p.skip(start)
	}

	end = start + 1
	// TODO: Do tabs count? That would be a mess.
	if start >= 2 && s[start-1] == ' ' && s[start-2] == ' ' {
		return &HardBreak{}, end, true
	}
	return &SoftBreak{}, end, true
}
