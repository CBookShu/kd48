package jsongen

import (
	"github.com/CBookShu/kd48/tools/config-loader/internal/csvparser"
)

type Payload struct {
	ConfigName string           `json:"config_name"`
	Revision   int64            `json:"revision"`
	Data       []map[string]any `json:"data"`
}

type Generator struct{}

func NewGenerator() *Generator {
	return &Generator{}
}

func (g *Generator) Generate(sheet *csvparser.Sheet, revision int64) (*Payload, error) {
	data := make([]map[string]any, len(sheet.Rows))

	for i, row := range sheet.Rows {
		rowData := make(map[string]any)
		for j, value := range row.Values {
			if j < len(sheet.Headers) {
				rowData[sheet.Headers[j].Name] = value.Parsed
			}
		}
		data[i] = rowData
	}

	return &Payload{
		ConfigName: sheet.ConfigName,
		Revision:   revision,
		Data:       data,
	}, nil
}
