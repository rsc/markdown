// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/cases"
)

// Note: Link and Image are the same underlying struct by design,
// so that code can safely convert between *Link and *Image.

// A Link is an [Inline] representing a [link] (<a> tag).
//
// [link]: https://spec.commonmark.org/0.31.2/#links
type Link struct {
	Inner     Inlines
	URL       string
	Title     string
	TitleChar byte // ', " or )
}

// An Image is an [Inline] representing an [image] (<a> tag).
//
// [image]: https://spec.commonmark.org/0.31.2/#images
type Image struct {
	Inner     Inlines
	URL       string
	Title     string
	TitleChar byte
}

func (*Link) Inline() {}

func (x *Link) printHTML(p *printer) {
	p.html(`<a href="`, htmlLinkEscaper.Replace(x.URL), `"`)
	if x.Title != "" {
		p.html(" title=\"")
		p.html(htmlEscaper.Replace(x.Title))
		p.html("\"")
	}
	p.html(">")
	for _, c := range x.Inner {
		c.printHTML(p)
	}
	p.html("</a>")
}

func (x *Link) printMarkdown(p *printer) {
	p.WriteByte('[')
	for _, c := range x.Inner {
		c.printMarkdown(p)
	}
	p.WriteString("](")
	u := mdLinkEscaper.Replace(x.URL)
	if u == "" || strings.ContainsAny(u, " ") {
		u = "<" + u + ">"
	}
	p.WriteString(u)
	printLinkTitleMarkdown(p, x.Title, x.TitleChar)
	p.WriteByte(')')
}

func printLinkTitleMarkdown(p *printer, title string, titleChar byte) {
	if title == "" {
		return
	}
	if titleChar == 0 {
		titleChar = '\''
	}
	closeChar := titleChar
	openChar := closeChar
	if openChar == ')' {
		openChar = '('
	}
	p.WriteString(" ")
	p.WriteByte(openChar)
	for i, line := range strings.Split(mdEscaper.Replace(title), "\n") {
		if i > 0 {
			p.nl()
		}
		p.WriteString(line)
		p.noTrim()
	}
	p.WriteByte(closeChar)
}

func (x *Link) printText(p *printer) {
	for _, c := range x.Inner {
		c.printText(p)
	}
}

func (*Image) Inline() {}

func (x *Image) printHTML(p *printer) {
	p.html(`<img src="`, htmlLinkEscaper.Replace(x.URL), `" alt="`)
	i := p.buf.Len()
	x.printText(p)
	// GitHub and Goldmark both rewrite \n to space
	// but the Dingus does not.
	// The spec says title can be split across lines but not
	// what happens at that point.
	out := p.buf.Bytes()
	for ; i < len(out); i++ {
		if out[i] == '\n' {
			out[i] = ' '
		}
	}
	p.html(`"`)
	if x.Title != "" {
		p.html(` title="`)
		p.text(x.Title)
		p.html(`"`)
	}
	p.html(` />`)
}

func (x *Image) printMarkdown(p *printer) {
	p.WriteString("!")
	(*Link)(x).printMarkdown(p)
}

func (x *Image) printText(p *printer) {
	for _, c := range x.Inner {
		c.printText(p)
	}
}

// parseLinkOpen is an [inlineParser] for a link open [.
// The caller has checked that s[start] == '['.
func parseLinkOpen(p *parser, s string, start int) (x Inline, end int, ok bool) {
	if p.Footnote {
		if x, end, ok := parseFootnoteRef(p, s, start); ok {
			return x, end, ok
		}
	}
	return &openPlain{Plain{s[start : start+1]}, start + 1}, start + 1, true
}

// parseImageOpen is an [inlineParser] for a link open ![.
// The caller has checked that s[start] == '!'.
func parseImageOpen(_ *parser, s string, start int) (x Inline, end int, ok bool) {
	if start+1 < len(s) && s[start+1] == '[' {
		return &openPlain{Plain{s[start : start+2]}, start + 2}, start + 2, true
	}
	return
}

