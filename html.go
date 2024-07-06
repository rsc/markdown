// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strconv"
	"strings"
	"unicode"
)

// An HTMLBlock is a [Block] representing an [HTML block].
//
// [HTML block]: https://spec.commonmark.org/0.31.2/#html-blocks
type HTMLBlock struct {
	Position
	// TODO should these be 'Text string'?
	Text []string // lines, without trailing newlines
}

func (*HTMLBlock) Block() {}

func (b *HTMLBlock) printHTML(p *printer) {
	for _, s := range b.Text {
		p.html(s)
		p.html("\n")
	}
}

func (b *HTMLBlock) printMarkdown(p *printer) {
	p.maybeNL()
	for i, line := range b.Text {
		if i > 0 {
			p.nl()
		}
		p.WriteString(line)
		p.noTrim()
	}
}

// An htmlBuilder is a [blockBuilder] for an [HTMLBlock].
// If endBlank is true, the block ends immediately before the first blank line.
// If endFunc is non-nil, the block ends immediately after the first line
// for which endFunc returns true.
type htmlBuilder struct {
	endBlank bool
	endFunc  func(string) bool
	text     []string //accumulated text
}

func (c *htmlBuilder) extend(p *parser, s line) (line, bool) {
	if c.endBlank && s.isBlank() {
		return s, false
	}
	t := s.string()
	c.text = append(c.text, t)
	if c.endFunc != nil && c.endFunc(t) {
		return line{}, false
	}
	return line{}, true
}

func (c *htmlBuilder) build(p *parser) Block {
	return &HTMLBlock{
		p.pos(),
		c.text,
	}
}

// An HTMLTag is an [Inline] representing a [raw HTML tag].
//
// [raw HTML tag]: https://spec.commonmark.org/0.31.2/#raw-html
type HTMLTag struct {
	Text string // TODO rename to HTML?
}

func (*HTMLTag) Inline() {}

func (x *HTMLTag) printHTML(p *printer) {
	p.html(x.Text)
}

func (x *HTMLTag) printMarkdown(p *printer) {
	// TODO are there newlines? probably not
	for i, line := range strings.Split(x.Text, "\n") {
		if i > 0 {
			p.nl()
		}
		p.WriteString(line)
		p.noTrim()
	}
}

func (x *HTMLTag) printText(p *printer) {}

// startHTMLBlock is a [starter] for an [HTMLBlock].
//
// See https://spec.commonmark.org/0.31.2/#html-blocks.
func startHTMLBlock(p *parser, s line) (line, bool) {
	// Early out: block must start with a <.
	tt := s
	tt.trimSpace(0, 3, false) // TODO figure out trimSpace final argument
	if tt.peek() != '<' {
		return s, false
	}
	t := tt.string()

	// Check all 7 block types.
	if startHTMLBlock1(p, s, t) ||
		startHTMLBlock2345(p, s, t) ||
		startHTMLBlock6(p, s, t) ||
		startHTMLBlock7(p, s, t) {
		return line{}, true
	}

	return s, false
}

const forceLower = 0x20 // ASCII letter | forceLower == ASCII lower-case

// startHTMLBlock1 handles HTML block type 1:
// line starting with <pre, <script, <style, or <textarea
// up through a (not necessarily matching) closing </pre> </script> </style> or </textarea>.
//
// s is the entire line, for saving if starting a block.
// t is the line as a string, with leading spaces removed; it starts with <.
func startHTMLBlock1(p *parser, s line, t string) bool {
	if len(t) < 2 {
		return false
	}
	if c := t[1] | forceLower; c != 'p' && c != 's' && c != 't' { // early out; check first letter
		return false
	}
	i := 2
	for i < len(t) && (t[i] != ' ' && t[i] != '\t' && t[i] != '>') {
		i++
	}
	if !isBlock1Tag(t[1:i]) {
		return false
	}
	b := &htmlBuilder{endFunc: endBlock1}
	p.addBlock(b)
	b.text = append(b.text, s.string())
	if endBlock1(t) {
		p.closeBlock()
	}
	return true
}

