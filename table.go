package migu

// Table is table definitions
type Table struct {
	Columns []*columnSchema
	Indexes []*Index
}

func (t Table) HasDatetimeColumn() bool {
	for _, column := range t.Columns {
		if column.DataType == "datetime" {
			return true
		}
	}
	return false
}
