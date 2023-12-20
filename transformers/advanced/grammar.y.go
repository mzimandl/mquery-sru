// Code generated by goyacc - DO NOT EDIT.

package advanced

import __yyfmt__ "fmt"

type yySymType struct {
	yys    int
	String string
	Node   node
}

type yyXError struct {
	state, xsym int
}

const (
	yyDefault = 57351
	yyEofCode = 57344
	AND       = 57348
	NOT       = 57347
	OR        = 57349
	PROX      = 57350
	TERM      = 57346
	yyErrCode = 57345

	yyMaxDepth = 200
	yyTabOfs   = -8
)

var (
	yyPrec = map[int]int{
		AND:  0,
		OR:   0,
		PROX: 0,
		NOT:  0,
	}

	yyXLAT = map[int]int{
		57344: 0,  // $end (8x)
		57348: 1,  // AND (8x)
		57349: 2,  // OR (8x)
		57350: 3,  // PROX (8x)
		41:    4,  // ')' (7x)
		40:    5,  // '(' (6x)
		57352: 6,  // node (6x)
		57347: 7,  // NOT (6x)
		57346: 8,  // TERM (6x)
		57353: 9,  // start (1x)
		57351: 10, // $default (0x)
		57345: 11, // error (0x)
	}

	yySymNames = []string{
		"$end",
		"AND",
		"OR",
		"PROX",
		"')'",
		"'('",
		"node",
		"NOT",
		"TERM",
		"start",
		"$default",
		"error",
	}

	yyTokenLiteralStrings = map[int]string{}

	yyReductions = map[int]struct{ xsym, components int }{
		0: {0, 1},
		1: {9, 1},
		2: {6, 1},
		3: {6, 3},
		4: {6, 2},
		5: {6, 3},
		6: {6, 3},
		7: {6, 3},
	}

	yyXErrors = map[yyXError]string{}

	yyParseTab = [15][]uint8{
		// 0
		{5: 12, 10, 13, 11, 9},
		{8},
		{7, 15, 16, 17},
		{6, 6, 6, 6, 6},
		{5: 12, 21, 13, 11},
		// 5
		{5: 12, 14, 13, 11},
		{4, 4, 4, 4, 4},
		{5: 12, 20, 13, 11},
		{5: 12, 19, 13, 11},
		{5: 12, 18, 13, 11},
		// 10
		{1, 1, 1, 1, 1},
		{2, 2, 2, 2, 2},
		{3, 3, 3, 3, 3},
		{1: 15, 16, 17, 22},
		{5, 5, 5, 5, 5},
	}
)

var yyDebug = 0

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyLexerEx interface {
	yyLexer
	Reduced(rule, state int, lval *yySymType) bool
}

func yySymName(c int) (s string) {
	x, ok := yyXLAT[c]
	if ok {
		return yySymNames[x]
	}

	if c < 0x7f {
		return __yyfmt__.Sprintf("%q", c)
	}

	return __yyfmt__.Sprintf("%d", c)
}

func yylex1(yylex yyLexer, lval *yySymType) (n int) {
	n = yylex.Lex(lval)
	if n <= 0 {
		n = yyEofCode
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("\nlex %s(%#x %d), lval: %+v\n", yySymName(n), n, n, lval)
	}
	return n
}