// endBlock1 reports whether the string contains
// </pre>, </script>, </style>, or </textarea>,
// using ASCII case-insensitive matching.
func endBlock1(s string) bool {
	start := -1
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1 < len(s) && s[i+1] == '/' {
			start = i + 2
		}
		if s[i] == '>' && start >= 0 {
			if isBlock1Tag(s[start:i]) {
				return true
			}
			start = -1
		}
	}
	return false
}

// isBlock1Tag reports whether tag is a tag that can open or close
// HTML block type 1.
func isBlock1Tag(tag string) bool {
	return lowerEq(tag, "pre") || lowerEq(tag, "script") || lowerEq(tag, "style") || lowerEq(tag, "textarea")
}

// lowerEq reports whether strings.ToLower(s) == lower
// assuming lower is entirely ASCII lower-case letters.
func lowerEq(s, lower string) bool {
	if len(s) != len(lower) {
		return false
	}
	lower = lower[:len(s)]
	for i := 0; i < len(s); i++ {
		if s[i]|forceLower != lower[i] {
			return false
		}
	}
	return true
}

// startHTMLBlock2345 handles HTML blocks types 2, 3, 4, and 5,
// the ones that start and end a specific string constant.
//
// s is the entire line, for saving if starting a block.
// t is the line as a string, with leading spaces removed; it starts with <.
func startHTMLBlock2345(p *parser, s line, t string) bool {
	var end string
	switch {
	default:
		return false

	// type 2: <!-- .. -->, or <!--> or <!---> because of simplistic parsing.
	case strings.HasPrefix(t, "<!--"): // type 2
		end = "-->"

	// type 3: <? ... ?>, or <?> because of simplistic parsing.
	case strings.HasPrefix(t, "<?"): // type 3
		end = "?>"

	// type 4: <![CDATA[ ... ]]>
	case strings.HasPrefix(t, "<![CDATA["):
		end = "]]>"

	// type 5: <!TEXT .. >
	// The spec says nothing about requiring a leading upper-case letter,
	// only that it should be an ASCII letter, but cmark-gfm, Goldmark,
	// and the Dingus all require upper-case, so we do too.
	// Presumably this is because the actual goal is to recognize the few
	// XML definitions that can appear, and they are all upper-case.
	// The result is that <!X> is an HTMLBlock but <!x> is an HTMLTag.
	// That's inconsistent, but Markdown is full of them, so we prioritize
	// consistency with all the existing implementations.
	case strings.HasPrefix(t, "<!") && len(t) >= 3 && 'A' <= t[2] && t[2] <= 'Z':
		end = ">"
	}

	b := &htmlBuilder{endFunc: func(s string) bool { return strings.Contains(s, end) }}
	p.addBlock(b)
	b.text = append(b.text, s.string())
	if b.endFunc(t) {
		// If terminator appears on the starting line, we're done.
		p.closeBlock()
	}
	return true
}

// startHTMLBlock6 handles HTML block type 6,
// which starts with the start of a recognized tag
// and ends at a blank line.
//
// s is the entire line, for saving if starting a block.
// t is the line as a string, with leading spaces removed; it starts with <.
func startHTMLBlock6(p *parser, s line, t string) bool {
	// Skip over < or </.
	start := 1
	if len(t) > 1 && t[1] == '/' {
		start = 2
	}

	// Scan ASCII alphanumeric tag name;
	// must be followed by space, tab, >, />, or end of line.
	end := start
	for end < len(t) && end < 16 && isLetterDigit(t[end]) {
		end++
	}
	if end < len(t) {
		switch t[end] {
		default:
			return false
		case ' ', '\t', '>':
			// ok
		case '/':
			if end+1 >= len(t) || t[end+1] != '>' {
				return false
			}
		}
	}

	// Check whether tag is a recognized name.
	tag := t[start:end]
	if tag == "" {
		return false
	}
	c := tag[0] | forceLower
	for _, name := range htmlTags {
		if name[0] == c && len(name) == len(tag) && lowerEq(tag, name) {
			if end < len(t) && t[end] == '\t' {
				// Goldmark recognizes space but not tab.
				// testdata/extra.txt 143.md
				p.corner = true
			}
			b := &htmlBuilder{endBlank: true}
			p.addBlock(b)
			b.text = append(b.text, s.string())
			return true
		}
	}
	return false
}

