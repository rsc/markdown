// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
	"unicode/utf8"
)

// An Inline is an inline Markdown element, one of
// [Plain], [Escaped], [Code], [Strong], [Emph], [Del],
// [Link], [AutoLink], [Image],
// [SoftBreak], [HardBreak],
// [HTMLTag],
// [Emoji], and [Task].
type Inline interface {
	Inline()

	printHTML(*printer)
	printText(*printer)
	printMarkdown(*printer)
}

// An Inlines is an [Inline] that represents a concatenation of Inlines.
type Inlines []Inline

func (Inlines) Inline() {}

func (x Inlines) printText(p *printer) {
	for _, inl := range x {
		inl.printText(p)
	}
}

func (x Inlines) printHTML(p *printer) {
	for _, inl := range x {
		inl.printHTML(p)
	}
}

func (x Inlines) printMarkdown(p *printer) {
	for _, inl := range x {
		inl.printMarkdown(p)
	}
}

// A Plain is an [Inline] that represents [plain textual content].
//
// [textual content]: https://spec.commonmark.org/0.31.2/#textual-content
type Plain struct {
	Text string
}

func (*Plain) Inline() {}

func (x *Plain) printText(p *printer) { p.text(x.Text) }
func (x *Plain) printHTML(p *printer) { p.text(x.Text) }

func (x *Plain) printMarkdown(p *printer) {
	// TODO: This is wrong if Plain contains characters that should be escaped.
	// Today that doesn't happen for our own parses, but constructed syntax trees
	// might contain them.
	// Deciding exactly what to escape is (or probably should be) somewhat context dependent.
	for i, line := range strings.Split(x.Text, "\n") {
		if i > 0 {
			p.nl()
		}
		p.WriteString(line)
		p.noTrim()
	}
}

// An Escaped is an [Inline] that represents a [backslash escaped symbol].
//
// [backslash escaped symbol]: https://spec.commonmark.org/0.31.2/#backslash-escapes
type Escaped struct {
	Plain // single character text (omitting the escaping backslash)
}

