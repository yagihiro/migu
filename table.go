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

func (t Table) ColumnMap() map[string]*columnSchema {
	m := map[string]*columnSchema{}
	for _, column := range t.Columns {
		m[column.ColumnName] = column
	}
	return m
}

func (t Table) IndexMap() map[string]*Index {
	m := map[string]*Index{}
	for _, index := range t.Indexes {
		m[index.Name] = index
	}
	return m
}
