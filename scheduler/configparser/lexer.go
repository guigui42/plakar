package configparser

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/PlakarKorp/plakar/scheduler"
)

const (
	B float64 = 1 << (10 * iota)
	KB
	MB
	GB
	TB
	PB
	EB
	ZB
	YB
)

var sizes = map[string]float64{
	"b":  B,
	"kb": KB,
	"mb": MB,
	"gb": GB,
	"tb": TB,
	"pb": PB,
	"eb": EB,
	"zb": ZB,
	"yb": YB,
}

var durations = map[string]time.Duration{
	"h": time.Hour,
	"m": time.Minute,
	"s": time.Second,

	"hr":  time.Hour,
	"min": time.Minute,
	"sec": time.Second,
}

var tokens = map[string]int{
	"and":         AND,
	"at":          AT,
	"backup":      BACKUP,
	"before":      BEFORE,
	"category":    CATEGORY,
	"check":       CHECK,
	"day":         DAY,
	"environment": ENVIRONMENT,
	"every":       EVERY,
	"exclude":     EXCLUDE,
	"from":        FROM,
	"job":         JOB,
	"latest":      LATEST,
	"maintenance": MAINTENANCE,
	"name":        NAME,
	"off":         OFF,
	"on":          ON,
	"perimeter":   PERIMETER,
	"reference":   REFERENCE,
	"reporting":   REPORTING,
	"restore":     RESTORE,
	"retention":   RETENTION,
	"since":       SINCE,
	"sync":        SYNC,
	"tag":         TAG,
	"to":          TO,
	"until":       UNTIL,
	"with":        WITH,
}

var weekdays = map[string]time.Weekday{
	"monday":    time.Monday,
	"tuesday":   time.Tuesday,
	"wednesday": time.Wednesday,
	"thursday":  time.Thursday,
	"friday":    time.Friday,
	"saturday":  time.Saturday,
	"sunday":    time.Sunday,

	"mon": time.Monday,
	"tue": time.Tuesday,
	"wed": time.Wednesday,
	"thu": time.Thursday,
	"fri": time.Friday,
	"sat": time.Saturday,
	"sun": time.Sunday,
}

var months = map[string]time.Month{
	"january":   time.January,
	"february":  time.February,
	"march":     time.March,
	"april":     time.April,
	"may":       time.May,
	"june":      time.June,
	"july":      time.July,
	"august":    time.August,
	"september": time.September,
	"october":   time.October,
	"november":  time.November,
	"december":  time.December,

	"jan": time.January,
	"feb": time.February,
	"mar": time.March,
	"apr": time.April,
	//"may": time.May,
	"jun": time.June,
	"jul": time.July,
	"aug": time.August,
	"sep": time.September,
	"oct": time.October,
	"nov": time.November,
	"dec": time.December,
}

func (parser *ConfigParser) Error(s string) {
	parser.err = errors.New(s)
}

var (
	CHARS_SPACES     = []byte("\r\n\t ")
	CHARS_SEPARATORS = []byte(",;")

	SUFFIX_TIME     = []string{"am", "pm"}
	SUFFIX_DURATION = []string{"d", "h", "m", "s"}
)

func isSpace(c byte) bool {
	return slices.Contains(CHARS_SPACES, c)
}

func isSeparator(c byte) bool {
	return slices.Contains(CHARS_SEPARATORS, c)
}

