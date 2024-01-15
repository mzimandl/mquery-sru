// Copyright 2023 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2023 Institute of the Czech National Corpus,
//                Faculty of Arts, Charles University
//   This file is part of MQUERY.
//
//  MQUERY is free software: you can redistribute it and/or modify
//  it under the terms of the GNU General Public License as published by
//  the Free Software Foundation, either version 3 of the License, or
//  (at your option) any later version.
//
//  MQUERY is distributed in the hope that it will be useful,
//  but WITHOUT ANY WARRANTY; without even the implied warranty of
//  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//  GNU General Public License for more details.
//
//  You should have received a copy of the GNU General Public License
//  along with MQUERY.  If not, see <https://www.gnu.org/licenses/>.

package basic

import (
	"errors"
	"regexp"
	"strings"
)

const EOF = 0

type basicTransformer struct {
	input       string
	parseResult node
	defaultAttr string
	errors      []error
}

// TODO
func (t *basicTransformer) TranslateWithinCtx(v string) string {
	switch v {
	case "sentence", "s":
		return "s"
	case "utterance", "u":
		return "u"
	case "paragraph", "p":
		return "p"
	case "turn", "t":
		return "t"
	case "text":
		return "doc"
	case "session":
		return "session"
	}
	return "??"
}

func (t *basicTransformer) AddError(err error) {
	t.errors = append(t.errors, err)
}

func (t *basicTransformer) Errors() []error {
	return t.errors
}

func (t *basicTransformer) Error(e string) {
	t.AddError(errors.New(e))
}

type tokenDef struct {
	regex *regexp.Regexp
	token int
}

var tokens = []tokenDef{
	{
		regex: regexp.MustCompile(`^NOT`),
		token: NOT,
	},
	{
		regex: regexp.MustCompile(`^AND`),
		token: AND,
	},
	{
		regex: regexp.MustCompile(`^OR`),
		token: OR,
	},
	{
		regex: regexp.MustCompile(`^PROX`),
		token: PROX,
	},
	{
		regex: regexp.MustCompile(`^\".*\"`),
		token: TERM,
	},
	{
		regex: regexp.MustCompile(`^[\S]*`),
		token: TERM,
	},
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n'
}

func (t *basicTransformer) Lex(lval *yySymType) int {
	// Skip spaces.
	for ; len(t.input) > 0 && isSpace(t.input[0]); t.input = t.input[1:] {
	}

	// Check if the input has ended.
	if len(t.input) == 0 {
		return EOF
	}

	// Check if one of the regular expressions matches.
	for _, tokDef := range tokens {
		str := tokDef.regex.FindString(t.input)
		if str != "" {
			t.input = t.input[len(str):]
			// Pass string content to the parser.
			switch tokDef.token {
			case TERM:
				lval.String = strings.Trim(str, "\"")
			default:
				lval.String = str
			}
			return tokDef.token
		}
	}

	// Otherwise return the next letter.
	ret := int(t.input[0])
	t.input = t.input[1:]
	return ret
}

func (t *basicTransformer) Generate() string {
	ans, err := t.parseResult.transform(t.defaultAttr)
	if err != nil {
		t.errors = append(t.errors, err)
	}
	return ans
}

func NewBasicTransformer(input string, defaultAttr string) (*basicTransformer, error) {
	t := &basicTransformer{input: input}
	yyParse(t)
	if len(t.errors) > 0 {
		return nil, t.errors[0]
	}
	return t, nil
}
