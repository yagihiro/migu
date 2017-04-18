package migu

import (
	"database/sql"
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strings"
)

type columnSchema struct {
	TableName              string
	ColumnName             string
	OrdinalPosition        int64
	ColumnDefault          sql.NullString
	IsNullable             string
	DataType               string
	CharacterMaximumLength *uint64
	CharacterOctetLength   sql.NullInt64
	NumericPrecision       sql.NullInt64
	NumericScale           sql.NullInt64
	ColumnType             string
	ColumnKey              string
	Extra                  string
	ColumnComment          string
	NonUnique              int64
	IndexName              string
}

func (schema *columnSchema) fieldAST() (*ast.Field, error) {
	types, err := schema.GoFieldTypes()
	if err != nil {
		return nil, err
	}
	field := &ast.Field{
		Names: []*ast.Ident{
			ast.NewIdent(toStructPublicFieldName(schema.ColumnName)),
		},
		Type: ast.NewIdent(types[0]),
	}
	var tags []string
	if schema.ColumnDefault.Valid {
		tags = append(tags, tagDefault+":"+schema.ColumnDefault.String)
	}
	if schema.hasPrimaryKey() {
		tags = append(tags, tagPrimaryKey)
	}
	if schema.hasAutoIncrement() {
		tags = append(tags, tagAutoIncrement)
	}
	if schema.hasUniqueKey() {
		tags = append(tags, tagUnique)
	}
	if schema.hasSize() {
		tags = append(tags, fmt.Sprintf("%s:%d", tagSize, *schema.CharacterMaximumLength))
	}
	if len(tags) > 0 {
		field.Tag = &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("`migu:\"%s\"`", strings.Join(tags, tagSeparater)),
		}
	}
	if schema.ColumnComment != "" {
		field.Comment = &ast.CommentGroup{
			List: []*ast.Comment{
				{Text: " // " + schema.ColumnComment},
			},
		}
	}
	return field, nil
}

func (schema *columnSchema) GoFieldTypes() ([]string, error) {
	switch schema.DataType {
	case "tinyint":
		if schema.isUnsigned() {
			if schema.isNullable() {
				return []string{"*uint8"}, nil
			}
			return []string{"uint8"}, nil
		}
		if schema.isNullable() {
			return []string{"*int8", "*bool", "sql.NullBool"}, nil
		}
		return []string{"int8", "bool"}, nil
	case "smallint":
		if schema.isUnsigned() {
			if schema.isNullable() {
				return []string{"*uint16"}, nil
			}
			return []string{"uint16"}, nil
		}
		if schema.isNullable() {
			return []string{"*int16"}, nil
		}
		return []string{"int16"}, nil
	case "mediumint", "int":
		if schema.isUnsigned() {
			if schema.isNullable() {
				return []string{"*uint", "*uint32"}, nil
			}
			return []string{"uint", "uint32"}, nil
		}
		if schema.isNullable() {
			return []string{"*int", "*int32"}, nil
		}
		return []string{"int", "int32"}, nil
	case "bigint":
		if schema.isUnsigned() {
			if schema.isNullable() {
				return []string{"*uint64"}, nil
			}
			return []string{"uint64"}, nil
		}
		if schema.isNullable() {
			return []string{"*int64", "sql.NullInt64"}, nil
		}
		return []string{"int64"}, nil
	case "varchar", "text", "mediumtext", "longtext":
		if schema.isNullable() {
			return []string{"*string", "sql.NullString"}, nil
		}
		return []string{"string"}, nil
	case "datetime":
		if schema.isNullable() {
			return []string{"*time.Time"}, nil
		}
		return []string{"time.Time"}, nil
	case "double":
		if schema.isNullable() {
			return []string{"*float32", "*float64", "sql.NullFloat64"}, nil
		}
		return []string{"float32", "float64"}, nil
	case "float":
		if schema.isNullable() {
			return []string{"*float32", "sql.NullFloat64"}, nil
		}
		return []string{"float32"}, nil
	default:
		return nil, fmt.Errorf("BUG: unexpected data type: %s", schema.DataType)
	}
}

func (schema *columnSchema) isUnsigned() bool {
	return strings.Contains(schema.ColumnType, "unsigned")
}

func (schema *columnSchema) isNullable() bool {
	return strings.ToUpper(schema.IsNullable) == "YES"
}

func (schema *columnSchema) hasPrimaryKey() bool {
	return schema.ColumnKey == "PRI" && strings.ToUpper(schema.IndexName) == "PRIMARY"
}

func (schema *columnSchema) hasAutoIncrement() bool {
	return schema.Extra == "auto_increment"
}

func (schema *columnSchema) hasUniqueKey() bool {
	return schema.ColumnKey != "" && schema.IndexName != "" && !schema.hasPrimaryKey()
}

func (schema *columnSchema) hasSize() bool {
	return schema.DataType == "varchar" && schema.CharacterMaximumLength != nil && *schema.CharacterMaximumLength != uint64(255)
}

func (schema *columnSchema) HasDifference(newColumn *field) bool {
	goTypes, err := schema.GoFieldTypes()
	if err != nil {
		panic(err)
	}
	if !inStrings(goTypes, newColumn.Type) {
		return true
	}

	fieldAST, err := schema.fieldAST()
	if err != nil {
		panic(err)
	}
	currentColumn, err := newField(newColumn.Type, fieldAST)
	currentColumn.Name = newColumn.Name
	if err != nil {
		panic(err)
	}
	return !reflect.DeepEqual(currentColumn, newColumn)
}
