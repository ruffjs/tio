package shadow

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/pkg/errors"
	"github.com/xwb1989/sqlparser"
	"ruff.io/tio/pkg/log"
	"ruff.io/tio/pkg/model"
)

const (
	tableName = "shadow"
	idColumn  = "thingId"
)

var (
	validColumns = map[string]bool{idColumn: true, "createdAt": true, "updatedAt": true, "version": true,
		"connected": true, "connectedAt": true, "disconnectedAt": true, "remoteAddr": true}
	validJsonColumnPrefix = []string{"tags", "state.reported", "state.desired", "metadata"}
	statusColumns         = map[string]bool{"connected": true, "connectedAt": true, "disconnectedAt": true, "remoteAddr": true}
)

type ParsedQuerySql struct {
	Select            string
	Where             string
	OrderBy           string
	OriginSelectAlias map[string]string
	SelectStatusAlias map[string]string // status field and alias
}

var regMatchJsonExtr = regexp.MustCompile("`(json_extract\\([^`]+\\))`")

func parseQuerySql(qrySql string) (ParsedQuerySql, error) {
	res := ParsedQuerySql{}
	stmt, err := sqlparser.Parse(qrySql)
	if err != nil {
		return res, err
	}
	selStmt, ok := stmt.(*sqlparser.Select)
	if !ok {
		return res, errors.New("unsupported sql type, only select been supported")
	}
	res.OriginSelectAlias = toSelectAlias(selStmt.SelectExprs)
	res.SelectStatusAlias = updateSelectFields(selStmt)

	err = sqlparser.Walk(func(node sqlparser.SQLNode) (kontinue bool, err error) {
		switch node := node.(type) {
		case *sqlparser.ColName:
			err := validColumn(node.Name.String())
			if err != nil {
				return false, err
			}
			n := toDbCol(node.Name.String())
			node.Name = sqlparser.NewColIdent(n)
		case *sqlparser.AliasedExpr:
			addAliasForSelectField(node)
		}
		return true, nil
	}, selStmt)
	if err != nil {
		return res, err
	}

	if sqlNodeToString(selStmt.From) != tableName {
		return res, fmt.Errorf("table must be %q", tableName)
	}

	for _, selExp := range selStmt.SelectExprs {
		if alSel, ok := selExp.(*sqlparser.AliasedExpr); ok {
			sn := sqlNodeToString(alSel)
			if strings.Contains(sn, "json_extract") {
				selStmt.SelectExprs = append(selStmt.SelectExprs, &sqlparser.AliasedExpr{
					Expr: &sqlparser.FuncExpr{
						Name:  sqlparser.NewColIdent("json_type"),
						Exprs: sqlparser.SelectExprs{&sqlparser.AliasedExpr{Expr: alSel.Expr}},
					},
					As: sqlparser.NewColIdent("$type_" + alSel.As.String()),
				})
			}
		}
	}
	res.Select = trimJsonExtractSpecialChar(sqlNodeToString(selStmt.SelectExprs))
	if res.Select != "*" && strings.Contains(res.Select, "*") {
		return res, errors.WithMessage(model.ErrInvalidParams, "invalid select field \"*\" in query")
	}

	if selStmt.Where != nil {
		res.Where = sqlNodeToString(selStmt.Where.Expr)
		// remote char ` for json_extract
		res.Where = trimJsonExtractSpecialChar(res.Where)
	}

	if selStmt.OrderBy != nil {
		res.OrderBy = sqlNodeToString(selStmt.OrderBy)
		res.OrderBy = strings.TrimPrefix(res.OrderBy, " order by ")
	}

	allSql := sqlNodeToString(selStmt)
	allSql = trimJsonExtractSpecialChar(allSql)
	log.Debugf("parsed SQL: %s", allSql)

	return res, nil
}