// startHTMLBlock7 handles HTML block type 7,
// which starts with a complete tag on a line by itself
// and ends at a blank line.
//
// s is the entire line, for saving if starting a block.
// t is the line as a string, with leading spaces removed; it starts with <.
func startHTMLBlock7(p *parser, s line, t string) bool {
	// Type 7 blocks cannot interrupt a paragraph,
	// so that rewrapping a paragraph with inline tags
	// cannot change them into starting an HTML block.
	if p.para() != nil {
		return false
	}

	if _, end, ok := parseHTMLOpenTag(p, t, 0); ok && skipSpace(t, end) == len(t) {
		if end != len(t) {
			// Goldmark disallows trailing space
			p.corner = true
		}
		b := &htmlBuilder{endBlank: true}
		p.addBlock(b)
		b.text = append(b.text, s.string())
		return true
	}
	if _, end, ok := parseHTMLClosingTag(p, t, 0); ok && skipSpace(t, end) == len(t) {
		b := &htmlBuilder{endBlank: true}
		p.addBlock(b)
		b.text = append(b.text, s.string())
		return true
	}
	return false
}

// parseHTMLTag is an [inlineParser] for an [HTMLTag].
// The caller has has checked that s[start] is '<'.
func parseHTMLTag(p *parser, s string, start int) (x Inline, end int, ok bool) {
	// “An HTML tag consists of an open tag, a closing tag, an HTML comment,
	// a processing instruction, a declaration, or a CDATA section.”
	if len(s)-start < 3 || s[start] != '<' {
		return
	}
	switch s[start+1] {
	default:
		return parseHTMLOpenTag(p, s, start)
	case '/':
		return parseHTMLClosingTag(p, s, start)
	case '!':
		switch s[start+2] {
		case '-':
			return parseHTMLComment(p, s, start)
		case '[':
			return parseHTMLCDATA(p, s, start)
		default:
			return parseHTMLDecl(p, s, start)
		}
	case '?':
		return parseHTMLProcInst(p, s, start)
	}
}

// parseHTMLOpenTag is an [inlineParser] for an HTML open tag.
// The caller has has checked that s[start] is '<'.
func parseHTMLOpenTag(p *parser, s string, i int) (x Inline, end int, ok bool) {
	// “An open tag consists of a < character, a tag name, zero or more attributes,
	// optional spaces, tabs, and up to one line ending, an optional / character, and a > character.”

	// < character
	if i >= len(s) || s[i] != '<' {
		// unreachable unless called wrong
		return
	}

	// tag name
	name, j, ok1 := parseTagName(s, i+1)
	if !ok1 {
		return
	}
	switch name {
	case "pre", "script", "style", "textarea":
		// Goldmark treats these as starting a new HTMLBlock
		// and ending the paragraph they appear in.
		p.corner = true
	}

	// zero or more attributes
	for {
		if j >= len(s) || s[j] != ' ' && s[j] != '\t' && s[j] != '\n' && s[j] != '/' && s[j] != '>' {
			return
		}
		_, k, ok := parseAttr(p, s, skipSpace(s, j))
		if !ok {
			break
		}
		j = k
	}

	// optional spaces, tabs, and up to one line ending
	k := skipSpace(s, j)
	if k != j {
		// Goldmark mishandles spaces before >.
		p.corner = true
	}
	j = k

	// an optional / character
	if j < len(s) && s[j] == '/' {
		j++
	}

	// and a > character.
	if j >= len(s) || s[j] != '>' {
		return
	}

	return &HTMLTag{s[i : j+1]}, j + 1, true
}

// parseHTMLClosingTag is an [inlineParser] for an HTML closing tag.
// The caller has has checked that s[start:] begins with "</".
func parseHTMLClosingTag(p *parser, s string, i int) (x Inline, end int, ok bool) {
	// “A closing tag consists of the string </, a tag name,
	// optional spaces, tabs, and up to one line ending, and the character >.”
	if i+2 >= len(s) || s[i] != '<' || s[i+1] != '/' {
		return
	}
	if skipSpace(s, i+2) != i+2 {
		// Goldmark allows spaces here but the spec and the Dingus do not.
		p.corner = true
	}

	if _, j, ok := parseTagName(s, i+2); ok {
		j = skipSpace(s, j)
		if j < len(s) && s[j] == '>' {
			return &HTMLTag{s[i : j+1]}, j + 1, true
		}
	}
	return
}

