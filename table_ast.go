package migu

import (
	"fmt"
	"go/ast"
	"reflect"
	"strconv"
	"strings"

	"github.com/naoina/migu/dialect"
)

type TableAST struct {
	Name        string
	Schema      *ast.StructType
	IndexSchema *ast.StructType
}

func (t *TableAST) HasSchema() bool {
	return t.Schema != nil
}

func (t *TableAST) ColumnMap() map[string]*field {
	m := map[string]*field{}
	for _, column := range t.MustColumns() {
		m[column.Name] = column
	}
	return m
}

func (t *TableAST) IndexMap() map[string]*Index {
	m := map[string]*Index{}
	for _, index := range t.MustIndexes() {
		m[index.Name] = index
	}
	return m
}

func (t *TableAST) Columns() ([]*field, error) {
	models := make([]*field, 0)
	if !t.HasSchema() {
		return models, fmt.Errorf("migu: TableAST.Columns error: %s schema is empty", t.Name)
	}

	structAST := t.Schema
	for _, fld := range structAST.Fields.List {
		typeName, err := detectTypeName(fld)
		if err != nil {
			return nil, fmt.Errorf("migu: TableAST.Columns error: " + err.Error())
		}
		f, err := newField(typeName, fld)
		if err != nil {
			return nil, fmt.Errorf("migu: TableAST.Columns error: " + err.Error())
		}
		if f.Ignore {
			continue
		}
		for _, ident := range fld.Names {
			field := *f
			field.Name = toSchemaFieldName(ident.Name)
			models = append(models, &field)
		}
	}
	return models, nil
}

func (t *TableAST) Indexes() ([]*Index, error) {
	indexes := make([]*Index, 0)
	if t.IndexSchema == nil {
		return indexes, nil
	}
	for _, fld := range t.IndexSchema.Fields.List {
		if fld.Tag == nil {
			continue
		}
		s, err := strconv.Unquote(fld.Tag.Value)
		if err != nil {
			return nil, fmt.Errorf("migu: TableAST.Indexes error. " + err.Error())
		}
		index, err := parseIndexStructTag(reflect.StructTag(s))
		if err != nil {
			return nil, fmt.Errorf("migu: TableAST.Indexes error. " + err.Error())
		}
		indexes = append(indexes, index)
	}
	return indexes, nil
}

func (t *TableAST) MustColumns() []*field {
	c, err := t.Columns()
	if err != nil {
		panic(err)
	}
	return c
}

func (t *TableAST) MustIndexes() []*Index {
	i, err := t.Indexes()
	if err != nil {
		panic(err)
	}
	return i
}

// see: https://dev.mysql.com/doc/refman/5.6/en/create-table.html
func (t *TableAST) CreateTableQuery(d dialect.Dialect) ([]string, error) {
	model, err := t.Columns()
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.CreateTableQuery error. " + err.Error())
	}
	columns := make([]string, len(model))
	for i, f := range model {
		columns[i] = columnSQL(d, f)
	}

	indexes, err := t.indexDefinitions(d)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.CreateTableQuery error. " + err.Error())
	}

	createDefinitions := append(columns, indexes...)
	createTableQuery := fmt.Sprintf(`CREATE TABLE %s (
  %s
)`, d.Quote(toSchemaTableName(t.Name)), strings.Join(createDefinitions, ",\n  "))

	return []string{createTableQuery}, nil
}

func (t *TableAST) indexDefinitions(d dialect.Dialect) ([]string, error) {
	indexes, err := t.Indexes()
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.indexDefinitions error. " + err.Error())
	}

	indexDefinitions := make([]string, 0)
	for _, index := range indexes {
		indexDefinitions = append(indexDefinitions, index.AsCreateTableDefinition(d))
	}
	return indexDefinitions, nil
}

