package errors

import (
    "testing"
)

func TestError_Error_WithLineColumn(t *testing.T) {
    e := &Error{
        Code:    ErrInvalidValue,
        Message: "invalid int32 value",
        Line:    4,
        Column:  2,
        Raw:     "abc",
    }
    got := e.Error()
    want := "[4:2] invalid int32 value: abc"
    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}

func TestError_Error_WithoutLineColumn(t *testing.T) {
    e := &Error{
        Code:    ErrInvalidCSV,
        Message: "CSV format error",
        Line:    -1,
        Column:  -1,
        Raw:     "bad",
    }
    got := e.Error()
    want := "CSV format error: bad"
    if got != want {
        t.Errorf("Error() = %q, want %q", got, want)
    }
}
