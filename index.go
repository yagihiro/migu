package migu

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"

	"github.com/astronoka/migu/dialect"
)

// Index is table index
type Index struct {
	Name        string
	Unique      bool
	ColumnNames []string
}

func (index *Index) isPrimaryKey() bool {
	return strings.ToUpper(index.Name) == "PRIMARY"
}

func (index *Index) isUniqueKey() bool {
	return index.Unique && !index.isPrimaryKey()
}

func (index *Index) AsASTField(indexNo int) (*ast.Field, error) {
	field := &ast.Field{
		Names: []*ast.Ident{
			ast.NewIdent(fmt.Sprintf("Index%02d", indexNo)),
		},
		Type: ast.NewIdent("interface{}"),
	}
	var tags []string
	if index.isPrimaryKey() {
		tags = append(tags, tagPrimaryKey)
	}
	if index.isUniqueKey() {
		tags = append(tags, tagUnique)
	}
	if len(index.ColumnNames) > 0 {
		tags = append(tags, tagIndex+":"+index.Name+","+strings.Join(index.ColumnNames, ","))
	}
	if len(tags) > 0 {
		field.Tag = &ast.BasicLit{
			Kind:  token.STRING,
			Value: fmt.Sprintf("`migu:\"%s\"`", strings.Join(tags, tagSeparater)),
		}
	}
	return field, nil
}

func (index *Index) AsCreateTableDefinition(d dialect.Dialect) string {
	quotedNames := make([]string, 0, len(index.ColumnNames))
	for _, name := range index.ColumnNames {
		quotedNames = append(quotedNames, d.Quote(name))
	}
	if index.isPrimaryKey() {
		return fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(quotedNames, ","))
	}
	keyType := "INDEX"
	if index.isUniqueKey() {
		keyType = "UNIQUE"
	}
	return fmt.Sprintf("%s %s (%s)", keyType, d.Quote(index.Name), strings.Join(quotedNames, ","))
}
