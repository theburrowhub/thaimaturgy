package rulesystem

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type formulaLexer struct {
	input string
	pos   int
}

func (l *formulaLexer) peek() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	return l.input[l.pos]
}

func (l *formulaLexer) next() byte {
	if l.pos >= len(l.input) {
		return 0
	}
	ch := l.input[l.pos]
	l.pos++
	return ch
}

func (l *formulaLexer) skipSpace() {
	for l.pos < len(l.input) && unicode.IsSpace(rune(l.input[l.pos])) {
		l.pos++
	}
}

func (l *formulaLexer) readNumber() (float64, error) {
	start := l.pos
	for l.pos < len(l.input) && (unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '.') {
		l.pos++
	}
	val, err := strconv.ParseFloat(l.input[start:l.pos], 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", l.input[start:l.pos])
	}
	return val, nil
}

func (l *formulaLexer) readIdent() string {
	start := l.pos
	for l.pos < len(l.input) && (unicode.IsLetter(rune(l.input[l.pos])) || unicode.IsDigit(rune(l.input[l.pos])) || l.input[l.pos] == '_') {
		l.pos++
	}
	return l.input[start:l.pos]
}

type formulaParser struct {
	lex  *formulaLexer
	vars map[string]float64
}

func (p *formulaParser) parseExpr() (float64, error) {
	return p.parseAddSub()
}

func (p *formulaParser) parseAddSub() (float64, error) {
	left, err := p.parseMulDiv()
	if err != nil {
		return 0, err
	}
	for {
		p.lex.skipSpace()
		switch p.lex.peek() {
		case '+':
			p.lex.next()
			right, err := p.parseMulDiv()
			if err != nil {
				return 0, err
			}
			left += right
		case '-':
			p.lex.next()
			right, err := p.parseMulDiv()
			if err != nil {
				return 0, err
			}
			left -= right
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parseMulDiv() (float64, error) {
	left, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	for {
		p.lex.skipSpace()
		switch p.lex.peek() {
		case '*':
			p.lex.next()
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			left *= right
		case '/':
			p.lex.next()
			right, err := p.parseUnary()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		default:
			return left, nil
		}
	}
}

func (p *formulaParser) parseUnary() (float64, error) {
	p.lex.skipSpace()
	if p.lex.peek() == '-' {
		p.lex.next()
		val, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	if p.lex.peek() == '+' {
		p.lex.next()
	}
	if strings.HasPrefix(p.lex.input[p.lex.pos:], "floor(") {
		p.lex.pos += len("floor(")
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.lex.skipSpace()
		if p.lex.peek() != ')' {
			return 0, fmt.Errorf("expected ')' after floor(")
		}
		p.lex.next()
		return float64(int64(val)), nil
	}
	return p.parsePrimary()
}

func (p *formulaParser) parsePrimary() (float64, error) {
	p.lex.skipSpace()
	ch := p.lex.peek()
	if ch == '(' {
		p.lex.next()
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.lex.skipSpace()
		if p.lex.peek() != ')' {
			return 0, fmt.Errorf("expected ')' at position %d", p.lex.pos)
		}
		p.lex.next()
		return val, nil
	}
	if unicode.IsDigit(rune(ch)) || ch == '.' {
		return p.lex.readNumber()
	}
	if unicode.IsLetter(rune(ch)) || ch == '_' {
		name := p.lex.readIdent()
		if val, ok := p.vars[name]; ok {
			return val, nil
		}
		return 0, fmt.Errorf("unknown variable %q", name)
	}
	return 0, fmt.Errorf("unexpected character %q at position %d", string(ch), p.lex.pos)
}

// EvalFormula evaluates a simple arithmetic expression with variables.
// Supports +, -, *, /, parentheses, integers, floats, and variable names.
func EvalFormula(expr string, vars map[string]float64) (float64, error) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return 0, fmt.Errorf("empty expression")
	}
	if vars == nil {
		vars = map[string]float64{}
	}
	p := &formulaParser{
		lex:  &formulaLexer{input: expr},
		vars: vars,
	}
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	p.lex.skipSpace()
	if p.lex.pos < len(p.lex.input) {
		return 0, fmt.Errorf("unexpected trailing input at position %d", p.lex.pos)
	}
	return val, nil
}
