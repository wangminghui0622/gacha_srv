{{$hasDecimal := false}}{{$hasTime := false}}{{range .data.Fields}}{{if or (eq .DataType "float64") (eq .DataType "sql.NullFloat64")}}{{$hasDecimal = true}}{{end}}{{if or (eq .DataType "time.Time") (eq .DataType "sql.NullTime")}}{{$hasTime = true}}{{end}}{{end}}import (
	"context"
	"errors"
	"fmt"
	{{if $hasTime}}"time"{{end}}
	{{if $hasDecimal}}"github.com/shopspring/decimal"{{end}}
	"github.com/SpectatorNan/gorm-zero/gormc"
    "github.com/SpectatorNan/gorm-zero/gormc/batchx"
	"github.com/SpectatorNan/gorm-zero/gormc/pagex"
	"github.com/zeromicro/go-zero/core/stores/cache"
	"gorm.io/gorm"

	{{if .third}}{{.third}}{{end}}
)
