// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

/*
text node can be

 - other literal text
 - run of * or _ characters
 - [
 - ![

keep delimiter stack pointing at non-other literal text
each node contains

 - type of delimiter [ ![ _ *
 - number of delimiters
 - active or not
 - potential opener, potential closer, or obth

when a ] is hit, call look for link or image
when end is hit, call process emphasis

look for link or image:

	find topmost [ or ![
	if none, emit literal ]
	if its inactive, remove and emit literal ]
	parse ahead to look for rest of link; if none, remove and emit literal ]
	run process emphasis on the interior,
	remove opener
	if this was a link (not an image), set all [ before opener to inactive, to avoid links inside links

process emphasis

	walk forward in list to find a closer.
	walk back to find first potential matching opener.
	if found:
		strong for length >= 2
		insert node
		drop delimiters between opener and closer
		remove 1 or 2 from open/close count, removing if now empty
		if closing has some left, go around again on this node
	if not:
		set openers bottom for this kind of element to before current_position
		if the closer at current pos is not an opener, remove it

seems needlessly complex. two passes

scan and find ` ` first.

pass 1. scan and find [ and ]() and leave the rest alone.

each completed one invokes emphasis on inner text and then on the overall list.

*/

type Inline interface {
	PrintHTML(*bytes.Buffer)
	PrintText(*bytes.Buffer)
	printMarkdown(*bytes.Buffer)
}

type Plain struct {
	Text string
}

func (*Plain) Inline() {}

func (x *Plain) PrintHTML(buf *bytes.Buffer) {
	htmlEscaper.WriteString(buf, x.Text)
}

func (x *Plain) printMarkdown(buf *bytes.Buffer) {
	buf.WriteString(x.Text)
}

func (x *Plain) PrintText(buf *bytes.Buffer) {
	htmlEscaper.WriteString(buf, x.Text)
}

type openPlain struct {
	Plain
	i int // position in input where bracket is
}

type emphPlain struct {
	Plain
	canOpen  bool
	canClose bool
	i        int // position in output where emph is
	n        int // length of original span
}

type Escaped struct {
	Plain
}

func (x *Escaped) printMarkdown(buf *bytes.Buffer) {
	buf.WriteByte('\\')
	x.Plain.printMarkdown(buf)
}

type Code struct {
	Text     string
	numTicks int
}

func (*Code) Inline() {}

func (x *Code) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<code>%s</code>", htmlEscaper.Replace(x.Text))
}

func (x *Code) printMarkdown(buf *bytes.Buffer) {
	ticks := strings.Repeat("`", x.numTicks)
	buf.WriteString(ticks)
	buf.WriteString(x.Text)
	buf.WriteString(ticks)
}

func (x *Code) PrintText(buf *bytes.Buffer) {
	htmlEscaper.WriteString(buf, x.Text)
}

type Strong struct {
	Inner []Inline
	Char  byte // '_' or '*'
}

func (x *Strong) Inline() {
}

func (x *Strong) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<strong>")
	for _, c := range x.Inner {
		c.PrintHTML(buf)
	}
	buf.WriteString("</strong>")
}

func (x *Strong) printMarkdown(buf *bytes.Buffer) {
	buf.WriteByte(x.Char)
	buf.WriteByte(x.Char)
	for _, c := range x.Inner {
		c.printMarkdown(buf)
	}
	buf.WriteByte(x.Char)
	buf.WriteByte(x.Char)
}

func (x *Strong) PrintText(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.PrintText(buf)
	}
}

type Emph struct {
	Inner []Inline
	Char  byte // '_' or '*'
}

func (*Emph) Inline() {}

func (x *Emph) PrintHTML(buf *bytes.Buffer) {
	buf.WriteString("<em>")
	for _, c := range x.Inner {
		c.PrintHTML(buf)
	}
	buf.WriteString("</em>")
}

func (x *Emph) printMarkdown(buf *bytes.Buffer) {
	buf.WriteByte(x.Char)
	for _, c := range x.Inner {
		c.printMarkdown(buf)
	}
	buf.WriteByte(x.Char)
}

func (x *Emph) PrintText(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.PrintText(buf)
	}
}

func (p *Parser) emit(i int) {
	if p.emitted < i {
		p.list = append(p.list, &Plain{p.s[p.emitted:i]})
		p.emitted = i
	}
}

func (p *Parser) skip(i int) {
	p.emitted = i
}

