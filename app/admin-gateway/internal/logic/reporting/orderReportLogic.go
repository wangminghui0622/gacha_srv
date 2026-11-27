package reporting

import (
	"context"
	"regexp"
	"strings"
	"time"

	"gacha_srv/app/admin-gateway/internal/svc"
	"gacha_srv/app/identity/identity"
	"gacha_srv/app/ops/ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

var orderReportPhonePattern = regexp.MustCompile(`^1[3-9]\d{9}$`)

type OrderReportLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewOrderReportLogic(ctx context.Context, svcCtx *svc.ServiceContext) *OrderReportLogic {
	return &OrderReportLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// OrderReportFilter 订单报表筛选条件。
type OrderReportFilter struct {
	Start    time.Time
	End      time.Time
	UserID   string
	Nickname string
	Phone    string
}

type OrderReportRow struct {
	Date     string `json:"date"`
	UserID   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	CostFen  int64  `json:"costFen"`
}

type OrderReportResponse struct {
	Rows []OrderReportRow `json:"rows"`
}

// Query 订单报表：手机号在网关解析为用户ID，其余条件交给 ops-rpc 查询。
func (l *OrderReportLogic) Query(filter OrderReportFilter) (*OrderReportResponse, error) {
	var phoneUserID int64
	if phone := strings.TrimSpace(filter.Phone); phone != "" {
		if !orderReportPhonePattern.MatchString(phone) {
			return nil, errorx.NewCodeError(errorx.CodeParamError, "请输入完整的 11 位手机号")
		}
		reply, err := l.svcCtx.IdentityRpc.FindUserPhoneExact(l.ctx, &identity.FindUserPhoneExactReq{
			TenantId: 1,
			Phone:    phone,
		})
		if err != nil {
			normalized := errorx.FromError(err)
			if codeErr, ok := normalized.(*errorx.CodeError); ok && codeErr.Code == errorx.CodeNotFound {
				// 手机号未绑定用户：按无数据处理，避免把筛选条件当成服务故障。
				return &OrderReportResponse{Rows: []OrderReportRow{}}, nil
			}
			l.Errorf("OrderReport resolve phone failed, err: %v", err)
			return nil, normalized
		}
		phoneUserID = reply.GetUserId()
		if phoneUserID == 0 {
			return &OrderReportResponse{Rows: []OrderReportRow{}}, nil
		}
	}

	reply, err := l.svcCtx.OpsRpc.GetOrderReport(l.ctx, &ops.OrderReportRequest{
		StartDate:   filter.Start.Format("2006-01-02"),
		EndDate:     filter.End.Format("2006-01-02"),
		UserId:      strings.TrimSpace(filter.UserID),
		Nickname:    strings.TrimSpace(filter.Nickname),
		PhoneUserId: phoneUserID,
	})
	if err != nil {
		l.Errorf("OrderReport failed, start: %s, end: %s, err: %v",
			filter.Start.Format("2006-01-02"), filter.End.Format("2006-01-02"), err)
		return nil, errorx.FromError(err)
	}
	out := &OrderReportResponse{Rows: make([]OrderReportRow, 0, len(reply.GetRows()))}
	for _, row := range reply.GetRows() {
		out.Rows = append(out.Rows, OrderReportRow{
			Date:     row.GetDate(),
			UserID:   row.GetUserId(),
			Nickname: row.GetNickname(),
			CostFen:  row.GetCostFen(),
		})
	}
	return out, nil
}
