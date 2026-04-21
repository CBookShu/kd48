package validator

import (
	"regexp"

	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
	"github.com/CBookShu/kd48/tools/config-loader/internal/errors"
)

var snakeCaseRegex = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

type Validator struct{}

func NewValidator() *Validator {
	return &Validator{}
}

func (v *Validator) Validate(sheet *csvparser.Sheet) error {
	seen := make(map[string]int)
	for i, h := range sheet.Headers {
		if !snakeCaseRegex.MatchString(h.Name) {
			return errors.New(errors.ErrInvalidHeader,
				"column name must be snake_case",
				1, i, h.Name)
		}
		if prev, exists := seen[h.Name]; exists {
			return errors.New(errors.ErrDuplicateColumn,
				"duplicate column name",
				1, i, h.Name).WithMeta("previous_column", prev)
		}
		seen[h.Name] = i
	}
	return nil
}
