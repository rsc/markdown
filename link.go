// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"golang.org/x/text/cases"
)

func parseLinkRefDef(p buildState, s string) (int, bool) {
	// “A link reference definition consists of a link label,
	// optionally preceded by up to three spaces of indentation,
	// followed by a colon (:),
	// optional spaces or tabs (including up to one line ending),
	// a link destination,
	// optional spaces or tabs (including up to one line ending),
	// and an optional link title,
	// which if it is present must be separated from the link destination
	// by spaces or tabs. No further character may occur.”
	i := skipSpace(s, 0)
	label, i, ok := parseLinkLabel(s, i)
	if !ok || i >= len(s) || s[i] != ':' {
		return 0, false
	}
	i = skipSpace(s, i+1)
	dest, i, ok := parseLinkDest(s, i)
	if !ok {
		return 0, false
	}
	moved := false
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		moved = true
		i++
	}

	// Take title if present and doesn't break parse.
	j := i
	if j >= len(s) || s[j] == '\n' {
		moved = true
		if j < len(s) {
			j++
		}
	}

	var title string
	var titleChar byte
	if moved {
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if t, c, j, ok := parseLinkTitle(s, j); ok {
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j >= len(s) || s[j] == '\n' {
				i = j
				title = t
				titleChar = c
			}
		}
	}

	// Must end line. Already trimmed spaces.
	if i < len(s) && s[i] != '\n' {
		return 0, false
	}
	if i < len(s) {
		i++
	}

	label = normalizeLabel(label)
	if p.link(label) == nil {
		p.defineLink(label, &Link{URL: dest, Title: title, TitleChar: titleChar})
	}
	return i, true
}

func parseLinkTitle(s string, i int) (title string, char byte, next int, found bool) {
	if i < len(s) && (s[i] == '"' || s[i] == '\'' || s[i] == '(') {
		want := s[i]
		if want == '(' {
			want = ')'
		}
		j := i + 1
		for ; j < len(s); j++ {
			if s[j] == want {
				title := s[i+1 : j]
				// TODO: Validate title?
				return mdUnescaper.Replace(title), want, j + 1, true
			}
			if s[j] == '(' && want == ')' {
				break
			}
			if s[j] == '\\' && j+1 < len(s) {
				j++
			}
		}
	}
	return "", 0, 0, false
}

func parseLinkLabel(s string, i int) (string, int, bool) {
	// “A link label begins with a left bracket ([) and ends with
	// the first right bracket (]) that is not backslash-escaped.
	// Between these brackets there must be at least one character
	// that is not a space, tab, or line ending.
	// Unescaped square bracket characters are not allowed
	// inside the opening and closing square brackets of link labels.
	// A link label can have at most 999 characters inside the square brackets.”
	if i >= len(s) || s[i] != '[' {
		return "", 0, false
	}
	j := i + 1
	for ; j < len(s); j++ {
		if s[j] == ']' {
			if j-(i+1) > 999 {
				break
			}
			if label := strings.Trim(s[i+1:j], " \t\n"); label != "" {
				// Note: CommonMark Dingus does not escape.
				return label, j + 1, true
			}
			break
		}
		if s[j] == '[' {
			break
		}
		if s[j] == '\\' && j+1 < len(s) {
			j++
		}
	}
	return "", 0, false
}

func normalizeLabel(s string) string {
	// “To normalize a label, strip off the opening and closing brackets,
	// perform the Unicode case fold, strip leading and trailing spaces, tabs, and line endings,
	// and collapse consecutive internal spaces, tabs, and line endings to a single space.”
	s = strings.Trim(s, " \t\n")
	var b strings.Builder
	space := false
	hi := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n':
			space = true
			continue
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c >= 0x80 {
				hi = true
			}
			b.WriteByte(c)
		}
	}
	s = b.String()
	if hi {
		s = cases.Fold().String(s)
	}
	return s
}

func parseLinkDest(s string, i int) (string, int, bool) {
	if i >= len(s) {
		return "", 0, false
	}

	// “A sequence of zero or more characters between an opening < and a closing >
	// that contains no line endings or unescaped < or > characters,”
	if s[i] == '<' {
		for j := i + 1; ; j++ {
			if j >= len(s) || s[j] == '\n' || s[j] == '<' {
				return "", 0, false
			}
			if s[j] == '>' {
				// TODO unescape?
				return mdUnescaper.Replace(s[i+1 : j]), j + 1, true
			}
			if s[j] == '\\' {
				j++
			}
		}
	}

	// “or a nonempty sequence of characters that does not start with <,
	// does not include ASCII control characters or space character,
	// and includes parentheses only if (a) they are backslash-escaped
	// or (b) they are part of a balanced pair of unescaped parentheses.
	depth := 0
	j := i
Loop:
	for ; j < len(s); j++ {
		switch s[j] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				break Loop
			}
			depth--
		case '\\':
			if j+1 < len(s) {
				j++
			}
		case ' ', '\t', '\n':
			break Loop
		}
	}

	dest := s[i:j]
	// TODO: Validate dest?
	// TODO: Unescape?
	// NOTE: CommonMark Dingus does not reject control characters.
	return mdUnescaper.Replace(dest), j, true
}

