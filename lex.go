// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

import (
	"strings"
	"unicode"
)

// isPunct reports whether c is Markdown punctuation.
func isPunct(c byte) bool {
	return '!' <= c && c <= '/' || ':' <= c && c <= '@' || '[' <= c && c <= '`' || '{' <= c && c <= '~'
}

// isLetter reports whether c is an ASCII letter.
func isLetter(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z'
}

// isDigit reports whether c is an ASCII digit.
func isDigit(c byte) bool {
	return '0' <= c && c <= '9'
}

// isLetterDigit reports whether c is an ASCII letter or digit.
func isLetterDigit(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9'
}

// isLDH reports whether c is an ASCII letter, digit, or hyphen.
func isLDH(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' || c == '-'
}

// isHexDigit reports whether c is an ASCII hexadecimal digit.
func isHexDigit(c byte) bool {
	return 'A' <= c && c <= 'F' || 'a' <= c && c <= 'f' || '0' <= c && c <= '9'
}

// isUnocdeSpace reports whether r is a Unicode space as defined by Markdown.
// This is not the same as unicode.IsSpace.
// For example, U+0085 does not satisfy isUnicodeSpace
// but does satisfy unicode.IsSpace.
func isUnicodeSpace(r rune) bool {
	if r < 0x80 {
		return r == ' ' || r == '\t' || r == '\f' || r == '\n'
	}
	return unicode.In(r, unicode.Zs)
}

// isUnocdeSpace reports whether r is Unicode punctuation as defined by Markdown.
// This is not the same as unicode.Punct; it also includes unicode.Symbol.
func isUnicodePunct(r rune) bool {
	if r < 0x80 {
		return isPunct(byte(r))
	}
	return unicode.In(r, unicode.Punct, unicode.Symbol)
}

// skipSpace returns i + the number of spaces, tabs, carriage returns, and newlines
// at the start of s[i:]. That is, it skips i past any such characters, returning the new i.
func skipSpace(s string, i int) int {
	// Note: Blank lines have already been removed.
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	return i
}

// mdEscaper escapes symbols that are used in inline Markdown sequences.
// TODO(rsc): There is a better way to do this.
var mdEscaper = strings.NewReplacer(
	`(`, `\(`,
	`)`, `\)`,
	`[`, `\[`,
	`]`, `\]`,
	`*`, `\*`,
	`_`, `\_`,
	`<`, `\<`,
	`>`, `\>`,
)

// mdLinkEscaper escapes symbols that have meaning inside a link target.
var mdLinkEscaper = strings.NewReplacer(
	`(`, `\(`,
	`)`, `\)`,
	`<`, `\<`,
	`>`, `\>`,
)

// mdUnscape returns the Markdown unescaping of s.
func mdUnescape(s string) string {
	if !strings.Contains(s, `\`) && !strings.Contains(s, `&`) {
		return s
	}
	return mdUnescaper.Replace(s)
}

// mdUnescaper unescapes Markdown escape sequences and HTML entities.
// TODO(rsc): Perhaps there is a better way to do this.
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