// updateSelectFields
// - Delete status field in select expressions, and return theirs name alia map.
// - If there is no id column, add it.
func updateSelectFields(stmt *sqlparser.Select) map[string]string {
	var res = make(map[string]string, 0)
	var selectExprs = make(sqlparser.SelectExprs, 0)
	hasThingIdField := false
	for _, sel := range stmt.SelectExprs {
		isStatusCol := false
		if _, ok := sel.(*sqlparser.StarExpr); ok {
			hasThingIdField = true
		}
		if s, ok := sel.(*sqlparser.AliasedExpr); ok {
			if e, ok := s.Expr.(*sqlparser.ColName); ok {
				if e.Name.String() == idColumn {
					hasThingIdField = true
				}
				if _, ok := statusColumns[e.Name.String()]; ok {
					if s.As.String() != "" {
						res[e.Name.String()] = s.As.String()
					} else {
						res[e.Name.String()] = e.Name.String()
					}
					isStatusCol = true
				}
			}
		}
		if !isStatusCol {
			selectExprs = append(selectExprs, sel)
		}
	}
	if !hasThingIdField {
		// id column must be added
		selectExprs = append(selectExprs, &sqlparser.AliasedExpr{Expr: &sqlparser.ColName{Name: sqlparser.NewColIdent(idColumn)}})
	}
	stmt.SelectExprs = selectExprs
	return res
}

func toSelectAlias(sel sqlparser.SelectExprs) map[string]string {
	selAlias := make(map[string]string, 0)
	for _, sel := range sel {
		if asel, ok := sel.(*sqlparser.AliasedExpr); ok {
			n := sqlNodeToString(asel.Expr)
			n = strings.Trim(n, "`")
			alia := sqlNodeToString(asel.As)
			if alia == "" {
				narr := strings.Split(n, ".")
				alia = narr[len(narr)-1]
			}
			selAlias[n] = alia
		}
	}
	return selAlias
}

// addAliasForSelectField
// - set json path field alias as last world of it
// - set alias for valid columns before update it to snake style
func addAliasForSelectField(node *sqlparser.AliasedExpr) {
	nb := sqlparser.NewTrackedBuffer(nil)
	node.Expr.Format(nb)
	nn := nb.String()
	var isJsonPath bool
	for _, p := range validJsonColumnPrefix {
		if strings.HasPrefix(nn, "`"+p+".") {
			isJsonPath = true
			break
		}
	}
	if isJsonPath && node.As.EqualString("") {
		tt := strings.ReplaceAll(nn, "`", "")
		arr := strings.Split(tt, ".")
		n := arr[len(arr)-1]
		node.As = sqlparser.NewColIdent(n)
	}

	if !isJsonPath && node.As.EqualString("") {
		if ok := validColumns[nn]; ok {
			node.As = sqlparser.NewColIdent(nn)
		}
	}
}

func validColumn(col string) error {
	if ok := validColumns[col]; ok {
		return nil
	}
	for _, p := range validJsonColumnPrefix {
		if strings.HasPrefix(col, p) {
			return nil
		}
	}
	return fmt.Errorf("column %q is invalid", col)
}

// Convert to db recognizable column
// - json_extract column for tags, state.reported, state.desired
// - convert camel style column to snake style
func toDbCol(col string) string {
	fArr := strings.Split(col, ".")
	f1 := fArr[0]
	if f1 == "tags" {
		return toJsonExtract(fArr)
	}
	if len(fArr) > 1 {
		f2 := fArr[1]
		if f1 == "state" && (f2 == "reported" || f2 == "desired") {
			return toJsonExtract(fArr[1:])
		}
	}
	// column first part to snake style
	fArr[0] = camelToSnake(f1)
	return strings.Join(fArr, ".")
}

// eg [a, b, c] to json_extract(a,$.b.c)
func toJsonExtract(fArr []string) string {
	return fmt.Sprintf("json_extract(%s, '$.%s')", fArr[0], strings.Join(fArr[1:], "."))
}

func camelToSnake(camelCase string) string {
	var buf bytes.Buffer
	for i, r := range camelCase {
		if unicode.IsUpper(r) {
			if i > 0 && unicode.IsLower(rune(camelCase[i-1])) {
				buf.WriteByte('_')
			}
			buf.WriteRune(unicode.ToLower(r))
		} else {
			buf.WriteRune(r)
		}
	}
	return buf.String()
}

func sqlNodeToString(n sqlparser.SQLNode) string {
	buf := sqlparser.NewTrackedBuffer(nil)
	n.Format(buf)
	return buf.String()
}

func trimJsonExtractSpecialChar(s string) string {
	return regMatchJsonExtr.ReplaceAllString(s, "$1")
}
