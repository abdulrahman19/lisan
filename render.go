package lisan

import (
	"errors"
	"fmt"

	"golang.org/x/text/message"
)

// errCountOutsidePlural guards {{count}} appearing where no count exists. The
// compiler rejects this, so reaching it at runtime would be a bug.
var errCountOutsidePlural = errors.New("uses {{count}} outside a plural entry")

// renderContext carries everything a phrase needs to produce final text.
type renderContext struct {
	printer  *message.Printer
	args     Args
	count    Number
	hasCount bool
}

// render fills in every placeholder and returns the finished string.
func (p phrase) render(ctx renderContext) (string, error) {
	out := make([]byte, 0, p.width)

	for _, seg := range p.segments {
		text, err := seg.resolve(ctx)
		if err != nil {
			return "", err
		}

		out = append(out, text...)
	}

	return string(out), nil
}

// resolve renders a single segment.
func (s segment) resolve(ctx renderContext) (string, error) {
	switch s.kind {
	case segmentLiteral:
		return s.text, nil
	case segmentCount:
		if !ctx.hasCount {
			return "", errCountOutsidePlural
		}

		return formatCount(ctx.printer, ctx.count, s.directive)
	case segmentArg:
		value, provided := ctx.args[s.text]
		if !provided {
			return "", fmt.Errorf("is missing a value for placeholder %s%s%s", openDelim, s.text, closeDelim)
		}

		return s.directive.apply(ctx.printer, value)
	default:
		return "", errUnsupportedDirective
	}
}