// parseLinkClose parses a link (or image) close ] or ](target) matching open.
func parseLinkClose(p *parser, s string, start int, open *openPlain) (*Link, int, bool) {
	i := start
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
					if title == "" {
						p.corner = true
					}
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
			label, i, ok := parseLinkLabel(p, s, i+1)
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

// printLinks prints the links in the map, sorted by key,
// as a sequence of [link reference definitions].
//
// [link reference definitions]: https://spec.commonmark.org/0.31.2/#link-reference-definitions
func printLinks(p *printer, links map[string]*Link) {
	// Print links sorted by keys for deterministic output.
	var keys []string
	for k := range links {
		if k != "" {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	for _, k := range keys {
		l := links[k]
		u := l.URL
		if u == "" || strings.ContainsAny(u, " ") {
			u = "<" + u + ">"
		}
		fmt.Fprintf(p, "[%s]: %s", k, u)
		printLinkTitleMarkdown(p, l.Title, l.TitleChar)
		p.nl()
	}
}

// parseLinkRefDef parses and saves in p a [link reference definition]
// at the start of s, if any.
// It returns the length of the link reference definition
// and whether one was found.
//
// [link reference definition]: https://spec.commonmark.org/0.31.2/#link-reference-definitions
func parseLinkRefDef(p *parser, s string) (int, bool) {
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
	label, i, ok := parseLinkLabel(p, s, i)
	if !ok || i >= len(s) || s[i] != ':' {
		return 0, false
	}
	i = skipSpace(s, i+1)
	suf := s[i:]
	dest, i, ok := parseLinkDest(s, i)
	if !ok {
		if suf != "" && suf[0] == '<' {
			// Goldmark treats <<> as a link definition.
			p.corner = true
		}
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
				if t == "" {
					// Goldmark adds title="" in this case.
					// We do not, nor does the Dingus.
					p.corner = true
				}
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

// parseLinkTitle parses a [link title] at s[i:], returning
// the terminating character, one of " ' or );
// the index just past the end of the link;
// and whether a link was found at all.
//
// [link title]: https://spec.commonmark.org/0.31.2/#link-title
func parseLinkTitle(s string, i int) (title string, char byte, end int, found bool) {
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

// parseLinkLabel parses a [link label] at s[i:], returning
// the label, the end index just past the label, and
// whether a label was found at all.
//
// [link label]: https://spec.commonmark.org/0.31.2/#link-label
func parseLinkLabel(p *parser, s string, i int) (string, int, bool) {
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
				// Goldmark does not apply 999 limit.
				p.corner = true
				break
			}
			if label := trimSpaceTabNewline(s[i+1 : j]); label != "" {
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

// normalizeLabel returns the normalized label for s, for uniquely identifying that label.
func normalizeLabel(s string) string {
	if strings.Contains(s, "[") || strings.Contains(s, "]") {
		// Labels cannot have [ ] so avoid the work of translating.
		// This is especially important for pathlogical cases like
		// [[[[[[[[[[a]]]]]]]]]] which would otherwise generate quadratic
		// amounts of garbage.
		return ""
	}

	// “To normalize a label, strip off the opening and closing brackets,
	// perform the Unicode case fold, strip leading and trailing spaces, tabs, and line endings,
	// and collapse consecutive internal spaces, tabs, and line endings to a single space.”
	s = trimSpaceTabNewline(s)
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
		// TODO: Avoid golang.org/x/text/cases.
		// It is more general than we need since we are not passing in a specific language.
		// Also, https://www.unicode.org/faq/casemap_charprop.html#2 says
		// “case-folded text should be used solely for internal processing and
		// generally should not be stored or displayed to the end user.”
		// But we use this string as the map key in p.links and then
		// display it in printLinks.
		// We should probably record the actual label separate from the folded one.
		// Table at https://www.unicode.org/Public/12.1.0/ucd/CaseFolding.txt.
		s = cases.Fold().String(s)
	}
	return s
}

// parseLinkDest parses a [link destination] at s[i:], returning
// the destination, the end index just past the destination,
// and whether a destination was found.
//
// [link destination]: https://spec.commonmark.org/0.31.2/#link-destination
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
				return mdUnescape(s[i+1 : j]), j + 1, true
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
			if depth > 32 {
				// Avoid quadratic inputs by stopping if too deep.
				// This is the same depth that cmark-gfm uses.
				return "", 0, false
			}
		case ')':
			if depth == 0 {
				break Loop
			}
			depth--
		case '\\':
			if j+1 < len(s) {
				if s[j+1] == ' ' || s[j+1] == '\t' {
					return "", 0, false
				}
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
	return mdUnescape(dest), j, true
}

// An AutoLink is an [Inline] representing an [autolink],
// which is an absolute URL or email address inside < > brackets.
//
// [autolink]: https://spec.commonmark.org/0.31.2/#autolinks
type AutoLink struct {
	Text string
	URL  string
}

func (*AutoLink) Inline() {}

func (x *AutoLink) printHTML(p *printer) {
	p.html(`<a href="`, htmlLinkEscaper.Replace(x.URL), `">`)
	p.text(x.Text)
	p.html(`</a>`)
}

func (x *AutoLink) printMarkdown(p *printer) {
	fmt.Fprintf(p, "<%s>", x.Text)
}

func (x *AutoLink) printText(p *printer) {
	p.text(x.Text)
}

// parseAutoLinkURI is an [inlineParser] for a URI [AutoLink].
// The caller has checked that s[start] == '<'.
func parseAutoLinkURI(s string, i int) (x Inline, end int, ok bool) {
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
		return
	}
	j++
	for j < len(s) && isScheme(s[j]) && j-(i+1) <= 32 {
		j++
	}
	if j-(i+1) < 2 || j-(i+1) > 32 || j >= len(s) || s[j] != ':' {
		return
	}
	j++
	for j < len(s) && isURL(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '>' {
		return
	}
	link := s[i+1 : j]
	// link = mdUnescaper.Replace(link)
	return &AutoLink{link, link}, j + 1, true
}

// parseAutoLinkEmail is an [inlineParser] for an email [AutoLink].
// The caller has checked that s[start] == '<'.
func parseAutoLinkEmail(s string, i int) (x Inline, end int, ok bool) {
	// CommonMark 0.30:
	//
	//	An email address, for these purposes, is anything that matches
	//	the non-normative regex from the HTML5 spec:
	//
	//	/^[a-zA-Z0-9.!#$%&'*+/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$/

	j := i
	if j+1 >= len(s) || s[j] != '<' || !isUser(s[j+1]) {
		return
	}
	j++
	for j < len(s) && isUser(s[j]) {
		j++
	}
	if j >= len(s) || s[j] != '@' {
		return
	}
	for {
		j++
		n, ok1 := skipDomainElem(s[j:])
		if !ok1 {
			return
		}
		j += n
		if j >= len(s) || s[j] != '.' && s[j] != '>' {
			return
		}
		if s[j] == '>' {
			break
		}
	}
	email := s[i+1 : j]
	return &AutoLink{email, "mailto:" + email}, j + 1, true
}

// skipDomainElem reports the length of a leading domain element in s,
// along with whether there is one.
// TODO quadratic.
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

// isUser reports whether c is an email user byte.
func isUser(c byte) bool {
	// A-Za-z0-9 plus ".!#$%&'*+/=?^_`{|}~-"
	return c == '!' ||
		'#' <= c && c <= '\'' ||
		'*' <= c && c <= '+' ||
		'-' <= c && c <= '9' ||
		c == '=' ||
		c == '?' ||
		'A' <= c && c <= 'Z' ||
		'^' <= c && c <= '`' ||
		'a' <= c && c <= 'z' ||
		'{' <= c && c <= '~'
}

// isScheme reports whether c is a scheme character.
func isScheme(c byte) bool {
	return isLetterDigit(c) || c == '+' || c == '.' || c == '-'
}

// isURL reports whether c is a URL character.
func isURL(c byte) bool {
	return c > ' ' && c != '<' && c != '>'
}

// GitHub Flavored Markdown autolinks extension
// https://github.github.com/gfm/#autolinks-extension-

// autoLinkText rewrites any extended autolinks in the body
// and returns the result.
//
// list is a list of Plain, Emph, Strong, and Del nodes.
// There are no Link nodes.
//
// The GitHub “spec” declares that “autolinks can only come at the
// beginning of a line, after whitespace, or any of the delimiting
// characters *, _, ~, and (”. However, the GitHub web site does not
// enforce this rule: text like "$abc@def.ghi is my email" links the
// text following the $ as an email address. It appears the actual rule
// is that autolinks cannot come after ASCII letters, although they can
// come after numbers or Unicode letters.
// Since the only point of implementing GitHub Flavored Markdown
// is to match GitHub's behavior, we do what they do, not what they say,
// at least for now.
//
// [GitHub “spec”]: https://github.github.com/gfm/
func autoLinkText(p *parser, list []Inline) []Inline {
	if !p.AutoLinkText {
		return list
	}

	var out []Inline // allocated lazily when we first change list
	for i, x := range list {
		switch x := x.(type) {
		case *Plain:
			if rewrite := autoLinkPlain(p, x.Text); rewrite != nil {
				if out == nil {
					out = append(out, list[:i]...)
				}
				out = append(out, rewrite...)
				continue
			}
		case *Strong:
			x.Inner = autoLinkText(p, x.Inner)
		case *Del:
			x.Inner = autoLinkText(p, x.Inner)
		case *Emph:
			x.Inner = autoLinkText(p, x.Inner)
		}
		if out != nil {
			out = append(out, x)
		}
	}
	if out == nil {
		return list
	}
	return out
}

// autoLinkPlain looks for text to auto-link in the plain text s.
// If it finds any, it returns an Inlines that should replace Plain{s}.
func autoLinkPlain(p *parser, s string) Inlines {
	vd := &validDomainChecker{s: s}
	var out []Inline
Restart:
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '@' {
			if before, link, after, ok := parseAutoEmail(p, s, i); ok {
				if before != "" {
					out = append(out, &Plain{Text: before})
				}
				out = append(out, link)
				vd.removePrefix(len(s) - len(after))
				s = after
				goto Restart
			}
		}

		// Might this be http:// https:// mailto:// xmpp:// or www. ?
		if (c == 'h' || c == 'm' || c == 'x' || c == 'w') && (i == 0 || !isLetter(s[i-1])) {
			if link, after, ok := parseAutoURL(p, s, i, vd); ok {
				if i > 0 {
					out = append(out, &Plain{Text: s[:i]})
				}
				out = append(out, link)
				vd.removePrefix(len(s) - len(after))
				s = after
				goto Restart
			}
		}
	}
	if out == nil {
		return nil
	}
	out = append(out, &Plain{Text: s})
	return out
}

// parseAutoURL parses an [extended URL autolink] or [extended www autolink],
// or [extended protocol autolink] from s[i:] if one exists,
// using vd as its valid domain checker.
// It returns the link, the text following the auto-link, and whether a link was found at all.
//
// [extended URL autolink]: https://github.github.com/gfm/#extended-url-autolink
// [extended www autolink]: https://github.github.com/gfm/#extended-www-autolink
// [extended protocol autolink]: https://github.github.com/gfm/#extended-email-autolink
func parseAutoURL(p *parser, s string, i int, vd *validDomainChecker) (link *Link, after string, found bool) {
	if s == "" {
		// unreachable unless called wrong
		return
	}
	switch s[i] {
	case 'h':
		var n int
		if strings.HasPrefix(s[i:], "https://") {
			n = len("https://")
		} else if strings.HasPrefix(s[i:], "http://") {
			n = len("http://")
		} else {
			return
		}
		return parseAutoHTTP(p, s[i:i+n], s, i, i+n, i+n+1, vd)
	case 'w':
		if !strings.HasPrefix(s[i:], "www.") {
			return
		}
		// GitHub Flavored Markdown says to use http://,
		// but it's not 1985 anymore. We live in the https:// future
		// (unless the parser is explicitly configured otherwise).
		// People who really care in their docs can write http:// themselves.
		scheme := "https://"
		if p.AutoLinkAssumeHTTP {
			scheme = "http://"
		}
		return parseAutoHTTP(p, scheme, s, i, i, i+4, vd)
	case 'm':
		if !strings.HasPrefix(s[i:], "mailto:") {
			return
		}
		return parseAutoMailto(p, s, i)
	case 'x':
		if !strings.HasPrefix(s[i:], "xmpp:") {
			return
		}
		return parseAutoXmpp(p, s, i)
	}
	// unreachable unless called wrong
	return
}

// parseAutoHTTP parses a URL link, returning the link,
// the text following the link, and whether a link was found at all.
//
// The text of the link starts at s[textstart:].
// The actual link URL starts at s[start:].
// The link must use at least s[start:min] or it is not a valid link.
// vd is the domain checker to use.
func parseAutoHTTP(p *parser, scheme, s string, textstart, start, min int, vd *validDomainChecker) (link *Link, after string, found bool) {
	n, ok := vd.parseValidDomain(start)
	if !ok {
		return
	}
	i := start + n
	domEnd := i

	// “After a valid domain, zero or more non-space non-< characters may follow.”
	paren := 0
	for i < len(s) {
		r, n := utf8.DecodeRuneInString(s[i:])
		if isUnicodeSpace(r) || r == '<' {
			break
		}
		if r == '(' {
			paren++
		}
		if r == ')' {
			paren--
		}
		i += n
	}

	// https://github.github.com/gfm/#extended-autolink-path-validation
Trim:
	for i > 0 {
		switch s[i-1] {
		case '?', '!', '.', ',', ':', '@', '_', '~':
			// Trim certain trailing punctuation.
			i--
			continue Trim

		case ')':
			// Trim trailing unmatched (by count only) parens.
			if paren < 0 {
				for s[i-1] == ')' && paren < 0 {
					paren++
					i--
				}
				continue Trim
			}

		case ';':
			// Trim entity reference.
			// After doing the work of the scan, we either cut that part off the string
			// or we stop the trimming entirely, so there's no chance of repeating
			// the scan on a future iteration and going accidentally quadratic.
			// Even though the Markdown spec already requires having a complete
			// list of all the HTML entities, the GitHub definition here just requires
			// "looks like" an entity, meaning its an ampersand, letters/digits, and semicolon.
			for j := i - 2; j > start; j-- {
				if j < i-2 && s[j] == '&' {
					i = j
					continue Trim
				}
				if !isLetterDigit(s[j]) {
					i--
					break Trim
				}
			}
			// unreachable since there is a dot in the domain,
			// which will hit !isLetterDigit above.
		}
		break Trim
	}

	// According to the literal text of the GitHub Flavored Markdown spec
	// and the actual behavior on GitHub,
	// www.example.com$foo turns into <a href="https://www.example.com$foo">,
	// but that makes the character restrictions in the valid-domain check
	// almost meaningless. So we insist that when all is said and done,
	// if the domain is followed by anything, that thing must be a slash,
	// even though GitHub is not that picky.
	// People might complain about www.example.com:1234 not working,
	// but if you want to get fancy with that kind of thing, just write http:// in front.
	if textstart == start && i > domEnd && s[domEnd] != '/' {
		i = domEnd
	}

	if i < min {
		return
	}

	link = &Link{
		Inner: []Inline{&Plain{Text: s[textstart:i]}},
		URL:   scheme + s[start:i],
	}
	return link, s[i:], true
}

// parseAutoEmail parses an [extended email autolink] with its @ sign at s[i].
// The parser has checked that s[i] == '@'.
// parseAutoEmail returns the text of s before the link, the link, the text after the link,
// and whether a link was found at all.
//
// [extended email autolink]: https://github.github.com/gfm/#extended-email-autolink
func parseAutoEmail(p *parser, s string, i int) (before string, link *Link, after string, ok bool) {
	if s[i] != '@' {
		// unreachable unless called wrong
		return
	}

	// “One ore more characters which are alphanumeric, or ., -, _, or +.”
	j := i
	for j > 0 && (isLDH(s[j-1]) || s[j-1] == '_' || s[j-1] == '+' || s[j-1] == '.') {
		j--
	}
	if i-j < 1 {
		return
	}

	// “One or more characters which are alphanumeric, or - or _, separated by periods (.).
	// There must be at least one period. The last character must not be one of - or _.”
	dots := 0
	k := i + 1
	for k < len(s) && (isLDH(s[k]) || s[k] == '_' || s[k] == '.') {
		if s[k] == '.' {
			if s[k-1] == '.' {
				// Empirically, .. stops the scan but foo@.bar is fine.
				break
			}
			dots++
		}
		k++
	}

	// “., -, and _ can occur on both sides of the @, but only . may occur at the end
	// of the email address, in which case it will not be considered part of the address”
	if s[k-1] == '.' {
		dots--
		k--
	}
	if s[k-1] == '-' || s[k-1] == '_' {
		return
	}
	if k-(i+1)-dots < 2 || dots < 1 {
		return
	}

	link = &Link{
		Inner: []Inline{&Plain{Text: s[j:k]}},
		URL:   "mailto:" + s[j:k],
	}
	return s[:j], link, s[k:], true
}

// parseAutoMailto parses a mailto: [extended protocol link] from s[i:].
// The parser has checked that s[i:] begins with "mailto:".
// parseAutoMailto returns the link, the text after the link, and whether a link was found at all.
//
// [extended protocol link]: https://github.github.com/gfm/#extended-protocol-autolink
func parseAutoMailto(p *parser, s string, i int) (link *Link, after string, ok bool) {
	j := i + len("mailto:")
	for j < len(s) && (isLDH(s[j]) || s[j] == '_' || s[j] == '+' || s[j] == '.') {
		j++
	}
	if j >= len(s) || s[j] != '@' {
		return
	}
	before, link, after, ok := parseAutoEmail(p, s[i:], j-i)
	if before != "mailto:" || !ok {
		return nil, "", false
	}
	link.Inner[0] = &Plain{Text: s[i : len(s)-len(after)]}
	return link, after, true
}

// parseAutoXmpp parses an xmpp: [extended protocol link] from s[i:].
// The parser has checked that s[i:] begins with "xmpp:".
// parseAutoXmpp returns the link, the text after the link, and whether a link was found at all.
//
// [extended protocol link]: https://github.github.com/gfm/#extended-protocol-autolink
func parseAutoXmpp(p *parser, s string, i int) (link *Link, after string, ok bool) {
	j := i + len("xmpp:")
	for j < len(s) && (isLDH(s[j]) || s[j] == '_' || s[j] == '+' || s[j] == '.') {
		j++
	}
	if j >= len(s) || s[j] != '@' {
		return
	}
	before, link, after, ok := parseAutoEmail(p, s[i:], j-i)
	if before != "xmpp:" || !ok {
		return nil, "", false
	}
	if after != "" && after[0] == '/' {
		k := 1
		for k < len(after) && (isLetterDigit(after[k]) || after[k] == '@' || after[k] == '.') {
			k++
		}
		after = after[k:]
	}
	url := s[i : len(s)-len(after)]
	link.Inner[0] = &Plain{Text: url}
	link.URL = url
	return link, after, true
}

// A validDomainChecker implements the operation of parsing a valid domain
// starting at a specific offset in a string, but it amortizes analysis of the string
// across multiple calls to avoid quadratic behavior when the checker is invoked
// at every offset (or many offsets) in the string.
type validDomainChecker struct {
	s   string
	cut int // before this index, no valid domains
}

// removePrefix removes the first n bytes from the target string s,
// so that future calls are valid for s[n:], not s.
func (v *validDomainChecker) removePrefix(n int) {
	v.s = v.s[n:]
	v.cut -= n
}

// parseValidDomain parses a [valid domain].
//
// If s[start:] starts with a valid domain, parseValidDomain returns
// the length of that domain and true. If s[start:] does not start with
// a valid domain, parseValidDomain returns n, false,
// where n is the length of a prefix guaranteed not to be acceptable
// to any future call to parseValidDomain.
//
// “A valid domain consists of segments of alphanumeric characters,
// underscores (_) and hyphens (-) separated by periods (.).
// There must be at least one period, and no underscores may be
// present in the last two segments of the domain.”
//
// The spec does not spell out whether segments can be empty.
// Empirically, in GitHub's implementation they can.
//
// [valid domain]: https://github.github.com/gfm/#valid-domain
func (v *validDomainChecker) parseValidDomain(start int) (n int, found bool) {
	if start < v.cut {
		// A previous call established there are no valid domains before v.cut.
		return 0, false
	}
	i := start
	dots := 0
	for ; i < len(v.s); i++ {
		c := v.s[i]
		if c == '_' {
			dots = -2
			continue
		}
		if c == '.' {
			dots++
			continue
		}
		if !isLDH(c) {
			break
		}
	}
	if dots >= 0 && i > start {
		return i - start, true
	}
	v.cut = i // there are no valid domains before i
	return 0, false
}
