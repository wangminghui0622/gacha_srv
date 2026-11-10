{{$hasDecimal := false}}{{$hasTime := false}}{{range .data.Fields}}{{if or (eq .DataType "float64") (eq .DataType "sql.NullFloat64")}}{{$hasDecimal = true}}{{end}}{{if or (eq .DataType "time.Time") (eq .DataType "sql.NullTime")}}{{$hasTime = true}}{{end}}{{end}}import (
	"context"
	"github.com/SpectatorNan/gorm-zero/gormc"
	{{if $hasDecimal}}"github.com/shopspring/decimal"{{end}}
	{{if $hasTime}}"time"{{end}}

	"gorm.io/gorm"
    "github.com/SpectatorNan/gorm-zero/gormc/pagex"
	{{if .third}}{{.third}}{{end}}
)