func (p *Parser) inline(s string) []Inline {
	s = strings.Trim(s, " \t")
	// Scan text looking for inlines.
	// Leaf inlines are converted immediately.
	// Non-leaf inlines have potential starts pushed on a stack while we await completion.
	// Links take priority over other emphasis, so the emphasis must be delayed.
	p.s = s
	p.list = nil
	p.emitted = 0
	var opens []int // indexes of open ![ and [ Plains in p.list
	var lastLinkOpen int
	i := 0
	for i < len(s) {
		var parser func(string, int) (Inline, int, int, bool)
		switch s[i] {
		case '\\':
			parser = parseEscape
		case '`':
			parser = parseCodeSpan
		case '<':
			parser = parseAutoLinkOrHTML
		case '[':
			parser = parseLinkOpen
		case '!':
			parser = parseImageOpen
		case '_', '*':
			parser = parseEmph
		case '\n': // TODO what about eof
			parser = parseBreak
		case '&':
			parser = parseHTMLEntity
		}
		if parser != nil {
			if x, start, end, ok := parser(s, i); ok {
				p.emit(start)
				if _, ok := x.(*openPlain); ok {
					opens = append(opens, len(p.list))
				}
				p.list = append(p.list, x)
				i = end
				p.skip(i)
				continue
			}
		}
		if s[i] == ']' && len(opens) > 0 {
			oi := opens[len(opens)-1]
			open := p.list[oi].(*openPlain)
			opens = opens[:len(opens)-1]
			if open.Text[0] == '!' || lastLinkOpen <= open.i {
				if x, end, ok := p.parseLinkClose(s, i, open); ok {
					p.emit(i)
					x.Inner = p.emph(nil, p.list[oi+1:])
					if open.Text[0] == '!' {
						p.list[oi] = (*Image)(x)
					} else {
						p.list[oi] = x
					}
					p.list = p.list[:oi+1]
					p.skip(end)
					i = end
					if open.Text[0] == '[' {
						// No links around links.
						lastLinkOpen = open.i
					}
					continue
				}
			}
		}
		i++
	}
	p.emit(len(s))
	p.list = p.emph(p.list[:0], p.list)
	return p.list
}

func (p *Parser) emph(dst, src []Inline) []Inline {
	var stack [2][]*emphPlain
	stackOf := func(c byte) int {
		if c == '*' {
			return 1
		}
		return 0
	}

	trimStack := func() {
		for i := range stack {
			stk := &stack[i]
			for len(*stk) > 0 && (*stk)[len(*stk)-1].i >= len(dst) {
				*stk = (*stk)[:len(*stk)-1]
			}
		}
	}

	for i := 0; i < len(src); i++ {
		if open, ok := src[i].(*openPlain); ok {
			// Convert unused link/image open marker to plain text.
			dst = append(dst, &open.Plain)
			continue
		}
		p, ok := src[i].(*emphPlain)
		if !ok {
			dst = append(dst, src[i])
			continue
		}
		if p.canClose {
			stk := &stack[stackOf(p.Text[0])]
		Loop:
			for p.Text != "" {
				// Looking for same symbol and compatible with p.Text.
				for i := len(*stk) - 1; i >= 0; i-- {
					start := (*stk)[i]
					if (p.canOpen && p.canClose || start.canOpen && start.canClose) && (p.n+start.n)%3 == 0 && (p.n%3 != 0 || start.n%3 != 0) {
						continue
					}
					var d int
					if len(p.Text) >= 2 && len(start.Text) >= 2 {
						// strong
						d = 2
					} else {
						// emph
						d = 1
					}
					x := &Emph{Char: p.Text[0], Inner: append([]Inline(nil), dst[start.i+1:]...)}
					start.Text = start.Text[:len(start.Text)-d]
					p.Text = p.Text[d:]
					if start.Text == "" {
						dst = dst[:start.i]
					} else {
						dst = dst[:start.i+1]
					}
					trimStack()
					if d == 2 {
						dst = append(dst, (*Strong)(x))
					} else {
						dst = append(dst, x)
					}
					continue Loop
				}
				break
			}
		}
		if p.Text != "" {
			if p.canOpen {
				p.i = len(dst)
				dst = append(dst, p)
				stk := &stack[stackOf(p.Text[0])]
				*stk = append(*stk, p)
			} else {
				dst = append(dst, &p.Plain)
			}
		}
	}
	return dst
}

