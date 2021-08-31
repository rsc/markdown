// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

/*
TODO

\r handling



SPEC BUGS:

Entity and numeric character references are recognized in any context besides code spans or code blocks, including URLs, link titles, and fenced code block info strings:

Entity and numeric character references are treated as literal text in code spans and code blocks:

should become

Entity and numeric character references are treated as literal text in code spans, code blocks, and raw HTML:

add example 31 here

Entity and numeric character references are recognized in any other context, including URLs, link titles, and fenced code block info strings:

--

[foo]: ###
vs
[foo]:
###



*/

package markdown

import (
	"bytes"
	"fmt"
	"strings"
)

type block struct {
	kind      string
	p         *block
	id        int
	loose     bool
	text      []string
	block     []*block
	width     int
	end       bool
	endFunc   func(string) bool
	endBlank  bool
	info      string
	fence     string
	bullet    byte
	num       int
	startLine int
	endLine   int
}

func parse(text string) *block {
	text = strings.ReplaceAll(text, "\x00", "\xFFFD")
	para = nil
	links = nil
	id = 0
	lineno = 0
	root = &block{kind: "root"}
	for text != "" {
		line := text[:strings.Index(text, "\n")+1]
		text = text[len(line):]
		lineno++
		addLine(root, str{text: line})
	}
	closePara()
	closeAllLists()
	return root
}

var root *block
var para *block
var paraParent *block
var list *block
var links map[string]*Link
var lineno int

var id int

func addBlock(b, c *block) *block {
	if len(b.block) > 0 {
		for e := b.block[len(b.block)-1]; e != nil; e = e.block[len(e.block)-1] {
			if e.kind == "list" {
				closeList(e)
			}
			if len(e.block) == 0 {
				break
			}
		}
	}
	closePara()
	c.p = b
	c.id = id
	c.startLine = lineno
	c.endLine = lineno
	id++
	b.block = append(b.block, c)
	return c
}

func closeList(b *block) {
	if b.end {
		panic("double close")
	}
	b.end = true
Loose:
	for i, c := range b.block {
		if c.kind != "item" {
			panic("list item")
		}
		if i+1 < len(b.block) && b.block[i+1].startLine-c.endLine > 1 {
			b.loose = true
			break Loose
		}
		for j, d := range c.block {
			if j+1 < len(c.block) && c.block[j+1].startLine-d.endLine > 1 {
				b.loose = true
				break Loose
			}
		}
	}

	if !b.loose {
		for _, c := range b.block {
			for _, d := range c.block {
				if d.kind == "para" {
					d.kind = "tpara"
				}
			}
		}
	}
}

func closeAllLists() {
	for b := root; b != nil; b = b.block[len(b.block)-1] {
		if b.kind == "list" {
			closeList(b)
		}
		if len(b.block) == 0 {
			break
		}
	}
}

type str struct {
	spaces int
	i      int
	tab    int
	text   string
}

