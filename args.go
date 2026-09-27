package lisan

import (
	"math"
	"reflect"
	"strconv"
	"strings"
)

// maxCountOperand bounds the CLDR integer operand. Counts beyond it are clamped
// so that operand derivation cannot overflow on 32-bit platforms.
const maxCountOperand = math.MaxInt32

// decimalBase is the radix used when trimming trailing fraction digits.
const decimalBase = 10

// Args carries runtime placeholder values. Keys correspond to {{name}}
// placeholders in the translation text; unused keys are ignored, so one map can
// serve several lookups.
type Args map[string]any

// Amount pairs a monetary value with an ISO 4217 currency code, for use with a
// {{key:currency}} placeholder.
type Amount struct {
	// Code is the ISO 4217 currency code, such as "USD".
	Code string
	// Value is the monetary value.
	Value float64
}

// Money builds an Amount for a {{key:currency}} placeholder. The code is
// validated when the translation is rendered, not here.
func Money(value float64, code string) Amount {
	return Amount{Value: value, Code: code}
}

// Number is a count paired with the CLDR plural operands derived from it. Build
// one with N and pass it to TN.
//
// Integer and floating-point values are treated differently on purpose: N(1) is
// the integer 1, while N(1.0) is "1.0" and carries one visible fraction digit.
// CLDR puts those in different categories in several languages.
type Number struct {
	value any
	// digits is CLDR operand i: integer digits of the absolute value.
	digits int
	// visible is CLDR operand v: fraction digit count, with trailing zeros.
	visible int
	// visibleTrimmed is CLDR operand w: fraction digit count, without them.
	visibleTrimmed int
	// frac is CLDR operand f: fraction digits as an integer, with trailing zeros.
	frac int
	// fracTrimmed is CLDR operand t: fraction digits as an integer, without them.
	fracTrimmed int
	// ok reports whether value was a usable number.
	ok bool
}

// N derives the CLDR plural operands for a count. It accepts any built-in Go
// integer or floating-point type; anything else yields a Number that makes TN
// report a miss.
func N(value any) Number {
	if already, isNumber := value.(Number); isNumber {
		return already
	}

	reflected := reflect.ValueOf(value)

	switch reflected.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return integerNumber(value, reflected.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return unsignedNumber(value, reflected.Uint())
	case reflect.Float32, reflect.Float64:
		return floatNumber(value, reflected.Float())
	default:
		return Number{value: value}
	}
}

// integerNumber builds operands for an integer count, which never has a
// visible fraction.
func integerNumber(original any, signed int64) Number {
	magnitude := signed
	if magnitude < 0 {
		magnitude = -magnitude
	}

	// The second test also catches math.MinInt64, whose negation stays negative.
	if magnitude < 0 || magnitude > maxCountOperand {
		magnitude = maxCountOperand
	}

	return Number{value: original, digits: int(magnitude), ok: true}
}

// unsignedNumber builds operands for an unsigned count.
func unsignedNumber(original any, magnitude uint64) Number {
	if magnitude > maxCountOperand {
		magnitude = maxCountOperand
	}

	return Number{value: original, digits: int(magnitude), ok: true}
}

// floatNumber builds operands for a floating-point count. A float always
// carries at least one visible fraction digit, so N(1.0) renders as "1.0".
func floatNumber(original any, value float64) Number {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return Number{value: original}
	}

	text := strconv.FormatFloat(math.Abs(value), 'f', -1, 64)

	whole, fracText, _ := strings.Cut(text, ".")
	if fracText == "" {
		fracText = "0"
	}

	magnitude, err := strconv.ParseInt(whole, decimalBase, 64)
	if err != nil || magnitude > maxCountOperand {
		magnitude = maxCountOperand
	}

	fracValue, err := strconv.Atoi(fracText)
	if err != nil {
		return Number{value: original}
	}

	trimmed := trimFraction(fracValue, len(fracText))

	return Number{
		value:          original,
		digits:         int(magnitude),
		visible:        len(fracText),
		visibleTrimmed: trimmed.digits,
		frac:           fracValue,
		fracTrimmed:    trimmed.value,
		ok:             true,
	}
}

// fraction holds the CLDR w and t operands: a fraction's digit count and value
// once trailing zeros have been removed.
type fraction struct {
	digits int
	value  int
}

// trimFraction strips trailing zeros from a fraction.
func trimFraction(value, digits int) fraction {
	if value == 0 {
		return fraction{}
	}

	for value%decimalBase == 0 {
		value /= decimalBase
		digits--
	}

	return fraction{digits: digits, value: value}
}
