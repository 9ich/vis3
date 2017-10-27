%{

package main

import (
	"bufio"
	"io"
	"log"
	"strconv"
	"unicode"
)

%}

%union{
	fs []float32
	s string
	f float32
}

%type <f> expr
%type <s> sym
%type <fs> exprlist

%token <f> NUM
%token <s> WORD


%%


stmtlist
	: // empty
	| stmtlist stmt
	;

stmt
	: sym exprlist ';'
	{
		lock.Lock()
		addCmd($1, $2)
		lock.Unlock()
	}
	| error ';'
	;

sym
	: WORD
	{
		$$ = $1
	}
	;

exprlist
	: // empty
	{
		$$ = []float32{}
	}
	| exprlist expr
	{
		$$ = append($1, $2)
	}
	;

expr
	: NUM
	{
		$$ = $1
	}
	;


%%


// yyLexer
type lexer struct {
	r *bufio.Reader
	c rune
}

func newLexer(r io.Reader) *lexer {
	l := new(lexer)
	l.r = bufio.NewReader(r)
	return l
}

func (l *lexer) Lex(lval *yySymType) int {
	var err error
	lval.s = ""
	for l.c == 0 || unicode.IsSpace(l.c) {
		l.c, _, err = l.r.ReadRune()
		if err != nil {
			return 0
		}
	}

	if l.c == '-' || l.c == '.' || unicode.IsDigit(l.c) {
		for l.c == '-' || l.c == '.' || (l.c >= '0' && l.c <= '9') {
			lval.s += string(l.c)
			l.c, _, err = l.r.ReadRune()
			if err != nil {
				return 0
			}
		}
		f, _ := strconv.ParseFloat(lval.s, 32)
		lval.f = float32(f)
		return NUM
	} else if unicode.IsLetter(l.c) {
		for unicode.IsLetter(l.c) || unicode.IsDigit(l.c) {
			lval.s += string(l.c)
			l.c, _, err = l.r.ReadRune()
			if err != nil {
				return 0
			}
		}
		return WORD
	} else {
		c := l.c
		l.c, _, _ = l.r.ReadRune()
		return int(c)
	}
}

func (l *lexer) Error(s string) {
	log.Print(s)
}