func addLine(root *block, line str) {
	b := root

	for {
		if !line.isBlank() {
			b.endLine = lineno
		}
		if len(b.block) > 0 {
			c := b.block[len(b.block)-1] // last child of b
			if c.end {
				goto More
			}
			switch c.kind {
			case "pre":
				if line.trimSpace(4, 4, true) {
					c.text = append(c.text, line.string())
					if !line.isBlank() {
						c.endLine = lineno
					}
					return
				}
			case "fence":
				c.endLine = lineno
				var fence, info string
				var n int
				if t := line; t.trimFence(&fence, &info, &n) && strings.HasPrefix(fence, c.fence) && info == "" {
					c.end = true
					return
				}
				if c.width > 0 {
					line.trimSpace(0, c.width, false)
				}
				c.text = append(c.text, line.string())
				return
			case "quote":
				if line.trimQuote() {
					b = c
					continue
				}
				if line.isBlank() {
					c.end = true
					continue
				}
			case "list":
				d := c.block[len(c.block)-1]
				if !d.end && line.trimSpace(d.width, d.width, true) {
					b = d
					continue
				}
			case "html":
				if c.endBlank && line.isBlank() {
					c.end = true
					break
				}
				c.endLine = lineno
				s := line.string()
				c.text = append(c.text, s)
				if c.endFunc != nil && c.endFunc(s) {
					c.end = true
				}
				return
			}
		}
	More:
		// Cannot continue c.
		// Start new block inside b.
		peek := line
		peek2 := line
		var n int
		var fence, info string
		switch {
		case para == nil && peek2.trimSpace(4, 4, false) && !peek2.isBlank():
			addBlock(b, &block{kind: "pre"})
			continue
		case peek.trimQuote():
			addBlock(b, &block{kind: "quote"})
			continue
		case peek.trimHeading(&n):
			s := peek.string()
			s = strings.TrimRight(s, " \t\n")
			if t := strings.TrimRight(s, "#"); t != strings.TrimRight(t, " \t") || t == "" {
				s = t
			}
			b = addBlock(b, &block{kind: "heading", width: n, text: []string{s}})
			return
		case para != nil && b == paraParent && peek.trimSetext(&n):
			p := para
			closePara()
			if len(p.text) == 0 {
				break
			}
			p.kind = "heading"
			p.width = n
			p.endLine = lineno
			return
		case peek.trimHR():
			b = addBlock(b, &block{kind: "hr"})
			return
		case peek.trimList(b, &b):
			line = peek
			continue
		case peek.trimHTML(b, &b):
			return
		case peek.trimFence(&fence, &info, &n):
			b = addBlock(b, &block{kind: "fence", fence: fence, info: info, width: n})
			return
		}
		break
	}

	if line.isBlank() && b.kind != "pre" && b.kind != "fence" {
		var d *block
		for c := b; c != nil && len(c.block) > 0 && !c.block[len(c.block)-1].end; c = d {
			d := c.block[len(c.block)-1]
			if d.kind == "quote" {
				d.end = true
			}
		}
		if b.kind == "item" && len(b.block) == 0 && lineno > b.startLine {
			b.end = true
		}
		closePara()
		return
	}

	if para == nil {
		para = addBlock(b, &block{kind: "para"})
		paraParent = b
	}

	para.endLine = lineno
	para.text = append(para.text, line.string())
}

func closePara() {
	if para == nil {
		return
	}
	s := strings.TrimRight(strings.Join(para.text, ""), "\n")
	for s != "" {
		end, ok := parseLinkRefDef(s)
		if !ok {
			break
		}
		s = s[skipSpace(s, end):]
	}
	if s == "" {
		para.text = nil
	} else {
		para.text = []string{s}
	}
	if para != paraParent.block[len(paraParent.block)-1] {
		panic("bad para")
	}
	if len(para.text) == 0 {
		// Need to keep record of paragraph for line number considerations
		// for loose/tight test, but don't print it.
		para.kind = "empty"
	}
	para = nil
	paraParent = nil
}

