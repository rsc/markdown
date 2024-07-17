// Copyright 2024 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package markdown

type line struct {
	spaces   int
	i        int
	tab      int
	text     string
	nl       byte // newline character ending this line: \r or \n or \r+\n or zero for EOF
	nonblank int  // index of first non-space, non-tab char in text; len(text) if none
}

func makeLine(text string, nl byte) line {
	s := line{text: text, nl: nl}
	s.setNonblank()
	return s
}

func (s *line) setNonblank() {
	i := s.i
	for i < len(s.text) && (s.text[i] == ' ' || s.text[i] == '\t') {
		i++
	}
	s.nonblank = i
}

func (s *line) peek() byte {
	if s.spaces > 0 {
		return ' '
	}
	if s.i >= len(s.text) {
		return 0
	}
	return s.text[s.i]
}

func (s *line) skipSpace() {
	s.spaces = 0
	if s.nonblank < s.i {
		panic("nonblank")
	}
	s.i = s.nonblank
}

func (s *line) trimSpace(min, max int, eolOK bool) bool {
	t := *s

	for n := 0; n < max; n++ {
		if t.spaces > 0 {
			t.spaces--
			continue
		}
		if t.i >= len(t.text) && eolOK {
			continue
		}
		// TODO performance bottleneck here using trimSpace with list extensions?
		// but each only fails once?
		if t.i < len(t.text) {
			switch t.text[t.i] {
			case '\t':
				t.spaces = 4 - (t.i-t.tab)&3 - 1
				t.i++
				t.tab = t.i // TODO seems wrong
				continue
			case ' ':
				t.i++
				continue
			}
		}
		if n >= min {
			break
		}
		return false
	}
	if t.nonblank < t.i {
		t.setNonblank()
	}
	*s = t
	return true
}

func (s *line) trim(c byte) bool {
	if s.spaces > 0 {
		if c == ' ' {
			s.spaces--
			return true
		}
		return false
	}
	if s.i < len(s.text) && s.text[s.i] == c {
		s.i++
		if s.nonblank < s.i {
			s.setNonblank()
		}
		return true
	}
	return false
}

func (s *line) skip(n int) {
	s.i += n
	if s.nonblank < s.i {
		s.setNonblank()
	}
}

func (s *line) string() string {
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
	// unreachable
	panic("bad spaces")
}

func trimLeftSpaceTab(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[i:]
}

func trimRightSpaceTab(s string) string {
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpaceTab(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpace(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t') {
		j--
	}
	return s[:j]
}

func trimSpaceTabNewline(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n') {
		i++
	}
	s = s[i:]
	j := len(s)
	for j > 0 && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n') {
		j--
	}
	return s[:j]
}

func (s *line) isBlank() bool {
	return s.nonblank == len(s.text)
}

func (s *line) eof() bool {
	return s.i >= len(s.text)
}

func (s *line) trimSpaceString() string {
	return s.text[s.nonblank:]
}

func (s *line) trimString() string {
	if s.nonblank < s.i {
		panic("bad blank")
	}
	return trimSpaceTab(s.text[s.nonblank:])
}