func isAlpha(c byte) bool {
	if c >= 'a' && c <= 'z' {
		return true
	}
	if c >= 'A' && c <= 'Z' {
		return true
	}
	return false
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isSpecialChar(c byte) bool {
	return slices.Contains([]byte("\n{}()<>"), c)
}

func (parser *ConfigParser) findLine(pos int) (int, int, string, error) {
	var buf []byte
	var lineno int = 1
	buf = parser.buf
	for {
		i := bytes.IndexByte(buf, '\n')
		if i == -1 {
			return 0, 0, "", fmt.Errorf("Out of bound")
		}

		if i < pos {
			buf = buf[i+1:]
			lineno += 1
			pos -= i + 1
			continue
		}

		return lineno, pos, string(buf[:i]), nil

	}
}

func (parser *ConfigParser) next() byte {
	if parser.pos == -1 {
		buf, err := io.ReadAll(parser.reader)
		if err != nil {
			parser.err = err
			return 0
		}
		parser.buf = buf
		parser.pos = 0
	}

	if len(parser.buf) <= parser.pos {
		parser.pos++
		return 0
	}

	b := parser.buf[parser.pos]
	if b == '\n' {
		parser.lineno++
	}
	parser.pos++
	return b
}

func (parser *ConfigParser) back() {
	parser.pos--
	if parser.pos < len(parser.buf) {
		b := parser.buf[parser.pos]
		if b == '\n' {
			parser.lineno--
		}
	}
}

func (parser *ConfigParser) lexError(lval *yySymType, err error) int {
	parser.lexerr = err
	lval.err = err
	return ERROR
}

func (parser *ConfigParser) Lex(lval *yySymType) int {
	for {
		parser.tokenPos = parser.pos
		b := parser.next()
		switch {
		case b == 0:
			return 0
		case b == '#':
			parser.skipComment()
			continue
		case isSpace(b):
			parser.skipSpace()
			continue
		case isSpecialChar(b):
			return int(b)
		case isSeparator(b):
			return int(b)
		case b == '@':
			return parser.scanReference(lval)
		case isAlpha(b):
			parser.back()
			return parser.scanToken(lval)
		case isDigit(b):
			parser.back()
			return parser.scanNumeric(lval)
		case b == '"':
			return parser.scanString(lval)
		default:
			return parser.lexError(lval, fmt.Errorf("Unexpected char '%c'", b))
		}
	}
}

func (parser *ConfigParser) skipSpace() {
	for isSpace(parser.next()) {
	}
	parser.back()
}

func (parser *ConfigParser) skipComment() {
	for {
		b := parser.next()
		if b == 0 || b == '\n' {
			break
		}
	}
	parser.back()
}

func (parser *ConfigParser) scanReference(lval *yySymType) int {
	var s strings.Builder

	s.WriteByte('@')
	for {
		c := parser.next()
		if !(isDigit(c) || isAlpha(c) || c == '_' || c == '-') {
			break
		}
		s.WriteByte(c)
	}
	parser.back()

	lval.s = s.String()
	return REFERENCE
}

func (parser *ConfigParser) scanToken(lval *yySymType) int {
	var s strings.Builder
	for {
		c := parser.next()
		if !isAlpha(c) {
			break
		}
		s.WriteByte(c)
	}
	parser.back()

	v := s.String()
	t, ok := tokens[v]
	if ok {
		return t
	}
	w, ok := weekdays[v]
	if ok {
		lval.datemask = scheduler.MakeWeekday(w)
		return WEEKDAY
	}
	m, ok := months[v]
	if ok {
		lval.month = m
		return MONTH
	}

	lval.s = v
	return STRING
}

var escapedChars = map[byte]byte{
	'n': '\n',
	't': '\t',
}

func (parser *ConfigParser) scanString(lval *yySymType) int {
	var s strings.Builder
	for {
		b := parser.next()
		if b == '"' {
			break
		}
		if b == '\\' {
			// parse escaped char
			n := parser.next()
			if n == 0 || n == '\n' {
				return parser.lexError(lval, fmt.Errorf("Unexpected char '%c' in string \"%s\"", n, s.String()))
			}
			t, ok := escapedChars[n]
			if ok {
				n = t
			}
			b = n
		}
		s.WriteByte(b)
	}

	lval.s = s.String()
	return STRING
}

func (parser *ConfigParser) scanNumeric(lval *yySymType) int {

	endsNumeric := func(c byte) bool {
		return c == 0 || c == '\n' || isSpace(c) || isSeparator(c)
	}

	allowedInNumericSuffix := func(c byte) bool {
		return isDigit(c) || isAlpha(c) || c == ':'
	}

	var pfx strings.Builder
	for {
		c := parser.next()
		if !isDigit(c) {
			parser.back()
			break
		}
		pfx.WriteByte(c)
	}

	var sfx strings.Builder
	for {
		c := parser.next()
		if endsNumeric(c) {
			parser.back()
			break
		}
		if !allowedInNumericSuffix(c) {
			return parser.lexError(lval, fmt.Errorf("Unexpected char '%c' in numeric token \"%s%s\"", c, pfx.String(), sfx.String()))
		}
		sfx.WriteByte(c)
	}

	head := pfx.String()
	tail := sfx.String()

	if tail == "" {
		i, err := strconv.ParseInt(head, 10, 64)
		if err != nil {
			return parser.lexError(lval, err)
		}
		lval.i = i
		return INTEGER
	}

	// time      4am 4:30pm
	//           4:00 04:00
	//           16:30
	if tail == "am" || tail == "pm" || tail[0] == ':' {
		return parser.parseTime(head, tail, lval)
	}

	// size      4mb 45kb 23tb 45gb 2pb
	//           4mB 45kB 23tB 45gB 2pB
	//           4MB 45KB 23TB 45GB 2PB
	_, ok := sizes[strings.ToLower(tail)]
	if ok {
		return parser.parseSize(head, tail, lval)
	}

	// duration  45min 12sec 2hr
	//           45m 12s 2h
	_, ok = durations[tail]
	if ok {
		return parser.parseDuration(head, tail, lval)
	}
	if slices.Contains(SUFFIX_DURATION, tail) {
		return parser.parseDuration(head, tail, lval)
	}

	return parser.lexError(lval, fmt.Errorf("Invalid numeric token \"%s%s\"", head, tail))
}

func (parser *ConfigParser) parseSize(head string, tail string, lval *yySymType) int {
	v, err := strconv.ParseFloat(head, 64)
	if err != nil {
		return parser.lexError(lval, fmt.Errorf("Error parsing size \"%s%s\": %v", head, tail, err))
	}
	f, _ := sizes[strings.ToLower(tail)]
	lval.size = v * f
	return SIZE
}

func (parser *ConfigParser) parseDuration(head string, tail string, lval *yySymType) int {
	v, err := strconv.ParseInt(head, 10, 64)
	if err != nil {
		return parser.lexError(lval, fmt.Errorf("Error parsing duration \"%s%s\": %v", head, tail, err))
	}
	f, _ := durations[strings.ToLower(tail)]
	lval.duration = time.Duration(v) * f
	return DURATION
}

func (parser *ConfigParser) parseTime(head string, tail string, lval *yySymType) int {
	hh := head
	mm := tail

	var isAM, isPM bool
	if strings.HasSuffix(mm, "am") {
		isAM = true
		mm = mm[:len(mm)-2]
	} else if strings.HasSuffix(mm, "pm") {
		isPM = true
		mm = mm[:len(mm)-2]
	}

	var hour, minute int
	var h uint64
	var err error

	if strings.HasPrefix(mm, ":") {
		mm = mm[1:]
		if len(mm) != 2 {
			goto fail
		}
		m, err := strconv.ParseUint(mm, 10, 32)
		if err != nil {
			goto fail
		}
		if m >= 60 {
			goto fail
		}
		minute = int(m)
	}

	h, err = strconv.ParseUint(hh, 10, 32)
	if err != nil {
		goto fail
	}

	switch {
	case isAM:
		if h < 1 || h > 12 {
			goto fail
		}
		if h == 12 {
			h = 0
		}
	case isPM:
		if h < 1 || h > 12 {
			goto fail
		}
		if h != 12 {
			h += 12
		}
	default:
		if h > 23 {
			goto fail
		}
	}
	hour = int(h)

	lval.time = scheduler.TimeFromHourMinSec(hour, minute, 0)
	return TIME

fail:
	return parser.lexError(lval, fmt.Errorf("Invalid time \"%s%s\"", head, tail))
}
