package mysqlwriter

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/CBookShu/kd48/tools/config-loader/internal/jsongen"
)

func TestWriter_Write(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	payload := &jsongen.Payload{
		ConfigName: "TestConfig",
		Revision:   1,
		Data:       []map[string]any{{"note": "test"}},
	}

	mock.ExpectExec("INSERT INTO lobby_config_revision").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	w := NewWriter(db)
	err = w.Write(payload, WriteOptions{
		Scope:   "test",
		Title:   "",
		Tags:    "",
		CSVText: "test",
	})
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations not met: %v", err)
	}
}
