package csvparser

import (
	"encoding/csv"
	"io"
	"path/filepath"
	"strings"
)

type Parser struct{}

func NewParser() *Parser {
	return &Parser{}
}

func (p *Parser) Parse(r io.Reader, filename string) (*Sheet, error) {
	reader := csv.NewReader(r)
	reader.LazyQuotes = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	if len(records) < 3 {
		return nil, &ParseError{Message: "CSV must have at least 3 header rows"}
	}

	numCols := len(records[0])
	headers := make([]ColumnHeader, numCols)
	for i := 0; i < numCols; i++ {
		headers[i] = ColumnHeader{
			Description: records[0][i],
			Name:        records[1][i],
			Type:        records[2][i],
		}
	}

	rows := make([]Row, 0, len(records)-3)
	for i := 3; i < len(records); i++ {
		values := make([]Value, len(records[i]))
		for j, raw := range records[i] {
			typ := ""
			if j < numCols {
				typ = headers[j].Type
			}
			// Call ParseValue to properly populate the Parsed field
			parsedValue, err := ParseValue(raw, typ)
			if err != nil {
				return nil, &ParseError{
					Message: err.Error(),
				}
			}
			values[j] = parsedValue
		}
		rows = append(rows, Row{Values: values})
	}

	return &Sheet{
		Headers:    headers,
		Rows:       rows,
		ConfigName: deriveConfigName(filename),
		SourceFile: filename,
	}, nil
}

func deriveConfigName(filename string) string {
	base := filepath.Base(filename)
	name := strings.TrimSuffix(base, ".csv")
	parts := strings.Split(name, "--")
	return parts[0]
}

type ParseError struct {
	Message string
}

func (e *ParseError) Error() string {
	return e.Message
}
