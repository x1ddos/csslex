// Copyright 2015 Google Inc. All Rights Reserved.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//     http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package csslex provides a simple CSS lexer, without using regexp.
package csslex

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// ItemType specifies type of Item.
type ItemType int

// Item types emitted by the lexer.
const (
	ItemError            ItemType = iota // Parsing error. Lex stops at first error.
	ItemSelector                         // CSS selector.
	ItemDecl                             // CSS declaration in a block.
	ItemBlockStart                       // Beginning of a regular CSS block, not inside At-Rule.
	ItemBlockEnd                         // Ending of a regular CSS block, not inside At-Rule.
	ItemAtRuleIdent                      // At-Rule identifier, including @ symbol.
	ItemAtRule                           // The content of an At-Rule.
	ItemAtRuleBlockStart                 // Beginning of an At-Rule block.
	ItemAtRuleBlockEnd                   // Ending of an At-Rule block.
)

const eof = -1

// Item is an atom of lexing process.
type Item struct {
	Typ ItemType
	Pos int
	Val string
}

// Lex creates a new lexer and returns channel which will be sent Item tokens.
// The lexing is started in a goroutine right away, before returing
// from this method.
func Lex(input string) chan *Item {
	l := &lexer{
		input: input,
		items: make(chan *Item),
	}
	go l.run()
	return l.items
}

// lexer is the parser state.
type lexer struct {
	input              string
	start, pos         int
	inBlock, inAtBlock bool
	state              stateFn
	items              chan *Item
}

// run lexes the input by executing state functions until the state is nil.
func (l *lexer) run() {
	for state := lexAny; state != nil; {
		prev := state
		state = state(l)
		l.state = prev
	}
	close(l.items)
}

// emit passes an item back to the client.
func (l *lexer) emit(t ItemType) {
	i := &Item{t, l.start, strings.Trim(l.input[l.start:l.pos], spaceChars)}
	l.items <- i
	l.start = l.pos
}

// next returns the next rune in the input.
func (l *lexer) next() rune {
	if int(l.pos) >= len(l.input) {
		return eof
	}
	r, w := utf8.DecodeRuneInString(l.input[l.pos:])
	l.pos += w
	return r
}

// backup steps back one rune. Can only be called once per call of next.
func (l *lexer) backup() {
	l.pos--
}

// ignore skips over the pending input before this point
func (l *lexer) ignore() {
	l.start = l.pos
}

// ignoreSpace consumes a run of runes from spaceChars.
func (l *lexer) ignoreSpace() {
	for strings.IndexRune(spaceChars, l.next()) >= 0 {
	}
	l.backup()
	l.ignore()
}

// untilRun consumes runes until one of the chars is encountered.
func (l *lexer) untilRun(chars string) rune {
	var r rune
	for r != eof && strings.IndexRune(chars, r) < 0 {
		r = l.next()
	}
	l.backup()
	return r
}

// errorf returns an error token and terminates the scan by passing
// back a nil pointer that will be the next state, terminating l.nextItem.
func (l *lexer) errorf(format string, args ...interface{}) stateFn {
	l.items <- &Item{ItemError, l.start, fmt.Sprintf(format, args...)}
	return nil
}

const (
	openComment  = "/*"
	closeComment = "*/"
	atRuleStart  = '@'
	selectorSep  = ','
	ruleSep      = ':'
	openBlock    = '{'
	closeBlock   = '}'
	spaceChars   = " \t\n\r"
)

type stateFn func(*lexer) stateFn

// lexAny is the starting point of lexing.
func lexAny(l *lexer) stateFn {
	for {
		r := l.next()
		if r == eof {
			return nil
		}
		if strings.ContainsRune(spaceChars, r) {
			l.ignore()
			continue
		}
		if r == closeBlock && l.inAtBlock && !l.inBlock {
			l.inAtBlock = false
			l.ignore()
			l.emit(ItemAtRuleBlockEnd)
			continue
		}
		l.backup()
		if strings.HasPrefix(l.input[l.pos:], openComment) {
			return lexComment
		}
		if r == atRuleStart {
			return lexAtRuleIdent
		}
		return lexSelector
	}
}

// lexComment parsers CSS comments.
func lexComment(l *lexer) stateFn {
	l.pos += len(openComment)
	i := strings.Index(l.input[l.pos:], closeComment)
	if i < 0 {
		return l.errorf("unclosed comment")
	}
	l.pos += i + len(closeComment)
	l.ignore()
	return l.state
}

// lexSelector parses CSS selectors. It emits each one separately,
// even if they describe the same block, i.e. separated by a comma.
func lexSelector(l *lexer) stateFn {
	r := l.untilRun(",{")
	if r == eof {
		return nil
	}
	defer func() {
		l.next()
		l.ignoreSpace()
	}()
	l.emit(ItemSelector)
	if r == selectorSep {
		return lexSelector
	}
	l.inBlock = true
	l.emit(ItemBlockStart)
	return lexBlock
}

// lexBlock parses CSS blocks found in curly braces.
func lexBlock(l *lexer) stateFn {
	r := l.untilRun(";}")
	if r == eof {
		return l.errorf("unclosed block")
	}
	defer func() {
		l.next()
		l.ignoreSpace()
	}()
	if strings.ContainsRune(l.input[l.start:l.pos], ruleSep) {
		l.emit(ItemDecl)
	}
	if r == closeBlock {
		l.inBlock = false
		l.emit(ItemBlockEnd)
		return lexAny
	}
	return lexBlock
}

// lexAtRuleIdent parses beginning of CSS At-Rule, which starts with '@' char.
func lexAtRuleIdent(l *lexer) stateFn {
	i := strings.IndexRune(l.input[l.pos:], ' ')
	if i < 1 {
		return l.errorf("missing at-rule ident")
	}
	l.pos += i
	l.emit(ItemAtRuleIdent)
	l.ignoreSpace()
	return lexAtRule
}

// lexAtRule parses whatever follows after an At-Rule identifier.
func lexAtRule(l *lexer) stateFn {
	r := l.untilRun(";{")
	if r == eof {
		return l.errorf("missing at-rule body")
	}
	defer func() {
		l.next()
		l.ignoreSpace()
	}()
	if r == openBlock {
		l.inAtBlock = true
		l.emit(ItemAtRuleBlockStart)
		return lexSelector
	}
	l.emit(ItemAtRule)
	return lexAny
}
