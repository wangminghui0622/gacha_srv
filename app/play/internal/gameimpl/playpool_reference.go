package gameimpl

import (
	"context"
	"fmt"
	"strings"

	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type playPoolReferenceRow struct {
	PoolID     int64  `gorm:"column:pool_id"`
	PoolName   string `gorm:"column:pool_name"`
	Mode       string `gorm:"column:mode"`
	Status     int64  `gorm:"column:status"`
	TagsJSON   string `gorm:"column:tags_json"`
	PrizeGrade string `gorm:"column:prize_grade"`
	Quantity   int64  `gorm:"column:quantity"`
}

type QueryPlayPoolReferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewQueryPlayPoolReferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *QueryPlayPoolReferencesLogic {
	return &QueryPlayPoolReferencesLogic{ctx: ctx, svcCtx: svcCtx, Logger: logx.WithContext(ctx)}
}

var playModesForReference = []string{"memorymatch", "gashapon", "fullcar", "combochain", "dig", "groupbuy"}

func playPrizeUnionSQL() string {
	parts := make([]string, 0, len(playModesForReference))
	for _, mode := range playModesForReference {
		parts = append(parts, fmt.Sprintf(
			"SELECT pool_id, prize_grade, quantity, sku_id, child_pool_id, prize_type FROM play_%s_prizes", mode))
	}
	return strings.Join(parts, " UNION ALL ")
}

// QueryPlayPoolReferences 反查商品/池子被哪些奖池引用。
func (l *QueryPlayPoolReferencesLogic) QueryPlayPoolReferences(in *play.QueryPlayPoolReferencesReq) (*play.QueryPlayPoolReferencesResp, error) {
	if (in.RefType != 1 && in.RefType != 2) || in.RefId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "引用类型或引用ID非法")
	}
	page, pageSize := in.Page, in.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}

	condition := "p.sku_id = ? AND p.prize_type = 2"
	source := "商品库"
	if in.RefType == 2 {
		condition = "p.child_pool_id = ?"
		source = "赏池"
	}
	from := " FROM (" + playPrizeUnionSQL() + ") p " +
		"JOIN play_pool_catalog c ON c.id = p.pool_id WHERE " + condition + " "

	var total int64
	if err := l.svcCtx.DB.Raw("SELECT COUNT(*) "+from, in.RefId).Scan(&total).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "查询引用失败")
	}

	var rows []playPoolReferenceRow
	query := "SELECT p.pool_id, c.name AS pool_name, c.mode, c.status, c.tags_json, " +
		"p.prize_grade, p.quantity " + from + "ORDER BY p.pool_id DESC LIMIT ? OFFSET ?"
	offset := (page - 1) * pageSize
	if err := l.svcCtx.DB.Raw(query, in.RefId, pageSize, offset).Scan(&rows).Error; err != nil {
		return nil, errorx.NewCodeError(errorx.CodeDBError, "查询引用失败")
	}

	resp := &play.QueryPlayPoolReferencesResp{Total: total, Page: page, PageSize: pageSize}
	for i := range rows {
		row := &rows[i]
		resp.Items = append(resp.Items, &play.PlayPoolReferenceItem{
			PoolId:     row.PoolID,
			PoolName:   row.PoolName,
			Mode:       row.Mode,
			Periods:    0,
			Status:     legacyPoolStatus(row.Status),
			Tags:       decodePoolTags(row.TagsJSON),
			Source:     source,
			RefType:    in.RefType,
			RefId:      in.RefId,
			PrizeGrade: row.PrizeGrade,
			Quantity:   row.Quantity,
		})
	}
	return resp, nil
}
