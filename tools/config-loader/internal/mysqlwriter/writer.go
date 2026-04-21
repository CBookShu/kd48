package mysqlwriter

import (
	"database/sql"
	"encoding/json"

	"github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
)

type WriteOptions struct {
	Scope   string
	Title   string
	Tags    string
	CSVText string
}

type Writer struct {
	db *sql.DB
}

func NewWriter(db *sql.DB) *Writer {
	return &Writer{db: db}
}

func (w *Writer) Write(payload *jsongen.Payload, opts WriteOptions) error {
	jsonBytes, err := json.Marshal(payload.Data)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO lobby_config_revision
		(config_name, revision, scope, title, tags, csv_text, json_payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NOW())`

	_, err = w.db.Exec(query,
		payload.ConfigName,
		payload.Revision,
		opts.Scope,
		opts.Title,
		opts.Tags,
		opts.CSVText,
		jsonBytes,
	)
	return err
}
