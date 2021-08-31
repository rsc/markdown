
package markdown

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/cases"
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
func xmain() {
	for _, x := range os.Args[1:] {
		list := inline(x)
		js, err := json.MarshalIndent(list, "", "\t")
		if err != nil {
			log.Fatal(err)
		}
		os.Stdout.Write(append(js, '\n'))
	}
}

type Inline interface {
	Inline()
	htmlTo(*bytes.Buffer)
	textTo(*bytes.Buffer)
}

type Plain struct {
	Text string
}

var htmlEscaper = strings.NewReplacer(
	"\"", "&quot;",
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

var htmlQuoteEscaper = strings.NewReplacer(
	"\"", "&quot;",
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
)

var htmlLinkEscaper = strings.NewReplacer(
	"\"", "%22",
	"&", "&amp;",
	"<", "&lt;",
	">", "&gt;",
	"\\", "%5C",
	" ", "%20",
	"`", "%60",
	"[", "%5B",
	"]", "%5D",
	"\x80", "%80",
	"\x81", "%81",
	"\x82", "%82",
	"\x83", "%83",
	"\x84", "%84",
	"\x85", "%85",
	"\x86", "%86",
	"\x87", "%87",
	"\x88", "%88",
	"\x89", "%89",
	"\x8A", "%8A",
	"\x8B", "%8B",
	"\x8C", "%8C",
	"\x8D", "%8D",
	"\x8E", "%8E",
	"\x8F", "%8F",
	"\x90", "%90",
	"\x91", "%91",
	"\x92", "%92",
	"\x93", "%93",
	"\x94", "%94",
	"\x95", "%95",
	"\x96", "%96",
	"\x97", "%97",
	"\x98", "%98",
	"\x99", "%99",
	"\x9A", "%9A",
	"\x9B", "%9B",
	"\x9C", "%9C",
	"\x9D", "%9D",
	"\x9E", "%9E",
	"\x9F", "%9F",
	"\xA0", "%A0",
	"\xA1", "%A1",
	"\xA2", "%A2",
	"\xA3", "%A3",
	"\xA4", "%A4",
	"\xA5", "%A5",
	"\xA6", "%A6",
	"\xA7", "%A7",
	"\xA8", "%A8",
	"\xA9", "%A9",
	"\xAA", "%AA",
	"\xAB", "%AB",
	"\xAC", "%AC",
	"\xAD", "%AD",
	"\xAE", "%AE",
	"\xAF", "%AF",
	"\xB0", "%B0",
	"\xB1", "%B1",
	"\xB2", "%B2",
	"\xB3", "%B3",
	"\xB4", "%B4",
	"\xB5", "%B5",
	"\xB6", "%B6",
	"\xB7", "%B7",
	"\xB8", "%B8",
	"\xB9", "%B9",
	"\xBA", "%BA",
	"\xBB", "%BB",
	"\xBC", "%BC",
	"\xBD", "%BD",
	"\xBE", "%BE",
	"\xBF", "%BF",
	"\xC0", "%C0",
	"\xC1", "%C1",
	"\xC2", "%C2",
	"\xC3", "%C3",
	"\xC4", "%C4",
	"\xC5", "%C5",
	"\xC6", "%C6",
	"\xC7", "%C7",
	"\xC8", "%C8",
	"\xC9", "%C9",
	"\xCA", "%CA",
	"\xCB", "%CB",
	"\xCC", "%CC",
	"\xCD", "%CD",
	"\xCE", "%CE",
	"\xCF", "%CF",
	"\xD0", "%D0",
	"\xD1", "%D1",
	"\xD2", "%D2",
	"\xD3", "%D3",
	"\xD4", "%D4",
	"\xD5", "%D5",
	"\xD6", "%D6",
	"\xD7", "%D7",
	"\xD8", "%D8",
	"\xD9", "%D9",
	"\xDA", "%DA",
	"\xDB", "%DB",
	"\xDC", "%DC",
	"\xDD", "%DD",
	"\xDE", "%DE",
	"\xDF", "%DF",
	"\xE0", "%E0",
	"\xE1", "%E1",
	"\xE2", "%E2",
	"\xE3", "%E3",
	"\xE4", "%E4",
	"\xE5", "%E5",
	"\xE6", "%E6",
	"\xE7", "%E7",
	"\xE8", "%E8",
	"\xE9", "%E9",
	"\xEA", "%EA",
	"\xEB", "%EB",
	"\xEC", "%EC",
	"\xED", "%ED",
	"\xEE", "%EE",
	"\xEF", "%EF",
	"\xF0", "%F0",
	"\xF1", "%F1",
	"\xF2", "%F2",
	"\xF3", "%F3",
	"\xF4", "%F4",
	"\xF5", "%F5",
	"\xF6", "%F6",
	"\xF7", "%F7",
	"\xF8", "%F8",
	"\xF9", "%F9",
	"\xFA", "%FA",
	"\xFB", "%FB",
	"\xFC", "%FC",
	"\xFD", "%FD",
	"\xFE", "%FE",
	"\xFF", "%FF",
)

func (*Plain) Inline() {}

func (x *Plain) htmlTo(buf *bytes.Buffer) {
	htmlEscaper.WriteString(buf, x.Text)
}

func (x *Plain) textTo(buf *bytes.Buffer) {
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

type Code struct {
	Text string
}

func (*Code) Inline() {}

func (x *Code) htmlTo(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<code>%s</code>", htmlEscaper.Replace(x.Text))
}

func (x *Code) textTo(buf *bytes.Buffer) {
	htmlEscaper.WriteString(buf, x.Text)
}

type Strong struct {
	Inner []Inline
}

func (x *Strong) Inline() {
}

func (x *Strong) htmlTo(buf *bytes.Buffer) {
	buf.WriteString("<strong>")
	for _, c := range x.Inner {
		c.htmlTo(buf)
	}
	buf.WriteString("</strong>")
}

func (x *Strong) textTo(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.textTo(buf)
	}
}

type Emph struct {
	Inner []Inline
}

func (*Emph) Inline() {}

func (x *Emph) htmlTo(buf *bytes.Buffer) {
	buf.WriteString("<em>")
	for _, c := range x.Inner {
		c.htmlTo(buf)
	}
	buf.WriteString("</em>")
}

func (x *Emph) textTo(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.textTo(buf)
	}
}