func yyParse(yylex yyLexer) int {
	const yyError = 11

	yyEx, _ := yylex.(yyLexerEx)
	var yyn int
	var yylval yySymType
	var yyVAL yySymType
	yyS := make([]yySymType, 200)

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yyerrok := func() {
		if yyDebug >= 2 {
			__yyfmt__.Printf("yyerrok()\n")
		}
		Errflag = 0
	}
	_ = yyerrok
	yystate := 0
	yychar := -1
	var yyxchar int
	var yyshift int
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	if yychar < 0 {
		yylval.yys = yystate
		yychar = yylex1(yylex, &yylval)
		var ok bool
		if yyxchar, ok = yyXLAT[yychar]; !ok {
			yyxchar = len(yySymNames) // > tab width
		}
	}
	if yyDebug >= 4 {
		var a []int
		for _, v := range yyS[:yyp+1] {
			a = append(a, v.yys)
		}
		__yyfmt__.Printf("state stack %v\n", a)
	}
	row := yyParseTab[yystate]
	yyn = 0
	if yyxchar < len(row) {
		if yyn = int(row[yyxchar]); yyn != 0 {
			yyn += yyTabOfs
		}
	}
	switch {
	case yyn > 0: // shift
		yychar = -1
		yyVAL = yylval
		yystate = yyn
		yyshift = yyn
		if yyDebug >= 2 {
			__yyfmt__.Printf("shift, and goto state %d\n", yystate)
		}
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	case yyn < 0: // reduce
	case yystate == 1: // accept
		if yyDebug >= 2 {
			__yyfmt__.Println("accept")
		}
		goto ret0
	}

	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			if yyDebug >= 1 {
				__yyfmt__.Printf("no action for %s in state %d\n", yySymName(yychar), yystate)
			}
			msg, ok := yyXErrors[yyXError{yystate, yyxchar}]
			if !ok {
				msg, ok = yyXErrors[yyXError{yystate, -1}]
			}
			if !ok && yyshift != 0 {
				msg, ok = yyXErrors[yyXError{yyshift, yyxchar}]
			}
			if !ok {
				msg, ok = yyXErrors[yyXError{yyshift, -1}]
			}
			if yychar > 0 {
				ls := yyTokenLiteralStrings[yychar]
				if ls == "" {
					ls = yySymName(yychar)
				}
				if ls != "" {
					switch {
					case msg == "":
						msg = __yyfmt__.Sprintf("unexpected %s", ls)
					default:
						msg = __yyfmt__.Sprintf("unexpected %s, %s", ls, msg)
					}
				}
			}
			if msg == "" {
				msg = "syntax error"
			}
			yylex.Error(msg)
			Nerrs++
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				row := yyParseTab[yyS[yyp].yys]
				if yyError < len(row) {
					yyn = int(row[yyError]) + yyTabOfs
					if yyn > 0 { // hit
						if yyDebug >= 2 {
							__yyfmt__.Printf("error recovery found error shift in state %d\n", yyS[yyp].yys)
						}
						yystate = yyn /* simulate a shift of "error" */
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery failed\n")
			}
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yySymName(yychar))
			}
			if yychar == yyEofCode {
				goto ret1
			}

			yychar = -1
			goto yynewstate /* try again in the same state */
		}
	}

	r := -yyn
	x0 := yyReductions[r]
	x, n := x0.xsym, x0.components
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= n
	if yyp+1 >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	exState := yystate
	yystate = int(yyParseTab[yyS[yyp].yys][x]) + yyTabOfs
	/* reduction by production r */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce using rule %v (%s), and goto state %d\n", r, yySymNames[x], yystate)
	}

	switch r {
	case 1:
		{
			yylex.(*basicTransformer).parseResult = &rootNode{yyS[yypt-0].Node}
		}
	case 2:
		{
			yyVAL.Node = &termNode{yyS[yypt-0].String}
		}
	case 3:
		{
			yyVAL.Node = &parenNode{yyS[yypt-1].Node}
		}
	case 4:
		{
			yyVAL.Node = &unaryNode{Op: "NOT", Child: yyS[yypt-0].Node}
		}
	case 5:
		{
			yyVAL.Node = &binaryNode{Op: "AND", Left: yyS[yypt-2].Node, Right: yyS[yypt-0].Node}
		}
	case 6:
		{
			yyVAL.Node = &binaryNode{Op: "OR", Left: yyS[yypt-2].Node, Right: yyS[yypt-0].Node}
		}
	case 7:
		{
			yyVAL.Node = &binaryNode{Op: "PROX", Left: yyS[yypt-2].Node, Right: yyS[yypt-0].Node}
		}

	}

	if yyEx != nil && yyEx.Reduced(r, exState, &yyVAL) {
		return -1
	}
	goto yystack /* stack new state and value */
}
