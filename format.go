package lisan

import (
	"errors"
	"fmt"
	"reflect"

	"golang.org/x/text/currency"
	"golang.org/x/text/message"
	"golang.org/x/text/number"
)

// errUnsupportedDirective guards against a directive kind that was parsed but
// has no renderer, which would be a bug in this package.
var errUnsupportedDirective = errors.New("unsupported format directive")

// defaultPercentFractionDigits caps the fraction digits of an unscaled percent.
// The CLDR percent pattern carries none, which would silently round 0.075 to
// 7%; losing the caller's data by default is the wrong trade for this package.
const defaultPercentFractionDigits = 3

// apply renders one argument value through this directive for the printer's
// locale.
func (d directive) apply(printer *message.Printer, value any) (string, error) {
	switch d.kind {
	case directiveNone:
		return plainText(value), nil
	case directiveNumber:
		return d.formatDecimal(printer, value)
	case directivePercent:
		return d.formatPercent(printer, value)
	case directiveCurrency:
		return formatCurrency(printer, value)
	default:
		return "", errUnsupportedDirective
	}
}

// formatDecimal renders a number with locale digits, grouping and separators.
func (d directive) formatDecimal(printer *message.Printer, value any) (string, error) {
	if !isNumeric(value) {
		return "", fmt.Errorf("expects a number for :number but got %T", value)
	}

	return printer.Sprint(number.Decimal(value, d.options()...)), nil
}

// formatPercent renders a ratio as a locale-formatted percentage, so 0.075
// becomes 7.5% in en-US.
func (d directive) formatPercent(printer *message.Printer, value any) (string, error) {
	if !isNumeric(value) {
		return "", fmt.Errorf("expects a number for :percent but got %T", value)
	}

	return printer.Sprint(number.Percent(value, d.options()...)), nil
}

// options translates a directive's scale into fraction-digit bounds.
func (d directive) options() []number.Option {
	if d.hasScale {
		return []number.Option{
			number.MinFractionDigits(d.scale),
			number.MaxFractionDigits(d.scale),
		}
	}

	if d.kind == directivePercent {
		return []number.Option{number.MaxFractionDigits(defaultPercentFractionDigits)}
	}

	return nil
}

// formatCurrency renders an Amount using the locale's currency conventions.
func formatCurrency(printer *message.Printer, value any) (string, error) {
	amount, isAmount := value.(Amount)
	if !isAmount {
		return "", fmt.Errorf("expects lisan.Money for :currency but got %T", value)
	}

	unit, err := currency.ParseISO(amount.Code)
	if err != nil {
		return "", fmt.Errorf("has unknown ISO 4217 currency code %q", amount.Code)
	}

	return printer.Sprint(currency.Symbol(unit.Amount(amount.Value))), nil
}

// formatCount renders the implicit {{count}} placeholder, preserving the
// fraction digits the caller's value carried.
func formatCount(printer *message.Printer, count Number, d directive) (string, error) {
	if !count.ok {
		return "", fmt.Errorf("was given a count that is not a number: %T", count.value)
	}

	if d.kind != directiveNone {
		return d.apply(printer, count.value)
	}

	return printer.Sprint(number.Decimal(count.value,
		number.MinFractionDigits(count.visible),
		number.MaxFractionDigits(count.visible),
	)), nil
}

// plainText renders an undirected placeholder value.
func plainText(value any) string {
	if text, isString := value.(string); isString {
		return text
	}

	return fmt.Sprint(value)
}

// isNumeric reports whether value is a built-in Go numeric type.
func isNumeric(value any) bool {
	switch reflect.ValueOf(value).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}