type AutoLink struct {
	Text string
	URL  string
}

func (*AutoLink) Inline() {}

func (x *AutoLink) htmlTo(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<a href=\"%s\">%s</a>", htmlLinkEscaper.Replace(x.URL), htmlEscaper.Replace(x.Text))
}

func (x *AutoLink) textTo(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "%s", htmlEscaper.Replace(x.Text))
}

type Link struct {
	Inner []Inline
	URL   string
	Title string
}

func (*Link) Inline() {}

func (x *Link) htmlTo(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<a href=\"%s\"", htmlLinkEscaper.Replace(x.URL))
	if x.Title != "" {
		fmt.Fprintf(buf, " title=\"%s\"", htmlQuoteEscaper.Replace(x.Title))
	}
	buf.WriteString(">")
	for _, c := range x.Inner {
		c.htmlTo(buf)
	}
	buf.WriteString("</a>")
}

func (x *Link) textTo(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.textTo(buf)
	}
}

type Image struct {
	Inner []Inline
	URL   string
	Title string
}

func (*Image) Inline() {}

func (x *Image) htmlTo(buf *bytes.Buffer) {
	fmt.Fprintf(buf, "<img src=\"%s\"", htmlLinkEscaper.Replace(x.URL))
	fmt.Fprintf(buf, " alt=\"")
	for _, c := range x.Inner {
		c.textTo(buf)
	}
	fmt.Fprintf(buf, "\"")
	if x.Title != "" {
		fmt.Fprintf(buf, " title=\"%s\"", htmlQuoteEscaper.Replace(x.Title))
	}
	buf.WriteString(" />")
}

func (x *Image) textTo(buf *bytes.Buffer) {
	for _, c := range x.Inner {
		c.textTo(buf)
	}
}

type HTMLTag struct {
	Text string
}

func (*HTMLTag) Inline() {}

func (x *HTMLTag) htmlTo(buf *bytes.Buffer) {
	buf.WriteString(x.Text)
}

func (x *HTMLTag) textTo(buf *bytes.Buffer) {}

type HardBreak struct{}

func (*HardBreak) Inline() {}

func (x *HardBreak) htmlTo(buf *bytes.Buffer) {
	buf.WriteString("<br />\n")
}

func (x *HardBreak) textTo(buf *bytes.Buffer) {}

type SoftBreak struct{}

func (*SoftBreak) Inline() {}

func (x *SoftBreak) htmlTo(buf *bytes.Buffer) {
	buf.WriteString("\n")
}

func (x *SoftBreak) textTo(buf *bytes.Buffer) {}

type inlineParser struct {
	s       string
	emitted int // s[:emitted] has been emitted into list
	list    []Inline
	links   map[string]*Link
}

func (p *inlineParser) emit(i int) {
	if p.emitted < i {
		p.list = append(p.list, &Plain{p.s[p.emitted:i]})
		p.emitted = i
	}
}

