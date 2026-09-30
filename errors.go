package lisan

import (
	"errors"
	"fmt"
	"strings"
)

// ErrNotConfigured is returned by the package-level API when Configure has not
// completed successfully.
var ErrNotConfigured = errors.New("lisan: not configured, call Configure first")

// Problem describes a single defect found while compiling a translation tree.
type Problem struct {
	// File is the offending file's path relative to the tree root.
	File string
	// ID is the translation identifier involved, empty when not tied to one.
	ID string
	// Message explains the defect.
	Message string
	// Line is the 1-based line within File, or zero when unknown.
	Line int
	// Column is the 1-based column within File, or zero when unknown.
	Column int
}

// String renders the problem as `file:line:col: id "x" message`.
func (p Problem) String() string {
	where := p.File
	if p.Line > 0 {
		where = fmt.Sprintf("%s:%d:%d", p.File, p.Line, p.Column)
	}

	if p.ID == "" {
		return where + ": " + p.Message
	}

	return fmt.Sprintf("%s: id %q %s", where, p.ID, p.Message)
}

// CompileError reports every defect found in a translation tree. Compilation
// does not stop at the first problem, so Problems holds all of them.
type CompileError struct {
	// Root names the tree that failed to compile.
	Root string
	// Problems lists every defect found, in file order.
	Problems []Problem
}

// Error implements error.
func (e *CompileError) Error() string {
	if len(e.Problems) == 1 {
		return "lisan: compile error: " + e.Problems[0].String()
	}

	root := e.Root
	if root == "" {
		root = "the translation tree"
	}

	lines := make([]string, 0, len(e.Problems)+1)
	lines = append(lines, fmt.Sprintf("lisan: compile error: %d problems in %s", len(e.Problems), root))

	for index, problem := range e.Problems {
		lines = append(lines, fmt.Sprintf("  %d. %s", index+1, problem.String()))
	}

	return strings.Join(lines, "\n")
}

// MissingTranslationError reports a lookup that could not be rendered. After a
// successful compile this can only be an unknown ID or an argument whose type
// does not match its format directive.
type MissingTranslationError struct {
	// ID is the identifier that was requested.
	ID string
	// Locale is the locale the lookup was made against.
	Locale string
	// Reason explains what went wrong.
	Reason string
}

// Error implements error.
func (e *MissingTranslationError) Error() string {
	return fmt.Sprintf("lisan: %s (id %q, locale %q)", e.Reason, e.ID, e.Locale)
}

// MissingEvent is handed to the WithOnMissing hook on every failed lookup.
type MissingEvent struct {
	// Err is the underlying *MissingTranslationError.
	Err error
	// ID is the identifier that was requested.
	ID string
	// Locale is the locale the lookup was made against.
	Locale string
}

// missingf builds a miss for id at locale and reports it to the hook.
func missingf(hook func(MissingEvent), id, locale, format string, args ...any) error {
	err := &MissingTranslationError{
		ID:     id,
		Locale: locale,
		Reason: fmt.Sprintf(format, args...),
	}

	if hook != nil {
		hook(MissingEvent{ID: id, Locale: locale, Err: err})
	}

	return err
}