var mdUnescaper = func() *strings.Replacer {
	var list = []string{
		`\!`, `!`,
		`\"`, `"`,
		`\#`, `#`,
		`\$`, `$`,
		`\%`, `%`,
		`\&`, `&`,
		`\'`, `'`,
		`\(`, `(`,
		`\)`, `)`,
		`\*`, `*`,
		`\+`, `+`,
		`\,`, `,`,
		`\-`, `-`,
		`\.`, `.`,
		`\/`, `/`,
		`\:`, `:`,
		`\;`, `;`,
		`\<`, `<`,
		`\=`, `=`,
		`\>`, `>`,
		`\?`, `?`,
		`\@`, `@`,
		`\[`, `[`,
		`\\`, `\`,
		`\]`, `]`,
		`\^`, `^`,
		`\_`, `_`,
		"\\`", "`",
		`\{`, `{`,
		`\|`, `|`,
		`\}`, `}`,
		`\~`, `~`,
	}

	for name, repl := range htmlEntity {
		list = append(list, name, repl)
	}
	return strings.NewReplacer(list...)
}()

func isPunct(c byte) bool {
	return '!' <= c && c <= '/' || ':' <= c && c <= '@' || '[' <= c && c <= '`' || '{' <= c && c <= '~'
}

func parseEscape(s string, i int) (Inline, int, int, bool) {
	if i+1 < len(s) {
		c := s[i+1]
		if isPunct(c) {
			return &Escaped{Plain{s[i+1 : i+2]}}, i, i + 2, true
		}
		if c == '\n' { // TODO what about eof
			end := i + 2
			for end < len(s) && (s[end] == ' ' || s[end] == '\t') {
				end++
			}
			return &HardBreak{}, i, end, true
		}
	}
	return nil, 0, 0, false
}

func parseCodeSpan(s string, i int) (Inline, int, int, bool) {
	start := i
	// Count leading backticks. Need to find that many again.
	n := 1
	for i+n < len(s) && s[i+n] == '`' {
		n++
	}
	for end := i + n; end < len(s); {
		if s[end] != '`' {
			end++
			continue
		}
		estart := end
		for end < len(s) && s[end] == '`' {
			end++
		}
		if end-estart == n {
			// Match.
			// Line endings are converted to single spaces.
			text := s[i+n : estart]
			text = strings.ReplaceAll(text, "\n", " ")

			// If enclosed text starts and ends with a space and is not all spaces,
			// one space is removed from start and end, to allow `` ` `` to quote a single backquote.
			if len(text) >= 2 && text[0] == ' ' && text[len(text)-1] == ' ' && strings.Trim(text, " ") != "" {
				text = text[1 : len(text)-1]
			}

			return &Code{text, n}, start, end, true
		}
	}

	// No match, so none of these backticks count: skip them all.
	// For example ``x` is not a single backtick followed by a code span.
	// Returning nil, 0, false would advance to the second backtick and try again.
	return &Plain{s[i : i+n]}, start, i + n, true
}