// parseTagName parses a leading tag name from s[start:],
// returning the tag and the end location.
func parseTagName(s string, start int) (tag string, end int, ok bool) {
	// “A tag name consists of an ASCII letter followed by zero or more ASCII letters, digits, or hyphens (-).”
	if start >= len(s) || !isLetter(s[start]) {
		return
	}
	end = start + 1
	for end < len(s) && isLDH(s[end]) {
		end++
	}
	return s[start:end], end, true
}

// parseAttr parses a leading attr (or attr=value) from s[start:],
// returning the entire attribute (including the =value) and the end location.
func parseAttr(p *parser, s string, start int) (attr string, end int, ok bool) {
	// “An attribute consists of spaces, tabs, and up to one line ending,
	// an attribute name, and an optional attribute value specification.”
	_, end, ok = parseAttrName(s, start)
	if !ok {
		return
	}
	if endVal, ok := parseAttrValueSpec(p, s, end); ok {
		end = endVal
	}
	return s[start:end], end, true
}

// parseAttrName parses a leading attribute name from s[start:],
// returning the name and the end location.
func parseAttrName(s string, start int) (name string, end int, ok bool) {
	// “An attribute name consists of an ASCII letter, _, or :,
	// followed by zero or more ASCII letters, digits, _, ., :, or -.”
	if start+1 >= len(s) || (!isLetter(s[start]) && s[start] != '_' && s[start] != ':') {
		return
	}
	end = start + 1
	for end < len(s) && (isLDH(s[end]) || s[end] == '_' || s[end] == '.' || s[end] == ':') {
		end++
	}
	return s[start:end], end, true
}

// parseAttrValueSpec parses a leading attribute value specification
// from s[start:], returning the end location.
func parseAttrValueSpec(p *parser, s string, start int) (end int, ok bool) {
	// “An attribute value specification consists of
	// optional spaces, tabs, and up to one line ending,
	// a = character,
	// optional spaces, tabs, and up to one line ending,
	// and an attribute value.”
	end = skipSpace(s, start)
	if end >= len(s) || s[end] != '=' {
		return
	}
	end = skipSpace(s, end+1)

	// “An attribute value consists of
	// an unquoted attribute value,
	// a single-quoted attribute value,
	// or a double-quoted attribute value.”
	// TODO: No escaping???
	if end < len(s) && (s[end] == '\'' || s[end] == '"') {
		// “A single-quoted attribute value consists of ',
		// zero or more characters not including ', and a final '.”
		// “A double-quoted attribute value consists of ",
		// zero or more characters not including ", and a final ".”
		i := strings.IndexByte(s[end+1:], s[end])
		if i < 0 {
			return
		}
		return end + 1 + i + 1, true
	}

	// “An unquoted attribute value is a nonempty string of characters
	// not including spaces, tabs, line endings, ", ', =, <, >, or `.”
	isAttrVal := func(c byte) bool {
		return c != ' ' && c != '\t' && c != '\n' &&
			c != '"' && c != '\'' &&
			c != '=' && c != '<' && c != '>' && c != '`'
	}
	i := end
	for i < len(s) && isAttrVal(s[i]) {
		i++
	}
	if i == end {
		return
	}
	return i, true
}

// parseHTMLComment is an [inlineParser] for an HTML comment.
// The caller has has checked that s[start:] begins with "<!-".
func parseHTMLComment(p *parser, s string, start int) (x Inline, end int, ok bool) {
	// “An HTML comment consists of <!-- + text + -->,
	// where text does not start with > or ->,
	// does not end with -, and does not contain --.”
	if strings.HasPrefix(s[start:], "<!-->") {
		end = start + len("<!-->")
		return &HTMLTag{s[start:end]}, end, true
	}
	if strings.HasPrefix(s[start:], "<!--->") {
		end = start + len("<!--->")
		return &HTMLTag{s[start:end]}, end, true
	}
	if x, end, ok := parseHTMLMarker(p, s, start, "<!--", "-->"); ok {
		return x, end, ok
	}
	return
}

