package csvparser

// Sheet 表示解析后的 CSV 表
type Sheet struct {
	Headers    []ColumnHeader
	Rows       []Row
	ConfigName string
	SourceFile string
}

// ColumnHeader 三行头信息
type ColumnHeader struct {
	Description string
	Name        string
	Type        string
}

// Row 单行数据
type Row struct {
	Values []Value
}

// Value 类型化的单元格值
type Value struct {
	Raw     string
	Parsed  interface{}
	Type    string
	IsEmpty bool
}