func parseAutoLinkOrHTML(s string, i int) (Inline, int, int, bool) {
	if x, end, ok := parseAutoLinkURI(s, i); ok {
		return x, i, end, true
	}
	if x, end, ok := parseAutoLinkEmail(s, i); ok {
		return x, i, end, true
	}
	if x, end, ok := parseHTMLTag(s, i); ok {
		return x, i, end, true
	}
	return nil, 0, 0, false
}
func isLetter(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

func isLDH(c byte) bool {
	return isLetterDigit(c) || c == '-'
}

func isLetterDigit(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
}

func parseLinkOpen(s string, i int) (Inline, int, int, bool) {
	return &openPlain{Plain{s[i : i+1]}, i + 1}, i, i + 1, true
}

func parseImageOpen(s string, i int) (Inline, int, int, bool) {
	if i+1 < len(s) && s[i+1] == '[' {
		return &openPlain{Plain{s[i : i+2]}, i + 2}, i, i + 2, true
	}
	return nil, 0, 0, false
}

func parseEmph(s string, i int) (Inline, int, int, bool) {
	c := s[i]
	j := i + 1
	for j < len(s) && s[j] == c {
		j++
	}

	var before, after rune
	if i == 0 {
		before = ' '
	} else {
		before, _ = utf8.DecodeLastRuneInString(s[:i])
	}
	if j >= len(s) {
		after = ' '
	} else {
		after, _ = utf8.DecodeRuneInString(s[j:])
	}

	// “A left-flanking delimiter run is a delimiter run that is
	// (1) not followed by Unicode whitespace, and either
	// (2a) not followed by a Unicode punctuation character, or
	// (2b) followed by a Unicode punctuation character
	// and preceded by Unicode whitespace or a Unicode punctuation character.
	// For purposes of this definition, the beginning and the end
	// of the line count as Unicode whitespace.”
	leftFlank := !isUnicodeSpace(after) &&
		(!isUnicodePunct(after) || isUnicodeSpace(before) || isUnicodePunct(before))

	// “A right-flanking delimiter run is a delimiter run that is
	// (1) not preceded by Unicode whitespace, and either
	// (2a) not preceded by a Unicode punctuation character, or
	// (2b) preceded by a Unicode punctuation character
	// and followed by Unicode whitespace or a Unicode punctuation character.
	// For purposes of this definition, the beginning and the end
	// of the line count as Unicode whitespace.”
	rightFlank := !isUnicodeSpace(before) &&
		(!isUnicodePunct(before) || isUnicodeSpace(after) || isUnicodePunct(after))

	var canOpen, canClose bool

	if c == '*' {
		// “A single * character can open emphasis iff
		// it is part of a left-flanking delimiter run.”

		// “A double ** can open strong emphasis iff
		// it is part of a left-flanking delimiter run.”
		canOpen = leftFlank

		// “A single * character can close emphasis iff
		// it is part of a right-flanking delimiter run.”

		// “A double ** can close strong emphasis iff
		// it is part of a right-flanking delimiter run.”
		canClose = rightFlank
	} else {
		// “A single _ character can open emphasis iff
		// it is part of a left-flanking delimiter run and either
		// (a) not part of a right-flanking delimiter run or
		// (b) part of a right-flanking delimiter run preceded by a Unicode punctuation character.”

		// “A double __ can open strong emphasis iff
		// it is part of a left-flanking delimiter run and either
		// (a) not part of a right-flanking delimiter run or
		// (b) part of a right-flanking delimiter run preceded by a Unicode punctuation character.”
		canOpen = leftFlank && (!rightFlank || isUnicodePunct(before))

		// “A single _ character can close emphasis iff
		// it is part of a right-flanking delimiter run and either
		// (a) not part of a left-flanking delimiter run or
		// (b) part of a left-flanking delimiter run followed by a Unicode punctuation character.”

		// “A double __ can close strong emphasis iff
		// it is part of a right-flanking delimiter run and either
		// (a) not part of a left-flanking delimiter run or
		// (b) part of a left-flanking delimiter run followed by a Unicode punctuation character.”
		canClose = rightFlank && (!leftFlank || isUnicodePunct(after))
	}

	return &emphPlain{Plain: Plain{s[i:j]}, canOpen: canOpen, canClose: canClose, n: j - i}, i, j, true
}

func isUnicodeSpace(r rune) bool {
	if r < 0x80 {
		return r == ' ' || r == '\t' || r == '\n'
	}
	return unicode.In(r, unicode.Zs)
}

func isUnicodePunct(r rune) bool {
	if r < 0x80 {
		return isPunct(byte(r))
	}
	return unicode.In(r, unicode.Punct)
}

func (p *Parser) parseLinkClose(s string, i int, open *openPlain) (*Link, int, bool) {
	if i+1 < len(s) {
		switch s[i+1] {
		case '(':
			// Inline link - [Text](Dest Title), with Title omitted or both Dest and Title omitted.
			i := skipSpace(s, i+2)
			var dest, title string
			var titleChar byte
			if i < len(s) && s[i] != ')' {
				var ok bool
				dest, i, ok = parseLinkDest(s, i)
				if !ok {
					break
				}
				i = skipSpace(s, i)
				if i < len(s) && s[i] != ')' {
					title, titleChar, i, ok = parseLinkTitle(s, i)
					if !ok {
						break
					}
					i = skipSpace(s, i)
				}
			}
			if i < len(s) && s[i] == ')' {
				return &Link{URL: dest, Title: title, TitleChar: titleChar}, i + 1, true
			}
			// NOTE: Test malformed ( ) with shortcut reference
			// TODO fall back on syntax error?

		case '[':
			// Full reference link - [Text][Label]
			label, i, ok := parseLinkLabel(s, i+1)
			if !ok {
				break
			}
			if link, ok := p.links[normalizeLabel(label)]; ok {
				return &Link{URL: link.URL, Title: link.Title}, i, true
			}
			// Note: Could break here, but CommonMark dingus does not
			// fall back to trying Text for [Text][Label] when Label is unknown.
			// Unclear from spec what the correct answer is.
			return nil, 0, false
		}
	}

	// Collapsed or shortcut reference link: [Text][] or [Text].
	end := i + 1
	if strings.HasPrefix(s[end:], "[]") {
		end += 2
	}

	if link, ok := p.links[normalizeLabel(s[open.i:i])]; ok {
		return &Link{URL: link.URL, Title: link.Title}, end, true
	}
	return nil, 0, false
}

func skipSpace(s string, i int) int {
	// Note: Blank lines have already been removed.
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	return i
}