func (x *Escaped) printMarkdown(p *printer) {
	p.md(`\`)
	p.md(x.Text)
}

// A Code is an [Inline] that represents a [code span].
//
// [code span]: https://spec.commonmark.org/0.31.2/#code-spans
type Code struct {
	Text string
}

func (*Code) Inline() {}

func (x *Code) printText(p *printer) { p.text(x.Text) }

func (x *Code) printHTML(p *printer) {
	p.html(`<code>`)
	p.text(x.Text)
	p.html(`</code>`)
}

var ticks = "````````````````````````````````````````````````````````````````" // 64 ticks

func (x *Code) printMarkdown(p *printer) {
	// Use the fewest backticks we can, and add spaces as needed.
	n := maxRun(x.Text, '`') + 1
	printTicks(p, n)

	// Note: len(x.Text)==0 is not possible to express in Markdown,
	// but if someone makes a buggy Code, we print it as ` ` (a code-formatted space),
	// since the only other choice would be to not print any code text at all, which is worse.
	space := len(x.Text) == 0 || x.Text[0] == '`' || x.Text[len(x.Text)-1] == '`'
	if space {
		p.WriteByte(' ')
	}
	p.WriteString(x.Text)
	if space {
		p.WriteByte(' ')
	}

	printTicks(p, n)
}

// maxRun returns the length of the longest run of b bytes in s.
func maxRun(s string, b byte) int {
	m := 0
	n := 0
	for i := range len(s) {
		if s[i] == b {
			n++
			m = max(m, n)
		} else {
			n = 0
		}
	}
	return m
}

// printTicks prints n backticks to p.
func printTicks(p *printer, n int) {
	for n > len(ticks) {
		p.md(ticks)
		n -= len(ticks)
	}
	p.md(ticks[:n])
}

// A Strong is an [Inline] that represents [strong emphasis] (bold text).
//
// [strong emphasis]: https://spec.commonmark.org/0.31.2/#emphasis-and-strong-emphasis
type Strong struct {
	Marker string
	Inner  Inlines
}

func (*Strong) Inline() {}

func (x *Strong) printText(p *printer) { x.Inner.printText(p) }

func (x *Strong) printHTML(p *printer) {
	p.html("<strong>")
	x.Inner.printHTML(p)
	p.html("</strong>")
}

func (x *Strong) printMarkdown(p *printer) {
	p.md(x.Marker)
	x.Inner.printMarkdown(p)
	p.md(x.Marker)
}

// An Emph is an [Inline] representing [emphasis] (italic text).
//
// [emphasis]: https://spec.commonmark.org/0.31.2/#emphasis-and-strong-emphasis
type Emph struct {
	Marker string
	Inner  Inlines
}

func (*Emph) Inline() {}

func (x *Emph) printText(p *printer) { x.Inner.printText(p) }

func (x *Emph) printHTML(p *printer) {
	p.html("<em>")
	x.Inner.printHTML(p)
	p.html("</em>")
}

func (x *Emph) printMarkdown(p *printer) {
	p.md(x.Marker)
	x.Inner.printMarkdown(p)
	p.md(x.Marker)
}

// A Deleted is an [Inline] that represents [deleted (strikethrough) text],
// a GitHub-flavored Markdown extension.
//
// [deleted (strikethrough) text]: https://github.github.com/gfm/#strikethrough-extension-
type Del struct {
	Marker string
	Inner  Inlines
}

func (*Del) Inline() {}

func (x *Del) printText(p *printer) { x.Inner.printText(p) }

func (x *Del) printHTML(p *printer) {
	p.html("<del>")
	x.Inner.printHTML(p)
	p.html("</del>")
}

func (x *Del) printMarkdown(p *printer) {
	p.WriteString(x.Marker)
	x.Inner.printMarkdown(p)
	p.WriteString(x.Marker)
}

// An Emoji is an [Inline] that represents an emoji, like :smiley:,
// an apparently undocumented but widely used GitHub Markdown extension.
type Emoji struct {
	Name string // emoji :name:, including colons
	Text string // Unicode for emoji sequence
}

func (*Emoji) Inline() {}

func (x *Emoji) printText(p *printer)     { p.text(x.Text) }
func (x *Emoji) printHTML(p *printer)     { p.text(x.Text) }
func (x *Emoji) printMarkdown(p *printer) { p.text(x.Text) }

// Parsing Inlines
//
// The parser walks over the text looking for opening special characters such as * or [
// and pushes them onto a parse stack. This same scan also looks for and pushes
// special leaf inlines such as escaped symbols, HTML entities, code spans, and line breaks.
// Text between special cases is also pushed, as plain text (type Plain).
// When the parser sees a closing special character such as * or ], it tries to match
// that closing special against a corresponding opening special. If it can, the stack
// content between the opening and the closing becomes the inner inline of a new
// node (for example, an Emph or a Link), and that new node is pushed on the stack
// in place of the opening and the inner content.
//
// The matching proceeds in two phases: brackets first, then emphasis
// (* and _, as well as the extensions ~ ' and "). This is because links and images
// take priority over emphasis: "this *nice [link*](/emph)" contains a link
// but no emphasis: the * outside the link brackets cannot match the * inside.
//
// Another subtlety is that link text can contain images:
//
//	[link ![foo](img.jpg)](/) ⇒
//	<a href="/">link <img src="img.jpg" alt="foo" /></a>
//
// But link text cannot contain links:
//
//	[link [foo](img.jpg)](/)  ⇒
//	[link <a href="img.jpg">foo</a>](/)
//
// (note the missing !).
//
// Image text can contain links, images, and emphasized text,
// but all that turns into plain text when formatted in the alt tag:
//
//	![link ![foo](img.jpg) [bar](/)](/) ⇒
//	<img src="/" alt="link foo bar" />
//
// The prohibition on links containing links applies even to
// links containing images containing links:
//
//	[outer ![link ![foo](img.jpg) [bar](/)](/)](/out) ⇒
//	[outer <img src="/" alt="link foo bar" />](/out)
//
//	[outer ![link ![foo](img.jpg) ![bar](/)](/)](/out) ⇒
//	<a href="/out">outer <img src="/" alt="link foo bar" /></a>
//
// Yet another subtlety is that straightforward implementation of parts of this
// would be accidentally quadratic (see tumblr.com/accidentallyquadratic),
// leading to performance problems and potential denial of service attacks.
// We must take care to avoid quadratic behavior.

// An inlineParser parses s[start:] into an Inline, returning the Inline
// and the string index where the inline ends
// (that is, the Inline represents s[start:end]).
// If it cannot parse s[start:], it returns ok=false.
// The caller has usually checked that s[start:] is likely to be appropriate
// for this parser.
type inlineParser func(p *parser, s string, start int) (x Inline, end int, ok bool)

// emit emits p.s[p.emitted:i] as plain text and then sets p.emitted = i.
// (p.emitted keeps track of the place in the current line has has been emitted
// onto the stack in some form already.)
func (p *parser) emit(i int) {
	if p.emitted < i {
		p.list = append(p.list, &Plain{p.s[p.emitted:i]})
		p.emitted = i
	}
}

// skip sets p.emitted = i.
func (p *parser) skip(i int) {
	p.emitted = i
}

// An openPlain is an [Inline] that represents an opening marker
// [ or ![ that has not yet been matched to a closing marker.
// It only exists on the parse stack, not in the final returned Markdown.
type openPlain struct {
	Plain
	i int // position in input where bracket is
}

// An emphPlain is an [Inline] that represents an opening emphasis marker
// such as * or _ that has not yet been matched to a closing marker.
// It only exists on the parse stack, not in the final returned Markdown.
type emphPlain struct {
	Plain
	canOpen  bool // marker can open emphasis
	canClose bool // marker can close emphasis
	i        int  // position in output where emph is
	n        int  // length of original span
}

// inline parses s into an Inlines.
//
// In terms of the “Parsing Inlines” comment above, inline handles the
// scanning of the string into a parse stack and the construction of links
// and images; the emphasis processing is delegated to [parser.emph].
func (p *parser) inline(s string) Inlines {
	p.lineInfo = lineInfo{}
	s = trimSpaceTab(s)

	p.s = s
	p.list = nil
	p.emitted = 0

	// Scan text looking for inlines.
	// Leaf inlines are converted immediately.
	// Potential link and image openings are pushed onto a stack while we await completion.
	// Emphasis is applied by p.emph once we identify the link and image boundaries.

	var opens []int          // indexes of open ![ and [ openPlains in p.list
	var ignoreLinkBefore int // ignore link openings before this stack offset, to avoid links inside links
	backticksReset := false  // for lazy initialization of p.backticks

	for off := 0; off < len(s); {
		// Determine the parser based on leading character.
		var parser inlineParser
		switch s[off] {
		case '\\':
			parser = parseEscape
		case '`':
			if !backticksReset {
				p.backticks.reset()
				backticksReset = true
			}
			parser = p.backticks.parseCodeSpan
		case '<':
			parser = parseAutoLinkOrHTML
		case '[':
			parser = parseLinkOpen
		case '!':
			parser = parseImageOpen
		case '_', '*':
			parser = parseEmph
		case '.':
			if p.SmartDot {
				parser = parseDot
			}
		case '-':
			if p.SmartDash {
				parser = parseDash
			}
		case '"', '\'':
			if p.SmartQuote {
				parser = parseEmph
			}
		case '~':
			if p.Strikethrough {
				parser = parseEmph
			}
		case '\n': // TODO what about eof
			parser = parseBreak
		case '&':
			parser = parseHTMLEntity
		case ':':
			if p.Emoji {
				parser = parseEmoji
			}
		}

		// If there is a parser, run it.
		if parser != nil {
			if x, end, ok := parser(p, s, off); ok {
				// Emit plain text to list up through start.
				p.emit(off)

				// Add x to list, recording locations of openPlain entries.
				if _, ok := x.(*openPlain); ok {
					opens = append(opens, len(p.list))
				}
				p.list = append(p.list, x)

				// Skip over x's extent in future plain text emits.
				p.skip(end)
				off = end

				// Keep parsing.
				continue
			}
		}

		// If there's a closing bracket, match it to an opening bracket.
		if s[off] == ']' && len(opens) > 0 {
			// Pop most recent opening index from opens.
			oi := opens[len(opens)-1]
			opens = opens[:len(opens)-1]

			// Match to the openPlain in the list.
			// An image is valid anywhere; a link is only valid if it starts
			// after ignoreLinkBefore, to avoid links containing links.
			open := p.list[oi].(*openPlain)
			if open.i >= ignoreLinkBefore || open.Text[0] == '!' {
				if x, end, ok := parseLinkClose(p, s, off, open); ok {
					p.emit(off)
					x.Inner = p.emph(nil, p.list[oi+1:])
					if open.Text[0] == '!' {
						// parseLinkClose always returns a *Link.
						// By design, Link and Image are the same underlying struct,
						// so we can convert to *Image here.
						p.list[oi] = (*Image)(x)
					} else {
						p.list[oi] = x
					}
					p.list = p.list[:oi+1]
					p.skip(end)
					off = end
					if open.Text[0] == '[' {
						// No links around links.
						ignoreLinkBefore = open.i
					}

					// Goldmark and the Dingus re-escape invalid-looking percents as %25,
					// but the spec does not seem to require this behavior.
					url := x.URL
					for i := 0; i < len(url); i++ {
						if url[i] == '%' && (i+2 >= len(url) || !isHexDigit(url[i+1]) || !isHexDigit(url[i+2])) {
							p.corner = true
							break
						}
					}
					continue
				}
			}
		}

		// Unspecial character; advance to next character.
		off++
	}

	// Emit remainder of string.
	p.emit(len(s))

	// Apply emphasis to stack (topmost Inlines we will return).
	p.list = p.emph(p.list[:0], p.list)

	// Merge adjacent Plain elements in the list, so that for example
	// abc*def is Plain{abc*def} and not Plain{abc}Plain{*}Plain{def}.
	// (The * was tracked separately because it might have started emphasis.)
	p.list = p.mergePlain(p.list)

	// Apply GitHub autolinks to result, if extension is enabled.
	p.list = autoLinkText(p, p.list)

	return p.list
}

// emph applies emphasis in a run of inlines that has already had links and images converted.
// The links and images themselves contains inlines that have already had emph run.
// This function only has to process the inlines in src itself.
// It appends the new sequence of inlines to dst.
// dst and src may point at the same underlying array, provided &dst[0] == &src[0],
// in which case appending to dst will overwrite src, but that's okay because the
// number of elements in src only decreases or stays the same as it gets converted.
// emph may edit the values in src.
//
// This algorithm is fairly complicated and also fairly difficult to disentangle.
// TODO: Try harder.
func (ps *parser) emph(dst, src []Inline) []Inline {
	// For each emphasis character, we maintain a stack of the
	// possible openings we have seen, as *emphPlain nodes,
	// for matching against closings using the same character.
	// (The conversion to *emphPlain happened during p.inline,
	// when it called parseEmph.)
	const (
		stackStrike      = 0 // also 1
		stackSingleQuote = 2
		stackDoubleQuote = 3
		stackStar        = 4  // also 5..9
		stackUnder       = 10 // also 11..15
		stackTotal       = 16
	)
	var stack [stackTotal][]*emphPlain

Src:
	for i := 0; i < len(src); i++ {
		// Look for emphPlains; append the rest to dst.
		inl := src[i]
		p, ok := inl.(*emphPlain)
		if !ok {
			if open, ok := inl.(*openPlain); ok {
				// Convert unused link/image open marker (*openPlain) to plain text (*Plain).
				inl = &open.Plain
			}
			dst = append(dst, inl)
			continue
		}

		if p.canClose {
			// If this is a potential closing emphasis, try to match to earlier opening.
			// A closing ** might match against an earlier ** but also might match
			// against two separate *, as in "*hello *world**",
			// or might match against only one *, as in *hello world**,
			// which ends in a literal *.
			// When a repeated character closes only a single character,
			// the handling of * (or _ or ~) removes one character
			// from p.Text and does goto PText to process p.Text again.
		PText:
			// Easy special cases.
			switch p.Text[0] {
			case '"':
				stk := stack[stackDoubleQuote]
				if len(stk) == 0 {
					goto EmitPlain
				}
				stk, start := stk[:len(stk)-1], stk[len(stk)-1]
				stack[stackDoubleQuote] = stk

				// Rewrite "hello" into “hello”.
				dst[start.i].(*emphPlain).Text = "“"
				p.Text = "”"
				dst = append(dst, &p.Plain)
				continue Src

			case '\'':
				stk := stack[stackSingleQuote]
				if len(stk) == 0 {
					goto EmitPlain
				}
				stk, start := stk[:len(stk)-1], stk[len(stk)-1]
				stack[stackSingleQuote] = stk

				// Rewrite 'hello' into ‘hello’.
				dst[start.i].(*emphPlain).Text = "‘"
				p.Text = "’"
				dst = append(dst, &p.Plain)
				continue Src
			}

			// General case: emphasis containing other inlines.
			var start *emphPlain
			switch p.Text[0] {
			case '~':
				si := stackStrike + len(p.Text) - 1
				stk := stack[si]
				if len(stk) == 0 {
					goto EmitPlain
				}
				start = stk[len(stk)-1]

			case '*', '_':
				// Complicated Markdown rule:
				// “If one of the delimiters can both open and close emphasis, then the sum of the lengths
				// of the delimiter runs containing the opening and closing delimiters must not
				// be a multiple of 3 unless both lengths are multiples of 3.”
				// (https://spec.commonmark.org/0.31.2/#emphasis-and-strong-emphasis, rule 9)
				allow := func(p, start *emphPlain) bool {
					return (!p.canOpen && !start.canClose) || // neither can do both
						(p.n+start.n)%3 != 0 || // total not a multiple of 3
						p.n%3 == 0 // both are multiples of 3 (checking one implies the other)
				}

				// Consider the six possible stacks (3 n%3 values × 2 canClose bool values)
				// and take the acceptable one that appears latest in dst.
				// We could have one stack for each of * and _ and then walk down it to
				// find an acceptable value, but if we do that, there is the possibility of
				// a malicious input causing us to walk arbitrarily far down the stack
				// only to find nothing, again and again, triggering quadratic behavior.
				si := stackStar
				if p.Text[0] == '_' {
					si = stackUnder
				}
				for i := si; i < si+6; i++ {
					if len(stack[i]) == 0 {
						continue
					}
					maybe := stack[i][len(stack[i])-1]
					if allow(p, maybe) && (start == nil || maybe.i > start.i) {
						start = maybe
					}
				}
				if start == nil {
					goto EmitPlain
				}
			}

			// Match open and close. If both sides have >= 2 delimiters,
			// we chop 2 off each; otherwise we chop 1.
			var d int
			if len(p.Text) >= 2 && len(start.Text) >= 2 {
				// strong
				d = 2
			} else {
				// emph
				d = 1
			}
			del := p.Text[0] == '~'

			// Create emphasis node containing stack between open and close.
			x := &Emph{Marker: p.Text[:d], Inner: append([]Inline(nil), ps.mergePlain(dst[start.i+1:])...)}

			// Remove used delimiters from start; if start is empty, remove it from dst.
			// Otherwise leave it at the top of dst (we will push x onto dst below).
			start.Text = start.Text[:len(start.Text)-d]
			if start.Text == "" {
				dst = dst[:start.i]
			} else {
				dst = dst[:start.i+1]
			}

			// Now that we've popped all the inner content from dst (and possibly start as well),
			// pop everything is gone from the stacks too.
			for i := range stack {
				if len(stack[i]) > 0 {
					stk := stack[i]
					for len(stk) > 0 && stk[len(stk)-1].i >= len(dst) {
						stk = stk[:len(stk)-1]
					}
					stack[i] = stk
				}
			}

			// Push x (of correct type) onto dst.
			// By design, Del, Strong, and Emph are all the same
			// underlying struct, so we create an Emph above and
			// convert it to the right type here.
			if del {
				dst = append(dst, (*Del)(x))
			} else if d == 2 {
				dst = append(dst, (*Strong)(x))
			} else {
				dst = append(dst, x)
			}

			// Remove used delimiters from p and go around again.
			p.Text = p.Text[d:]
			if p.Text == "" {
				continue Src
			}
			goto PText
		}

	EmitPlain:
		if p.canOpen {
			p.i = len(dst)
			dst = append(dst, p)
			si := -1
			switch p.Text[0] {
			case '~':
				si = stackStrike + len(p.Text) - 1
			case '\'':
				si = stackSingleQuote
			case '"':
				si = stackDoubleQuote
			case '*', '_':
				si = stackStar
				if p.Text[0] == '_' {
					si = stackUnder
				}
				if p.canClose {
					si += 3
				}
				si += p.n % 3
			}
			stk := &stack[si]
			*stk = append(*stk, p)
		} else {
			dst = append(dst, &p.Plain)
		}

		// Rewrite unmatched quotes to right quotes.
		// Do this after the p.canOpen switch above,
		// which looks for the original ASCII quotes.
		if p.Text == "'" {
			p.Text = "’"
		}
		if p.Text == "\"" {
			if p.canClose {
				p.Text = "”"
			} else {
				p.Text = "“"
			}
		}
	}

	return ps.mergePlain(dst)
}

// parseEscape is an [inlineParser] for an [Escaped] or [HardBreak].
func parseEscape(p *parser, s string, start int) (x Inline, end int, ok bool) {
	if start+1 < len(s) {
		c := s[start+1]
		end = start + 2
		if isPunct(c) {
			return &Escaped{Plain{s[start+1 : end]}}, end, true
		}
		if c == '\n' { // TODO what about eof
			if start > 0 && s[start-1] == '\\' {
				p.corner = true // goldmark mishandles \\\ newline
			}
			return &HardBreak{}, end, true
		}
	}
	return nil, 0, false
}

// parseAutoLinkOrHTML is an [inlineParser] for a Markdown autolink (not GitHub autolink)
// or an HTML tag. The caller has checked that s[start] == '<'.
func parseAutoLinkOrHTML(p *parser, s string, start int) (x Inline, end int, ok bool) {
	if x, end, ok = parseAutoLinkURI(s, start); ok {
		return
	}
	if x, end, ok = parseAutoLinkEmail(s, start); ok {
		return
	}
	if x, end, ok = parseHTMLTag(p, s, start); ok {
		return
	}
	return
}

// parseDot is an [inlineParser] for a “smart” ellipsis when the
// SmartDot extension is enabled. It rewrites "..." into "…".
func parseDot(p *parser, s string, i int) (x Inline, end int, ok bool) {
	if i+2 < len(s) && s[i+1] == '.' && s[i+2] == '.' {
		return &Plain{"…"}, i + 3, true
	}
	return
}

// parseDash is an [inlineParser] for a “smart” endash and emdash
// when the SmartDash extension is enabled.
// It rewrites -- into – and --- into —.
func parseDash(p *parser, s string, i int) (x Inline, end int, ok bool) {
	if i+1 >= len(s) || s[i+1] != '-' {
		return
	}
	n := 2
	for i+n < len(s) && s[i+n] == '-' {
		n++
	}

	// Obviously -- is – and --- is —,
	// but what about ----? -----? ------?
	// We blindly follow cmark-gfm's rules.
	em, en := 0, 0
	switch {
	case n%3 == 0:
		em = n / 3
	case n%2 == 0:
		en = n / 2
	case n%3 == 2:
		em = (n - 2) / 3
		en = 1
	case n%3 == 1:
		em = (n - 4) / 3
		en = 2
	}
	return &Plain{strings.Repeat("—", em) + strings.Repeat("–", en)}, i + n, true
}

// parseEmoji is an [inlineParser] for an [Emoji], which is
// a GitHub-style emoji reference like ":smiley:".
// The caller has checked that s[start] == ':'.
func parseEmoji(p *parser, s string, start int) (x Inline, end int, ok bool) {
	for end := start + 1; ; end++ {
		if end >= len(s) || end-start > 2+maxEmojiLen {
			break
		}
		if s[end] == ':' {
			name := s[start+1 : end]
			end++
			if utf, ok := emoji[name]; ok {
				return &Emoji{s[start:end], utf}, end, true
			}
			break
		}
	}
	return nil, 0, false
}

// maxBackticks is the maximum number of backticks allowed for an inline code span.
// To avoid super-linear (not quite quadratic) behavior, we need to track the last position
// where a run of exactly N backticks was seen, for each possible N, rather than scan
// backward to find them. This means we must place some limit on N (or use a map).
// cmark-gfm imposes a limit of 80, which seems good enough.
// (If your backticks don't fit on a punch card, you can't use them!)
const maxBackticks = 80

// A backtickParser holds the state for parseCodeSpan looking for backticks.
type backtickParser struct {
	last    [maxBackticks]int // last[n] = start offset where final run of n backticks was seen
	scanned bool              // whether we've scanned the string already
}

// reset resets the backtickParser for use with a new string.
func (b *backtickParser) reset() {
	*b = backtickParser{}
}

// parseCodeSpan is (as b.parseCodeSpan) an [inlineParser] for a [Code],
// which is an n-backtick-delimited code span for some n.
// The naive implementation of backtick scanning would take O(n√n) time on an input like
//
//	` `` ``` ```` ````` `````` ``````` ````````
//
// It's not quite quadratic, because you can only make O(√n) scans of suffixes of a string of
// length n, but those will still do O(n√n) character comparisons because there are so many
// more backtick runs toward the start of the string than toward the end.
//
// Successful scans are always fine: they consume all the text they scanned.
// To avoid O(n√n) behavior, an unsuccessful scan records the
// To avoid this, during an unsuccessful scan for any length, we record the
// last location of every run of n backticks for all n, in an array indexed by n-1.
// Then, the next time we do a scan in the string, we can tell whether it will
// be successful by checking whether start < last[n-1]. If not, there's no
// terminator out there and we can avoid scanning.
// Otherwise, there's a guaranteed terminator, so a successful scan
// pays for itself by shortening s by the scan amount.
func (b *backtickParser) parseCodeSpan(p *parser, s string, start int) (x Inline, end int, ok bool) {
	// Count leading backticks. Need to find that many again.
	n := 1
	for start+n < len(s) && s[start+n] == '`' {
		n++
	}

	// If we've already scanned the whole string (for a different count),
	// we can skip a failed scan by checking whether we saw this count.
	// To enable this optimization, following cmark-gfm, we declare by fiat
	// that more than maxBackticks backquotes is too many.
	if n > len(b.last) || b.scanned && b.last[n-1] < start+n {
		goto NoMatch
	}

	for end = start + n; end < len(s); {
		if s[end] != '`' {
			end++
			continue
		}
		estart := end
		for end < len(s) && s[end] == '`' {
			end++
		}
		m := end - estart
		if !b.scanned && m < len(b.last) {
			b.last[m-1] = estart
		}
		if m == n {
			// Match.
			// Line endings are converted to single spaces.
			text := s[start+n : estart]
			text = strings.ReplaceAll(text, "\n", " ")

			// If enclosed text starts and ends with a space and is not all spaces,
			// one space is removed from start and end, to allow `` ` `` to quote a single backquote.
			if len(text) >= 2 && text[0] == ' ' && text[len(text)-1] == ' ' && trimSpace(text) != "" {
				text = text[1 : len(text)-1]
			}

			return &Code{text}, end, true
		}
	}
	b.scanned = true

NoMatch:
	// No match, so none of these backticks count: skip them all.
	// For example ``x` is not a single backtick followed by a code span.
	// Returning nil, 0, false would advance to the second backtick and try again.
	end = start + n
	return &Plain{s[start:end]}, end, true
}

// parseEmph is an [inlineParser] for an emphasis open or close (* _ ~)
// represented as an [emphPlain].
func parseEmph(p *parser, s string, start int) (x Inline, end int, ok bool) {
	c := s[start]
	end = start + 1
	if c == '*' || c == '~' || c == '_' {
		for end < len(s) && s[end] == c {
			end++
		}
	}
	if c == '~' && end-start != 2 {
		// Goldmark does not accept ~text~
		// and incorrectly accepts ~~~text~~~.
		// Only ~~ is correct.
		p.corner = true
	}
	if c == '~' && end-start > 2 {
		// Skip over all the ~ so that we don't see
		// the last two as a marker later and also to
		// avoid quadratic scans over the ~s.
		return &Plain{s[start:end]}, end, true
	}

	// Pick up the runes before and after the end.
	before, after := ' ', ' '
	if start > 0 {
		before, _ = utf8.DecodeLastRuneInString(s[:start])
	}
	if end < len(s) {
		after, _ = utf8.DecodeRuneInString(s[end:])
	}

	// See https://spec.commonmark.org/0.31.2/#emphasis-and-strong-emphasis.
	//
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

	switch c {
	case '\'', '"':
		canOpen = leftFlank && !rightFlank && before != ']' && before != ')'
		canClose = rightFlank
	case '*', '~':
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
	case '_':
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

	x = &emphPlain{
		Plain:    Plain{s[start:end]},
		canOpen:  canOpen,
		canClose: canClose,
		n:        end - start,
	}
	return x, end, true
}

// mergePlain converts emphPlain nodes to Plain nodes
// (openPlain nodes have already been converted)
// and then merges each run of Plain nodes in list to a single Plain node.
func (p *parser) mergePlain(list []Inline) []Inline {
	out := list[:0]
	start := 0
	for i := 0; ; i++ {
		if i < len(list) {
			switch x := list[i].(type) {
			case *Plain:
				continue
			case *emphPlain:
				list[i] = &x.Plain
				continue
			}
		}
		// Non-Plain or end of list.
		if start < i {
			out = append(out, mergePlainRun(list[start:i]))
		}
		if i >= len(list) {
			break
		}
		out = append(out, list[i])
		start = i + 1
	}
	return out
}

// mergePlainRun merges list, which is known to be entirely *Plain nodes,
// down to a single Plain node.
func mergePlainRun(list []Inline) *Plain {
	if len(list) == 1 {
		return list[0].(*Plain)
	}
	var all []string
	for _, pl := range list {
		all = append(all, pl.(*Plain).Text)
	}
	return &Plain{Text: strings.Join(all, "")}
}