// parseHTMLCDATA is an [inlineParser] for an HTML CDATA section.
// The caller has has checked that s[start:] begins with "<![".
func parseHTMLCDATA(p *parser, s string, i int) (x Inline, end int, ok bool) {
	// “A CDATA section consists of the string <![CDATA[,
	// a string of characters not including the string ]]>, and the string ]]>.”
	return parseHTMLMarker(p, s, i, "<![CDATA[", "]]>")
}

// parseHTMLDecl is an [inlineParser] for an HTML declaration section.
// The caller has has checked that s[start:] begins with "<!".
func parseHTMLDecl(p *parser, s string, i int) (x Inline, end int, ok bool) {
	// “A declaration consists of the string <!, an ASCII letter,
	// zero or more characters not including the character >, and the character >.”
	if i+2 < len(s) && isLetter(s[i+2]) {
		if 'a' <= s[i+2] && s[i+2] <= 'z' {
			p.corner = true // goldmark requires uppercase
		}
		return parseHTMLMarker(p, s, i, "<!", ">")
	}
	return
}

// parseHTMLDecl is an [inlineParser] for an HTML processing instruction.
// The caller has has checked that s[start:] begins with "<?".
func parseHTMLProcInst(p *parser, s string, i int) (x Inline, end int, ok bool) {
	// “A processing instruction consists of the string <?,
	// a string of characters not including the string ?>, and the string ?>.”
	return parseHTMLMarker(p, s, i, "<?", "?>")
}

// parseHTMLMarker is a generalized parser for the
// various prefix/suffix-denote HTML markers.
// If s[start:] starts with prefix and is followed eventually by suffix,
// then parseHTMLMarker returns an HTMLTag for that section of s
// along with start, end, ok to implement the result of an [inlineParser].
func parseHTMLMarker(p *parser, s string, start int, prefix, suffix string) (x Inline, end int, ok bool) {
	if strings.HasPrefix(s[start:], prefix) {
		// To avoid quadratic behavior looking at <!-- <!-- <!-- <!-- ...
		// we record when a search for a terminator has failed on this line
		// and don't bother to search again.
		switch suffix[0] {
		case ']':
			if p.noCDATAEnd {
				return
			}
		case '>':
			if p.noDeclEnd {
				return
			}
		case '-':
			if p.noCommentEnd {
				return
			}
		case '?':
			if p.noProcInstEnd {
				return
			}
		}

		if i := strings.Index(s[start+len(prefix):], suffix); i >= 0 {
			end = start + len(prefix) + i + len(suffix)
			return &HTMLTag{s[start:end]}, end, true
		}

		p.noDeclEnd = true // no > on line
		switch suffix[0] {
		case ']':
			p.noCDATAEnd = true // no ]]> on line
		case '-':
			p.noCommentEnd = true // no --> on line
		case '?':
			p.noProcInstEnd = true // no ?> on line
		}
	}
	return
}

// parseHTMLEntity is an [inlineParser] for an HTML entity reference,
// such as &quot;, &#123;, or &#x12AB;.
func parseHTMLEntity(_ *parser, s string, start int) (x Inline, end int, ok bool) {
	i := start
	if i+1 < len(s) && s[i+1] == '#' {
		i += 2
		var r int
		if i < len(s) && (s[i] == 'x' || s[i] == 'X') {
			// hex
			i++
			j := i
			for j < len(s) && isHexDigit(s[j]) {
				j++
			}
			if j-i < 1 || j-i > 6 || j >= len(s) || s[j] != ';' {
				return
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
				return
			}
			r, _ = strconv.Atoi(s[i:j])
			end = j + 1
		}
		if r > unicode.MaxRune || r == 0 {
			// Invalid code points and U+0000 are replaced by U+FFFD.
			r = unicode.ReplacementChar
		}
		return &Plain{string(rune(r))}, end, true
	}

	// Max name in list is 32 bytes. Try for 64 for good measure.
	for j := i + 1; j < len(s) && j-i < 64; j++ {
		if s[j] == '&' { // Stop possible quadratic search on &&&&&&&.
			break
		}
		if s[j] == ';' {
			if r, ok := htmlEntity[s[i:j+1]]; ok {
				return &Plain{r}, j + 1, true
			}
			break
		}
	}

	return
}