func parseAutoLinkURI(s string, i int) (Inline, int, bool) {
	// CommonMark 0.30:
	//
	//	For purposes of this spec, a scheme is any sequence of 2–32 characters
	//	beginning with an ASCII letter and followed by any combination of
	//	ASCII letters, digits, or the symbols plus (”+”), period (”.”), or
	//	hyphen (”-”).
	//
	//	An absolute URI, for these purposes, consists of a scheme followed by
	//	a colon (:) followed by zero or more characters other ASCII control
	//	characters, space, <, and >. If the URI includes these characters,
	//	they must be percent-encoded (e.g. %20 for a space).

	j := i
	if j+1 >= len(s) || s[j] != '<' || !isLetter(s[j+1]) {
		return nil, 0, false
	}
	j++
	for j < len(s) && isScheme(s[j]) && j-(i+1) <= 32 {
		j++
	}
	if j-(i+1) < 2 || j-(i+1) > 32 || j >= len(s) || s[j] != ':' {
		return nil, 0, false
	}
	j++
	for j < len(s) && isURL(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '>' {
		return nil, 0, false
	}
	link := s[i+1 : j]
	// link = mdUnescaper.Replace(link)
	return &AutoLink{link, link}, j + 1, true
}

func parseAutoLinkEmail(s string, i int) (Inline, int, bool) {
	// CommonMark 0.30:
	//
	//	An email address, for these purposes, is anything that matches
	//	the non-normative regex from the HTML5 spec:
	//
	//	/^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/

	j := i
	if j+1 >= len(s) || s[j] != '<' || !isUser(s[j+1]) {
		return nil, 0, false
	}
	j++
	for j < len(s) && isUser(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '@' {
		return nil, 0, false
	}
	for {
		j++
		n, ok := skipDomainElem(s[j:])
		if !ok {
			return nil, 0, false
		}
		j += n
		if j >= len(s) || s[j] != '.' && s[j] != '>' {
			return nil, 0, false
		}
		if s[j] == '>' {
			break
		}
	}
	email := s[i+1 : j]
	return &AutoLink{email, "mailto:" + email}, j + 1, true
}

func isUser(c byte) bool {
	if isLetterDigit(c) {
		return true
	}
	s := ".!#$%&'*+/=?^_`{|}~-"
	for i := 0; i < len(s); i++ {
		if c == s[i] {
			return true
		}
	}
	return false
}

func isHexDigit(c byte) bool {
	return 'A' <= c && c <= 'F' || 'a' <= c && c <= 'f' || '0' <= c && c <= '9'
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func skipDomainElem(s string) (int, bool) {
	// String of LDH, up to 63 in length, with LetterDigit
	// at both ends (1-letter/digit names are OK).
	// Aka /[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?/.
	if len(s) < 1 || !isLetterDigit(s[0]) {
		return 0, false
	}
	i := 1
	for i < len(s) && isLDH(s[i]) && i <= 63 {
		i++
	}
	if i > 63 || !isLetterDigit(s[i-1]) {
		return 0, false
	}
	return i, true
}

func isScheme(c byte) bool {
	return isLetterDigit(c) || c == '+' || c == '.' || c == '-'
}

func isURL(c byte) bool {
	return c > ' ' && c != '<' && c != '>'
}

type AutoLink struct {
	Text string
	URL  string
}

func (*AutoLink) Inline() {}

func (x *AutoLink) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<a href=\"%s\">%s</a>", htmlLinkEscaper.Replace(x.URL), htmlEscaper.Replace(x.Text))
}

func (x *AutoLink) printMarkdown(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<%s>", x.Text)
}

func (x *AutoLink) PrintText(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "%s", htmlEscaper.Replace(x.Text))
}

type Link struct {
	Inner     []Inline
	URL       string
	Title     string
	TitleChar byte // ', " or )
}

func (*Link) Inline() {}

func (x *Link) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<a href=\"%s\"", htmlLinkEscaper.Replace(x.URL))
	if x.Title != "" {
		fmt.Fprintf(buf, " title=\"%s\"", htmlQuoteEscaper.Replace(x.Title))
	}
	buf.WriteString(">")
	for _, c := range x.Inner {
		c.PrintHTML(buf)
	}
	buf.WriteString("</a>")
}

func (x *Link) printMarkdown(buf *bytes.Buffer) {
	buf.WriteByte('[')
	x.printRemainingMarkdown(buf)
}

func (x *Link) printRemainingMarkdown(buf *bytes.Buffer) {
	// TODO(jba): escaping
	for _, c := range x.Inner {
		c.printMarkdown(buf)
	}
	buf.WriteString("](")
	buf.WriteString(x.URL)
	if x.Title != "" {
		closeChar := x.TitleChar
		openChar := closeChar
		if openChar == ')' {
			openChar = '('
		}
		fmt.Fprintf(buf, " %c%s%c", openChar, x.Title /*TODO: escape*/, closeChar)
	}
	buf.WriteByte(')')
}

func (x *Link) PrintText(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.PrintText(buf)
	}
}

type Image struct {
	Inner     []Inline
	URL       string
	Title     string
	TitleChar byte
}

func (*Image) Inline() {}

func (x *Image) PrintHTML(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<img src=\"%s\"", htmlLinkEscaper.Replace(x.URL))
	fmt.Fprintf(buf, " alt=\"")
	for _, c := range x.Inner {
		c.PrintText(buf)
	}
	fmt.Fprintf(buf, "\"")
	if x.Title != "" {
		fmt.Fprintf(buf, " title=\"%s\"", htmlQuoteEscaper.Replace(x.Title))
	}
	buf.WriteString(" />")
}

func (x *Image) printMarkdown(buf *bytes.Buffer) {
	buf.WriteString("![")
	(*Link)(x).printRemainingMarkdown(buf)
}

func (x *Image) PrintText(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.PrintText(buf)
	}
}