func (t *TableAST) AlterTableQueries(d dialect.Dialect, currentTable *Table) ([]string, error) {
	tableName := toSchemaTableName(t.Name)
	currentColumMap := currentTable.ColumnMap()
	currentIndexMap := currentTable.IndexMap()
	addSQLs, err := t.GenerateAddFieldSQLs(d, currentColumMap)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.AlterTableQueries error: " + err.Error())
	}
	dropSQLs, err := t.GenerateDropFieldSQLs(d, currentColumMap)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.AlterTableQueries error: " + err.Error())
	}
	modifySQLs, err := t.GenerateModifyFieldSQLs(d, currentColumMap)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.AlterTableQueries error: " + err.Error())
	}

	dropIndexSQLs, err := t.GenerateDropIndexSQLs(d, currentIndexMap)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.AlterTableQueries error: " + err.Error())
	}
	addIndexSQLs, err := t.GenerateAddIndexSQLs(d, currentIndexMap)
	if err != nil {
		return nil, fmt.Errorf("migu: TableAST.AlterTableQueries error: " + err.Error())
	}

	migrations := make([]string, 0)
	migrations = append(migrations, addSQLs...)
	migrations = append(migrations, dropSQLs...)
	migrations = append(migrations, modifySQLs...)
	migrations = append(migrations, dropIndexSQLs...)
	migrations = append(migrations, addIndexSQLs...)
	if len(migrations) <= 0 {
		return nil, nil
	}

	alterTableQuery := fmt.Sprintf(`ALTER TABLE %s
  %s`, d.Quote(tableName), strings.Join(migrations, ",\n  "))
	return []string{alterTableQuery}, nil
}

func (t *TableAST) GenerateAddFieldSQLs(d dialect.Dialect, currentColumMap map[string]*columnSchema) ([]string, error) {
	sqls := make([]string, 0)
	for _, column := range t.MustColumns() {
		if _, exist := currentColumMap[column.Name]; exist {
			continue
		}
		sqls = append(sqls, fmt.Sprintf(`ADD %s`, columnSQL(d, column)))
	}
	return sqls, nil
}

func (t *TableAST) GenerateDropFieldSQLs(d dialect.Dialect, currentColumMap map[string]*columnSchema) ([]string, error) {
	sqls := make([]string, 0)
	newColumnMap := t.ColumnMap()
	for columnName, _ := range currentColumMap {
		if _, exist := newColumnMap[columnName]; exist {
			continue
		}
		sqls = append(sqls, fmt.Sprintf(`DROP %s`, d.Quote(toSchemaTableName(columnName))))
	}
	return sqls, nil
}

func (t *TableAST) GenerateModifyFieldSQLs(d dialect.Dialect, currentColumMap map[string]*columnSchema) ([]string, error) {
	sqls := make([]string, 0)
	for _, column := range t.MustColumns() {
		if _, exist := currentColumMap[column.Name]; !exist {
			continue
		}
		currentColum := currentColumMap[column.Name]
		if !currentColum.HasDifference(column) {
			continue
		}
		sqls = append(sqls, fmt.Sprintf(`MODIFY %s`, columnSQL(d, column)))
	}
	return sqls, nil
}

func (t *TableAST) GenerateAddIndexSQLs(d dialect.Dialect, currentIndexMap map[string]*Index) ([]string, error) {
	sqls := make([]string, 0)
	newIndexMap := t.IndexMap()
	for indexName, index := range newIndexMap {
		if currentIndex, exist := currentIndexMap[indexName]; exist {
			if reflect.DeepEqual(index, currentIndex) {
				continue
			}
		}
		sqls = append(sqls, fmt.Sprintf("ADD %s", index.AsCreateTableDefinition(d)))
	}
	return sqls, nil
}

func (t *TableAST) GenerateDropIndexSQLs(d dialect.Dialect, currentIndexMap map[string]*Index) ([]string, error) {
	sqls := make([]string, 0)
	newIndexMap := t.IndexMap()
	for indexName, index := range currentIndexMap {
		if newIndex, exist := newIndexMap[indexName]; exist {
			if reflect.DeepEqual(index, newIndex) {
				continue
			}
		}

		if index.isPrimaryKey() {
			sqls = append(sqls, "DROP PRIMARY KEY")
		} else {
			sqls = append(sqls, fmt.Sprintf(`DROP INDEX %s`, d.Quote(indexName)))
		}
	}
	return sqls, nil
}