func (p *inlineParser) skip(i int) {
	p.emitted = i
}

func inline(s string) []Inline {
	s = strings.Trim(s, " \t")
	// Scan text looking for inlines.
	// Leaf inlines are converted immediately.
	// Non-leaf inlines have potential starts pushed on a stack while we await completion.
	// Links take priority over other emphasis, so the emphasis must be delayed.
	var p inlineParser
	p.links = links
	p.s = s
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
		case '\n':
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

/*
Now, look back in the stack (staying above stack_bottom and the openers_bottom for this delimiter type) for the first matching potential opener (“matching” means same delimiter).

If one is found:

Figure out whether we have emphasis or strong emphasis: if both closer and opener spans have length >= 2, we have strong, otherwise regular.

Insert an emph or strong emph node accordingly, after the text node corresponding to the opener.

Remove any delimiters between the opener and closer from the delimiter stack.

Remove 1 (for regular emph) or 2 (for strong emph) delimiters from the opening and closing text nodes. If they become empty as a result, remove them and remove the corresponding element of the delimiter stack. If the closing node is removed, reset current_position to the next element in the stack.

If none is found:

Set openers_bottom to the element before current_position. (We know that there are no openers for this kind of closer up to and including this point, so this puts a lower bound on future searches.)

If the closer at current_position is not a potential opener, remove it from the delimiter stack (since we know it can’t be a closer either).

Advance current_position to the next element in the stack.

After we’re done, we remove all delimiters above stack_bottom from the delimiter stack.

---

Emphasis begins with a delimiter that can open emphasis and ends with a delimiter that can close emphasis, and that uses the same character (_ or *) as the opening delimiter. The opening and closing delimiters must belong to separate delimiter runs. If one of the delimiters can both open and close emphasis, then the sum of the lengths of the delimiter runs containing the opening and closing delimiters must not be a multiple of 3 unless both lengths are multiples of 3.
*/

func (p *inlineParser) emph(dst, src []Inline) []Inline {
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
		fmt.Printf("#%d %T %+v\n", i, src[i], src[i])
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
					fmt.Printf("\tmatch %T %+v\n", start, start)
					if (p.canOpen && p.canClose || start.canOpen && start.canClose) && (p.n+start.n)%3 == 0 && (p.n%3 != 0 || start.n%3 != 0) {
						println("badsize")
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
					x := &Emph{Inner: append([]Inline(nil), dst[start.i+1:]...)}
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
				fmt.Printf("\tsaveopen\n")
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
		if c == '\n' {
			end := i + 2
			for end < len(s) && (s[end] == ' ' || s[end] == '\t') {
				end++
			}
			return &HardBreak{}, i, end, true
		}
	}
	return nil, 0, 0, false
}

func parseHTMLEntity(s string, i int) (Inline, int, int, bool) {
	start := i
	if i+1 < len(s) && s[i+1] == '#' {
		i += 2
		var r, end int
		if i < len(s) && (s[i] == 'x' || s[i] == 'X') {
			// hex
			i++
			j := i
			for j < len(s) && isHexDigit(s[j]) {
				j++
			}
			if j-i < 1 || j-i > 6 || j >= len(s) || s[j] != ';' {
				return nil, 0, 0, false
			}
			r64, _ := strconv.ParseInt(s[i:j], 16, 0)
			r = int(r64)
			end = j + 1
		} else {
			// decimal
			j := i
			for j < len(s) && isDigit(s[j]) {
				j++
			}
			if j-i < 1 || j-i > 7 || j >= len(s) || s[j] != ';' {
				return nil, 0, 0, false
			}
			r, _ = strconv.Atoi(s[i:j])
			end = j + 1
		}
		if r > unicode.MaxRune || r == 0 {
			r = unicode.ReplacementChar
		}
		return &Plain{string(rune(r))}, start, end, true
	}

	// Max name in list is 32 bytes. Try for 64 for good measure.
	for j := i + 1; j < len(s) && j-i < 64; j++ {
		if s[j] == '&' { // Stop possible quadratic search on &&&&&&&.
			break
		}
		if s[j] == ';' {
			if r, ok := htmlEntity[s[i:j+1]]; ok {
				return &Plain{r}, start, j + 1, true
			}
			break
		}
	}

	return nil, 0, 0, false
}

func parseBreak(s string, i int) (Inline, int, int, bool) {
	start := i
	for start > 0 && (s[start-1] == ' ' || s[start-1] == '\t') {
		start--
	}
	end := i + 1
	for end < len(s) && (s[end] == ' ' || s[end] == '\t') {
		end++
	}
	// TODO: Do tabs count? That would be a mess.
	if i >= 2 && s[i-1] == ' ' && s[i-2] == ' ' {
		return &HardBreak{}, start, end, true
	}
	return &SoftBreak{}, start, end, true
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

			return &Code{text}, start, end, true
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

func isLetter(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

func isLetterDigit(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
}

func isHexDigit(c byte) bool {
	return 'A' <= c && c <= 'F' || 'a' <= c && c <= 'f' || '0' <= c && c <= '9'
}

func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

func isScheme(c byte) bool {
	return isLetterDigit(c) || c == '+' || c == '.' || c == '-'
}

func isURL(c byte) bool {
	return c > ' ' && c != '<' && c != '>'
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

func isLDH(c byte) bool {
	return isLetterDigit(c) || c == '-'
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

func parseHTMLTag(s string, i int) (Inline, int, bool) {
	// “An HTML tag consists of an open tag, a closing tag, an HTML comment,
	// a processing instruction, a declaration, or a CDATA section.”
	if i+3 < len(s) {
		switch s[i+1] {
		default:
			return parseHTMLOpenTag(s, i)
		case '/':
			return parseHTMLClosingTag(s, i)
		case '!':
			switch s[i+2] {
			case '-':
				return parseHTMLComment(s, i)
			case '[':
				return parseHTMLCDATA(s, i)
			default:
				return parseHTMLDecl(s, i)
			}
		case '?':
			return parseHTMLProcInst(s, i)
		}
	}
	return nil, 0, false
}

func parseHTMLOpenTag(s string, i int) (Inline, int, bool) {
	// “An open tag consists of a < character, a tag name, zero or more attributes,
	// optional spaces, tabs, and up to one line ending, an optional / character, and a > character.”
	if _, j, ok := parseTagName(s, i+1); ok {
		for {
			if j >= len(s) || s[j] != ' ' && s[j] != '\t' && s[j] != '\n' && s[j] != '/' && s[j] != '>' {
				return nil, 0, false
			}
			_, k, ok := parseAttr(s, j)
			if !ok {
				break
			}
			j = k
		}
		j = skipSpace(s, j)
		if j < len(s) && s[j] == '/' {
			j++
		}
		if j < len(s) && s[j] == '>' {
			return &HTMLTag{s[i : j+1]}, j + 1, true
		}
	}
	return nil, 0, false
}

func parseHTMLClosingTag(s string, i int) (Inline, int, bool) {
	// “A closing tag consists of the string </, a tag name,
	// optional spaces, tabs, and up to one line ending, and the character >.”
	if _, j, ok := parseTagName(s, i+2); ok {
		j = skipSpace(s, j)
		if j < len(s) && s[j] == '>' {
			return &HTMLTag{s[i : j+1]}, j + 1, true
		}
	}
	return nil, 0, false
}

func parseTagName(s string, i int) (string, int, bool) {
	// “A tag name consists of an ASCII letter followed by zero or more ASCII letters, digits, or hyphens (-).”
	if i+1 < len(s) && isLetter(s[i]) {
		j := i + 1
		for j < len(s) && isLDH(s[j]) {
			j++
		}
		return s[i:j], j, true
	}
	return "", 0, false
}

func parseAttr(s string, i int) (string, int, bool) {
	// “An attribute consists of spaces, tabs, and up to one line ending,
	// an attribute name, and an optional attribute value specification.”
	i = skipSpace(s, i)
	if _, j, ok := parseAttrName(s, i); ok {
		if _, k, ok := parseAttrValueSpec(s, j); ok {
			j = k
		}
		return s[i:j], j, true
	}
	return "", 0, false
}

func parseAttrName(s string, i int) (string, int, bool) {
	// “An attribute name consists of an ASCII letter, _, or :,
	// followed by zero or more ASCII letters, digits, _, ., :, or -.”
	if i+1 < len(s) && (isLetter(s[i]) || s[i] == '_' || s[i] == ':') {
		j := i + 1
		for j < len(s) && (isLDH(s[j]) || s[j] == '_' || s[j] == '.' || s[j] == ':') {
			j++
		}
		return s[i:j], j, true
	}
	return "", 0, false
}

func parseAttrValueSpec(s string, i int) (string, int, bool) {
	// “An attribute value specification consists of
	// optional spaces, tabs, and up to one line ending,
	// a = character,
	// optional spaces, tabs, and up to one line ending,
	// and an attribute value.”
	i = skipSpace(s, i)
	if i+1 < len(s) && s[i] == '=' {
		i = skipSpace(s, i+1)
		if _, j, ok := parseAttrValue(s, i); ok {
			return s[i:j], j, true
		}
	}
	return "", 0, false
}

func parseAttrValue(s string, i int) (string, int, bool) {
	// “An attribute value consists of
	// an unquoted attribute value,
	// a single-quoted attribute value,
	// or a double-quoted attribute value.”
	// TODO: No escaping???
	if i < len(s) && (s[i] == '\'' || s[i] == '"') {
		// “A single-quoted attribute value consists of ',
		// zero or more characters not including ', and a final '.”
		// “A double-quoted attribute value consists of ",
		// zero or more characters not including ", and a final ".”
		if j := strings.IndexByte(s[i+1:], s[i]); j >= 0 {
			end := i + 1 + j + 1
			return s[i:end], end, true
		}
	}

	// “An unquoted attribute value is a nonempty string of characters
	// not including spaces, tabs, line endings, ", ', =, <, >, or `.”
	j := i
	for j < len(s) && strings.IndexByte(" \t\n\"'=<>`", s[j]) < 0 {
		j++
	}
	if j > i {
		return s[i:j], j, true
	}
	return "", 0, false
}

func parseHTMLComment(s string, i int) (Inline, int, bool) {
	// “An HTML comment consists of <!-- + text + -->,
	// where text does not start with > or ->,
	// does not end with -, and does not contain --.”
	if !strings.HasPrefix(s[i:], "<!-->") &&
		!strings.HasPrefix(s[i:], "<!--->") {
		if x, end, ok := parseHTMLMarker(s, i, "<!--", "-->"); ok {
			if t := x.(*HTMLTag).Text; !strings.Contains(t[i+len("<!--"):len(t)-len("->")], "--") {
				return x, end, ok
			}
		}
	}
	return nil, 0, false
}

func parseHTMLCDATA(s string, i int) (Inline, int, bool) {
	// “A CDATA section consists of the string <![CDATA[,
	// a string of characters not including the string ]]>, and the string ]]>.”
	return parseHTMLMarker(s, i, "<![CDATA[", "]]>")
}

func parseHTMLDecl(s string, i int) (Inline, int, bool) {
	// “A declaration consists of the string <!, an ASCII letter,
	// zero or more characters not including the character >, and the character >.”
	if i+2 < len(s) && isLetter(s[i+2]) {
		return parseHTMLMarker(s, i, "<", ">")
	}
	return nil, 0, false
}

func parseHTMLProcInst(s string, i int) (Inline, int, bool) {
	// “A processing instruction consists of the string <?,
	// a string of characters not including the string ?>, and the string ?>.”
	return parseHTMLMarker(s, i, "<?", "?>")
}

func parseHTMLMarker(s string, i int, prefix, suffix string) (Inline, int, bool) {
	if strings.HasPrefix(s[i:], prefix) {
		if j := strings.Index(s[i+len(prefix):], suffix); j >= 0 {
			end := i + len(prefix) + j + len(suffix)
			return &HTMLTag{s[i:end]}, end, true
		}
	}
	return nil, 0, false
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

func (p *inlineParser) parseLinkClose(s string, i int, open *openPlain) (*Link, int, bool) {
	if i+1 < len(s) {
		switch s[i+1] {
		case '(':
			// Inline link - [Text](Dest Title), with Title omitted or both Dest and Title omitted.
			i := skipSpace(s, i+2)
			var dest, title string
			if i < len(s) && s[i] != ')' {
				var ok bool
				dest, i, ok = parseLinkDest(s, i)
				if !ok {
					break
				}
				i = skipSpace(s, i)
				if i < len(s) && s[i] != ')' {
					title, i, ok = parseLinkTitle(s, i)
					if !ok {
						break
					}
					i = skipSpace(s, i)
				}
			}
			if i < len(s) && s[i] == ')' {
				return &Link{URL: dest, Title: title}, i + 1, true
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

func parseLinkTitle(s string, i int) (string, int, bool) {
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
				return mdUnescaper.Replace(title), j + 1, true
			}
			if s[j] == '(' && want == ')' {
				break
			}
			if s[j] == '\\' && j+1 < len(s) {
				j++
			}
		}
	}
	return "", 0, false
}

func parseLinkLabel(s string, i int) (string, int, bool) {
	// “A link label begins with a left bracket ([) and ends with
	// the first right bracket (]) that is not backslash-escaped.
	// Between these brackets there must be at least one character
	// that is not a space, tab, or line ending.
	// Unescaped square bracket characters are not allowed
	// inside the opening and closing square brackets of link labels.
	// A link label can have at most 999 characters inside the square brackets.”
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
