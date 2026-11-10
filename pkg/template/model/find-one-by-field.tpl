
{{$paramType := .in}}{{range .data.Fields}}{{if eq .Name.ToCamel $.upperField}}{{if eq .DataType "sql.NullString"}}{{$paramType = "*string"}}{{else}}{{if eq .DataType "sql.NullInt64"}}{{$paramType = "*int64"}}{{else}}{{if eq .DataType "sql.NullTime"}}{{$paramType = "*time.Time"}}{{else}}{{if eq .DataType "sql.NullFloat64"}}{{$paramType = "*decimal.Decimal"}}{{end}}{{end}}{{end}}{{end}}{{end}}{{end}}
func (m *default{{.upperStartCamelObject}}Model) FindOneBy{{.upperField}}(ctx context.Context, {{if eq $paramType .in}}{{.in}}{{else}}{{.lowerStartCamelField}} {{$paramType}}{{end}}) (*{{.upperStartCamelObject}}, error) {
	{{if .withCache}}{{.cacheKey}}
	var resp {{.upperStartCamelObject}}
	err := m.QueryRowIndexCtx(ctx, &resp, {{.cacheKeyVariable}}, m.formatPrimary, func(conn *gorm.DB, v interface{}) (interface{}, error) {
		if err := conn.Model(&{{.upperStartCamelObject}}{}).Where("{{.originalField}}", {{.lowerStartCamelField}}).Take(&resp).Error; err != nil {
			return nil, err
		}
		return resp.{{.upperStartCamelPrimaryKey}}, nil
	}, m.queryPrimary)
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}{{else}}var resp {{.upperStartCamelObject}}
	err := m.conn.WithContext(ctx).Model(&{{.upperStartCamelObject}}{}).Where("{{.originalField}}", {{.lowerStartCamelField}}).Take(&resp).Error
	switch err {
	case nil:
		return &resp, nil
	case gormc.ErrNotFound:
		return nil, ErrNotFound
	default:
		return nil, err
	}
}{{end}}