func parseLinkRefDef(s string) (int, bool) {
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
	if j < len(s) && s[j] == '\n' {
		moved = true
		j++
	}
	var title string
	if moved {
		for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
			j++
		}
		if t, j, ok := parseLinkTitle(s, j); ok {
			for j < len(s) && (s[j] == ' ' || s[j] == '\t') {
				j++
			}
			if j >= len(s) || s[j] == '\n' {
				i = j
				title = t
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

	if links == nil {
		links = make(map[string]*Link)
	}
	label = normalizeLabel(label)
	if _, ok := links[label]; !ok {
		links[label] = &Link{URL: dest, Title: title}
	}
	return i, true
}

func (s *str) trimQuote() bool {
	t := *s
	t.trimSpace(0, 3, false)
	if !t.trim('>') {
		return false
	}
	t.trimSpace(0, 1, true)
	*s = t
	return true
}

func (s *str) trimHeading(width *int) bool {
	t := *s
	t.trimSpace(0, 3, false)
	if !t.trim('#') {
		return false
	}
	n := 1
	for n < 6 && t.trim('#') {
		n++
	}
	if !t.trimSpace(1, 1, true) {
		return false
	}
	*width = n
	*s = t
	return true
}

func (s *str) trimSetext(n *int) bool {
	t := *s
	t.trimSpace(0, 3, false)
	c := t.peek()
	if c == '-' || c == '=' {
		for t.trim(c) {
		}
		t.skipSpace()
		if t.trim('\n') {
			if c == '=' {
				*n = 1
			} else {
				*n = 2
			}
			return true
		}
	}
	return false
}

func (s *str) trimHR() bool {
	t := *s
	t.trimSpace(0, 3, false)
	switch c := t.peek(); c {
	case '-', '_', '*':
		for i := 0; ; i++ {
			if !t.trim(c) {
				if i >= 3 {
					break
				}
				return false
			}
			t.skipSpace()
		}
		return t.trim('\n')
	}
	return false
}

func (s *str) trimHTML(b *block, bp **block) bool {
	t := strings.TrimLeft(s.text[s.i:], " \t")
	if t == "" || t[0] != '<' {
		return false
	}

	var end string
	switch {
	case strings.HasPrefix(t, "<!--"):
		end = "-->"
	case strings.HasPrefix(t, "<?"):
		end = "?>"
	case strings.HasPrefix(t, "<![CDATA["):
		end = "]]>"
	case strings.HasPrefix(t, "<!") && len(t) >= 3 && isLetter(t[2]):
		end = ">"
		// TODO 1, 7
	}
	if end != "" {
		b = addBlock(b, &block{kind: "html", endFunc: func(s string) bool { return strings.Contains(s, end) }})
		if b.endFunc(t) {
			b.end = true
		}
		b.text = append(b.text, s.string())
		*bp = b
		return true
	}

	// case 6
	i := 1
	if i < len(t) && t[i] == '/' {
		i++
	}
	buf := make([]byte, 0, 16)
	for ; i < len(t) && len(buf) < 16; i++ {
		c := t[i]
		if 'A' <= c && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c < 'a' || 'z' < c {
			break
		}
		buf = append(buf, c)
	}
	if i < len(t) {
		switch t[i] {
		default:
			goto Next
		case ' ', '\t', '\n', '>':
			// ok
		case '/':
			if i+1 >= len(t) || t[i+1] != '>' {
				goto Next
			}
		}
	}

	if len(buf) == 0 {
		goto Next
	}
	{
		c := buf[0]
		var ok bool
		for _, name := range htmlTags {
			if name[0] == c && len(name) == len(buf) && name == string(buf) {
				ok = true
				break
			}
		}
		if !ok {
			goto Next
		}
	}

	b = addBlock(b, &block{kind: "html", endBlank: true})
	b.text = append(b.text, s.string())
	*bp = b
	return true

Next:
	// case 1
	if i < len(t) && t[1] != '/' && (t[i] == ' ' || t[i] == '\t' || t[i] == '\n' || t[i] == '>') {
		switch string(buf) {
		case "pre", "script", "style", "textarea":
			b = addBlock(b, &block{kind: "html", endFunc: hasEndPre})
			if hasEndPre(t) {
				b.end = true
			}
			b.text = append(b.text, s.string())
			*bp = b
			return true
		}
	}

	// case 7
	if para == nil {
		if _, e, ok := parseHTMLOpenTag(t, 0); ok && skipSpace(t, e) == len(t) {
			b = addBlock(b, &block{kind: "html", endBlank: true})
			b.text = append(b.text, s.string())
			*bp = b
			return true
		}
		if _, e, ok := parseHTMLClosingTag(t, 0); ok && skipSpace(t, e) == len(t) {
			b = addBlock(b, &block{kind: "html", endBlank: true})
			b.text = append(b.text, s.string())
			*bp = b
			return true
		}
	}

	return false
}

func hasEndPre(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == '<' && i+1 < len(s) && s[i+1] == '/' {
			buf := make([]byte, 0, 8)
			for i += 2; i < len(s) && len(buf) < 8; i++ {
				c := s[i]
				if 'A' <= c && c <= 'Z' {
					c += 'a' - 'A'
				}
				if c < 'a' || 'z' < c {
					break
				}
				buf = append(buf, c)
			}
			if i < len(s) && s[i] == '>' {
				switch string(buf) {
				case "pre", "script", "style", "textarea":
					return true
				}
			}
		}
	}
	return false
}

func (s *str) trimFence(fence, info *string, n *int) bool {
	t := *s
	*n = 0
	for *n < 3 && t.trimSpace(1, 1, false) {
		*n++
	}
	switch c := t.peek(); c {
	case '`', '~':
		f := t.trimString()
		n := 0
		for i := 0; ; i++ {
			if !t.trim(c) {
				if i >= 3 {
					break
				}
				return false
			}
			n++
		}
		txt := mdUnescaper.Replace(t.trimString())
		if c == '`' && strings.Contains(txt, "`") {
			return false
		}
		i := strings.IndexAny(txt, " \t\n")
		if i >= 0 {
			txt = txt[:i]
		}
		*info = txt

		*fence = f[:n]
		*s = str{}
		return true
	}
	return false
}

var htmlTags = []string{
	"address",
	"article",
	"aside",
	"base",
	"basefont",
	"blockquote",
	"body",
	"caption",
	"center",
	"col",
	"colgroup",
	"dd",
	"details",
	"dialog",
	"dir",
	"div",
	"dl",
	"dt",
	"fieldset",
	"figcaption",
	"figure",
	"footer",
	"form",
	"frame",
	"frameset",
	"h1",
	"h2",
	"h3",
	"h4",
	"h5",
	"h6",
	"head",
	"header",
	"hr",
	"html",
	"iframe",
	"legend",
	"li",
	"link",
	"main",
	"menu",
	"menuitem",
	"nav",
	"noframes",
	"ol",
	"optgroup",
	"option",
	"p",
	"param",
	"section",
	"source",
	"summary",
	"table",
	"tbody",
	"td",
	"tfoot",
	"th",
	"thead",
	"title",
	"tr",
	"track",
	"ul",
}

func (s *str) peek() byte {
	return s.text[s.i]
}

func (s *str) trimList(b *block, bp **block) bool {
	t := *s
	n := 0
	for i := 0; i < 3; i++ {
		if !t.trimSpace(1, 1, false) {
			break
		}
		n++
	}
	bullet := t.peek()
	var num int
Switch:
	switch bullet {
	default:
		return false
	case '-', '*', '+':
		t.trim(bullet)
		n++
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		for j := t.i; ; j++ {
			if j >= len(t.text) {
				return false
			}
			c := t.text[j]
			if c == '.' || c == ')' {
				// success
				bullet = c
				j++
				n += j - t.i
				t.i = j
				break Switch
			}
			if c < '0' || '9' < c {
				return false
			}
			if j-t.i >= 9 {
				return false
			}
			num = num*10 + int(c) - '0'
		}

	}
	if !t.trimSpace(1, 1, true) {
		return false
	}
	n++
	tt := t
	m := 0
	for i := 0; i < 3 && tt.trimSpace(1, 1, false); i++ {
		m++
	}
	if !tt.trimSpace(1, 1, true) {
		n += m
		t = tt
	}

	// point of no return

	var list *block
	if len(b.block) > 0 {
		c := b.block[len(b.block)-1]
		if c.kind == "list" {
			list = c
		}
	}
	if list == nil || list.bullet != bullet {
		// “When the first list item in a list interrupts a paragraph—that is,
		// when it starts on a line that would otherwise count as
		// paragraph continuation text—then (a) the lines Ls must
		// not begin with a blank line,
		// and (b) if the list item is ordered, the start number must be 1.”
		if list == nil && para != nil && (t.isBlank() || num > 1) {
			return false
		}
		list = addBlock(b, &block{kind: "list", bullet: bullet, num: num})
	}
	b = addBlock(list, &block{kind: "item", width: n})
	*bp = b
	*s = t
	return true
}

func (s *str) skipSpace() {
	s.spaces = 0
	for s.i < len(s.text) && (s.text[s.i] == ' ' || s.text[s.i] == '\t') {
		s.i++
	}
}

func (s *str) trimSpace(min, max int, eolOK bool) bool {
	t := *s
	for n := 0; n < max; n++ {
		if t.spaces > 0 {
			t.spaces--
			continue
		}
		if t.i < len(t.text) {
			switch t.text[t.i] {
			case '\t':
				t.spaces = 4 - (t.i-t.tab)&3 - 1
				t.i++
				t.tab = t.i
				continue
			case ' ':
				t.i++
				continue
			case '\n':
				if eolOK {
					continue
				}
			}
		}
		if n >= min {
			break
		}
		return false
	}
	*s = t
	return true
}

func (s *str) trim(c byte) bool {
	if s.i < len(s.text) && s.text[s.i] == c {
		s.i++
		return true
	}
	return false
}

func (s *str) trimPrefix(prefix string) bool {
	t := *s

	for i := 0; i < len(prefix); i++ {
		if prefix[i] == ' ' {
			if t.spaces == 0 && t.i < len(t.text) {
				switch t.text[t.i] {
				case '\t':
					t.spaces = 4 - (t.i-t.tab)&3
					t.i++
					t.tab = t.i
				case ' ':
					t.spaces = 1
					t.i++
				case '\n':
					t.spaces = 1
				}
			}
			if t.spaces == 0 {
				return false
			}
			t.spaces--
		} else {
			if t.i >= len(t.text) || t.text[t.i] != prefix[i] {
				return false
			}
			t.i++
		}
	}

	*s = t
	return true
}

func (s *str) string() string {
	switch s.spaces {
	case 0:
		return s.text[s.i:]
	case 1:
		return " " + s.text[s.i:]
	case 2:
		return "  " + s.text[s.i:]
	case 3:
		return "   " + s.text[s.i:]
	}
	panic("bad spaces")
}

func (s *str) isBlank() bool {
	return strings.Trim(s.text[s.i:], " \t\n") == ""
}

func (s *str) trimString() string {
	return strings.Trim(s.text[s.i:], " \t\n")
}

func toHTML(b *block) string {
	var buf bytes.Buffer
	printTo(&buf, b)
	return buf.String()
}

func printTo(buf *bytes.Buffer, b *block) {
	switch b.kind {
	case "root":
		for _, c := range b.block {
			printTo(buf, c)
		}
	case "heading":
		fmt.Fprintf(buf, "<h%d>", b.width)
		printInline(buf, strings.TrimSuffix(strings.Join(b.text, ""), "\n"))
		fmt.Fprintf(buf, "</h%d>\n", b.width)
	case "quote":
		buf.WriteString("<blockquote>\n")
		for _, c := range b.block {
			printTo(buf, c)
		}
		buf.WriteString("</blockquote>\n")
	case "list":
		if b.bullet == '.' || b.bullet == ')' {
			buf.WriteString("<ol")
			if b.num != 1 {
				fmt.Fprintf(buf, " start=\"%d\"", b.num)
			}
			buf.WriteString(">\n")
		} else {
			buf.WriteString("<ul>\n")
		}
		for _, c := range b.block {
			printTo(buf, c)
		}
		if b.bullet == '.' || b.bullet == ')' {
			buf.WriteString("</ol>\n")
		} else {
			buf.WriteString("</ul>\n")
		}
	case "item":
		buf.WriteString("<li>")
		if len(b.block) > 0 && b.block[0].kind != "tpara" {
			buf.WriteString("\n")
		}
		for i, c := range b.block {
			printTo(buf, c)
			if i+1 < len(b.block) && c.kind == "tpara" {
				buf.WriteString("\n")
			}
		}
		buf.WriteString("</li>\n")
	case "pre":
		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString("<pre><code>")
		for len(b.text) > 0 && b.text[len(b.text)-1] == "\n" {
			b.text = b.text[:len(b.text)-1]
		}
		for _, s := range b.text {
			buf.WriteString(htmlEscaper.Replace(s))
		}
		buf.WriteString("</code></pre>\n")
	case "fence":
		if buf.Len() > 0 && buf.Bytes()[buf.Len()-1] != '\n' {
			buf.WriteString("\n")
		}
		buf.WriteString("<pre><code")
		if b.info != "" {
			fmt.Fprintf(buf, " class=\"language-%s\"", htmlQuoteEscaper.Replace(b.info))
		}
		buf.WriteString(">")
		for _, s := range b.text {
			buf.WriteString(htmlEscaper.Replace(s))
		}
		buf.WriteString("</code></pre>\n")
	case "para", "tpara":
		if b.kind == "para" {
			buf.WriteString("<p>")
		}
		printInline(buf, b.text[0])
		if b.kind == "para" {
			buf.WriteString("</p>\n")
		}
	case "hr":
		buf.WriteString("<hr />\n")
	case "html":
		for _, s := range b.text {
			buf.WriteString(s)
		}
	case "empty":
		// nothing
	default:
		buf.WriteString("???" + b.kind)
	}
}

func printb(buf *bytes.Buffer, b *block, prefix string) {
	fmt.Fprintf(buf, "(%s %d-%d", b.kind, b.startLine, b.endLine)
	if b.end {
		fmt.Fprintf(buf, " E")
	}
	if b.loose {
		fmt.Fprintf(buf, " loose")
	}
	if b.width > 0 {
		fmt.Fprintf(buf, " w=%d", b.width)
	}
	for _, line := range b.text {
		fmt.Fprintf(buf, "\n%s\t%q", prefix, line)
	}
	prefix += "\t"
	for _, b := range b.block {
		fmt.Fprintf(buf, "\n%s", prefix)
		printb(buf, b, prefix)
	}
	fmt.Fprintf(buf, ")")
}

func dump(b *block) string {
	var buf bytes.Buffer
	printb(&buf, b, "")
	return buf.String()
}

func printInline(buf *bytes.Buffer, s string) {
	for _, x := range inline(s) {
		x.htmlTo(buf)
	}
}

func oldPrintInline(buf *bytes.Buffer, s string) {
	start := 0
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && canEscape(s[i+1]) {
			printString(buf, s[start:i])
			start = i + 1
			i += 2
			continue
		}
		i++
	}
	printString(buf, s[start:])
}

func printString(buf *bytes.Buffer, s string) {
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			buf.WriteString(s[start:i])
			buf.WriteString("&quot;")
			start = i + 1
		case '&':
			buf.WriteString(s[start:i])
			buf.WriteString("&amp;")
			start = i + 1
		case '<':
			buf.WriteString(s[start:i])
			buf.WriteString("&lt;")
			start = i + 1
		case '>':
			buf.WriteString(s[start:i])
			buf.WriteString("&gt;")
			start = i + 1
		}
	}
	buf.WriteString(s[start:])
}

func canEscape(c byte) bool {
	return '!' <= c && c <= '/' ||
		':' <= c && c <= '@' ||
		'[' <= c && c <= '`' ||
		'{' <= c && c <= '~'
}
