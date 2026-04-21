package errors

import "fmt"

type ErrorCode int

const (
	ErrInvalidCSV ErrorCode = iota
	ErrInvalidHeader
	ErrInvalidTypeName
	ErrInvalidValue
	ErrTimeEmpty
	ErrDuplicateColumn
	ErrDuplicateKey
	ErrMySQLWrite
	ErrRedisPublish
	ErrGoGenerate
)

type Error struct {
	Code    ErrorCode
	Message string
	Line    int
	Column  int
	Raw     string
	Meta    map[string]interface{}
}

func (e *Error) Error() string {
	if e.Line >= 0 && e.Column >= 0 {
		return fmt.Sprintf("[%d:%d] %s: %s", e.Line, e.Column, e.Message, e.Raw)
	}
	return fmt.Sprintf("%s: %s", e.Message, e.Raw)
}

func New(code ErrorCode, message string, line, column int, raw string) *Error {
	return &Error{
		Code:    code,
		Message: message,
		Line:    line,
		Column:  column,
		Raw:     raw,
	}
}

func (e *Error) WithMeta(key string, value interface{}) *Error {
	if e.Meta == nil {
		e.Meta = make(map[string]interface{})
	}
	e.Meta[key] = value
	return e
}
