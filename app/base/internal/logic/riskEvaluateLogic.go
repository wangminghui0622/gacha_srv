package logic

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/core"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	basemodelnc "gacha_srv/pkg/model_nocache/gacha_base"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// riskActionChallenge 本仓案件全部 CHALLENGE（D4：action 恒 3）
const riskActionChallenge = 3

type RiskEvaluateLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRiskEvaluateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RiskEvaluateLogic {
	return &RiskEvaluateLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RiskEvaluate 风控评估（同步预检，非 SAGA 分支）。
//
// 错误语义：
//   - 业务失败（规则命中）→ CodeError(4002)，网关当场返回给前端
//   - 系统错误 → codes.Internal，折叠为 500
//
// 关键设计：
//   - 规则读取纯读（不占事务连接）；命中案例幂等落库（uk case_no="rk_"+requestId），
//     落库后返回——案例人工处置（Claim/Resolve），不撤销。
//   - 规则语义：启用规则任一命中即拒绝；全部未命中=通过（无落库）。
//   - M14d：状态数值化 1 OPEN；risk_score 由 severity 映射（LOW 30/MEDIUM 60/HIGH 90）；
//     action 恒 3 CHALLENGE。
func (l *RiskEvaluateLogic) RiskEvaluate(in *base.RiskEvaluateReq) (*base.RiskEvaluateResp, error) {
	if in.RequestId == "" || in.Scene == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "requestId与scene不能为空")
	}

	// ① 读启用规则（纯读）
	var rules []basemodelnc.RiskRules
	if err := l.svcCtx.DB.WithContext(l.ctx).Where("scene = ? AND enabled = 1", in.Scene).Find(&rules).Error; err != nil {
		l.Errorf("RiskEvaluate query rules failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	// 任一命中即拒绝
	var hit *basemodelnc.RiskRules
	for i := range rules {
		r := &rules[i]
		if factHolds(r.FactKey, r.Op, r.ExpectedValue, evalFacts{userId: in.UserId, boxId: in.BoxId, amountFen: in.AmountFen}) {
			hit = r
			break
		}
	}
	if hit == nil {
		l.Infof("RiskEvaluate passed, requestId: %s, scene: %s", in.RequestId, in.Scene)
		return &base.RiskEvaluateResp{Passed: true}, nil
	}
	reason := fmt.Sprintf("触发风控规则（%s %s %s）", hit.FactKey, hit.Op, hit.ExpectedValue)

	// ② 落案例（uk case_no 幂等；撞键读回已有结果）
	caseNo := "rk_" + in.RequestId
	entry := &basemodelnc.RiskCases{
		CaseNo:    caseNo,
		RuleNo:    hit.RuleNo,
		Scene:     in.Scene,
		UserId:    in.UserId,
		BoxId:     in.BoxId,
		AmountFen: in.AmountFen,
		RiskScore: core.SeverityScoreOf(hit.Severity),
		Action:    riskActionChallenge,
		Status:    repo.RiskCaseStatusOpen, // 数值 1 OPEN（M14d 状态机起点）
		CreatedAt: time.Now(),
	}
	if err := l.svcCtx.DB.WithContext(l.ctx).Create(entry).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			// 同 case_no 已落库：读回已有案例，幂等返回拒绝
			return l.readBackCase(caseNo)
		}
		l.Errorf("RiskEvaluate failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ③ 命中：案例已落库，返回业务码
	l.Errorf("RiskEvaluate rejected, requestId: %s, scene: %s, rule: %s, caseNo: %s, reason: %s",
		in.RequestId, in.Scene, hit.RuleNo, caseNo, reason)
	return nil, errorx.NewCodeError(errorx.CodeRiskRejected, reason)
}

// readBackCase 幂等路径：读已有案例如实返回拒绝
func (l *RiskEvaluateLogic) readBackCase(caseNo string) (*base.RiskEvaluateResp, error) {
	entry, err := l.svcCtx.RiskCaseModel.FindOneByCaseNo(l.ctx, caseNo)
	if err != nil {
		if errors.Is(err, basemodelnc.ErrNotFound) {
			l.Errorf("readBackCase: case not found, caseNo: %s", caseNo)
			return nil, status.Error(codes.Internal, "风控案例不存在")
		}
		l.Errorf("readBackCase failed, caseNo: %s, err: %v", caseNo, err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	msg := fmt.Sprintf("触发风控规则（%s），案例单号 %s", entry.RuleNo, entry.CaseNo)
	l.Infof("readBackCase: rejected, caseNo: %s, rule: %s", caseNo, entry.RuleNo)
	return nil, errorx.NewCodeError(errorx.CodeRiskRejected, msg)
}

// evalFacts 规则评估事实集（风控专用；资格评估 M14d 起走 CEL）
type evalFacts struct {
	userId    int64
	boxId     int64
	amountFen int64
}

// factHolds 判断规则条件是否满足。fact_key∈{always,user_id,box_id,amount_fen}；
// op∈{eq,neq,gte,lte,in}；未知事实键/非法数值=不满足（风控保守放行）
func factHolds(factKey, op, expected string, f evalFacts) bool {
	var fact string
	switch factKey {
	case "always":
		return true
	case "user_id":
		fact = strconv.FormatInt(f.userId, 10)
	case "box_id":
		fact = strconv.FormatInt(f.boxId, 10)
	case "amount_fen":
		fact = strconv.FormatInt(f.amountFen, 10)
	default:
		return false
	}

	switch op {
	case "eq":
		return fact == expected
	case "neq":
		return fact != expected
	case "in":
		for _, item := range strings.Split(expected, ",") {
			if strings.TrimSpace(item) == fact {
				return true
			}
		}
		return false
	case "gte", "lte":
		f, err1 := strconv.ParseInt(fact, 10, 64)
		e, err2 := strconv.ParseInt(strings.TrimSpace(expected), 10, 64)
		if err1 != nil || err2 != nil {
			return false
		}
		if op == "gte" {
			return f >= e
		}
		return f <= e
	default:
		return false
	}
}
