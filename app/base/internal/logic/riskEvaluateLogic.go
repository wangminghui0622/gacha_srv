package logic

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gacha_srv/app/base/base"
	"gacha_srv/app/base/internal/core"
	"gacha_srv/app/base/internal/repo"
	"gacha_srv/app/base/internal/svc"
	"gacha_srv/app/dtm/client/dtmgrpc"
	basemodelnc "gacha_srv/pkg/model_nocache/gacha_base"
	"gacha_srv/pkg/utils/errorx"
	"gacha_srv/pkg/utils/txutil"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// errCaseDup 事务内哨兵：同 case_no 风控案例已存在（历史分支已评估）
var errCaseDup = errors.New("risk case duplicated")

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

// RiskEvaluate SAGA 分支 02 正向：风控评估。
//
// 错误语义（SAGA 分支铁律）：
//   - 业务失败（规则命中）→ codes.Aborted(5002)，DTM 触发补偿
//   - 系统错误 → codes.Internal，DTM 退避重试
//
// 关键设计：
//   - 规则读取在屏障事务外（纯读）；命中案例在屏障事务内落库
//     （uk case_no="rk_"+requestId 幂等），落库后返回 Aborted——
//     案例人工处置（Claim/Resolve），不随补偿撤销。
//   - 规则语义：启用规则任一命中即拒绝；全部未命中=通过（无落库，无屏障注册）。
//   - M14d：状态数值化 1 OPEN；risk_score 由 severity 映射（LOW 30/MEDIUM 60/HIGH 90）；
//     action 恒 3 CHALLENGE。
func (l *RiskEvaluateLogic) RiskEvaluate(in *base.RiskEvaluateReq) (*base.RiskEvaluateResp, error) {
	if in.RequestId == "" || in.Scene == "" {
		return nil, status.Error(codes.Aborted, "code=400 msg=requestId与scene不能为空")
	}

	// ① 屏障事务外读启用规则（纯读）
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

	// ② 屏障事务内落案例（uk 幂等；撞键/屏障跳过=读回已有结果）
	barrier, err := dtmgrpc.BarrierFromGrpc(l.ctx)
	if err != nil {
		l.Errorf("RiskEvaluate get barrier failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	rawDB, err := l.svcCtx.DB.DB()
	if err != nil {
		l.Errorf("RiskEvaluate get raw db failed, err: %v", err)
		return nil, status.Error(codes.Internal, err.Error())
	}
	caseNo := "rk_" + in.RequestId
	businessRan := false
	err = barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
		businessRan = true
		db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
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
		if err := db.Create(entry).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return errCaseDup
			}
			return err
		}
		return nil
	})
	if errors.Is(err, errCaseDup) || !businessRan {
		// 同 case_no 已落库/屏障跳过：读回已有案例，幂等返回拒绝
		return l.readBackCase(caseNo)
	}
	if err != nil {
		l.Errorf("RiskEvaluate failed, requestId: %s, err: %v", in.RequestId, err)
		return nil, status.Error(codes.Internal, err.Error())
	}

	// ③ 命中：案例已落库，返回 Aborted 触发补偿
	l.Errorf("RiskEvaluate rejected, requestId: %s, scene: %s, rule: %s, caseNo: %s, reason: %s",
		in.RequestId, in.Scene, hit.RuleNo, caseNo, reason)
	return nil, status.Error(codes.Aborted,
		fmt.Sprintf("code=%d msg=%s", errorx.CodeRiskRejected, reason))
}

// readBackCase 幂等/屏障跳过路径：读已有案例如实返回拒绝
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
	return nil, status.Error(codes.Aborted, fmt.Sprintf("code=%d msg=%s", errorx.CodeRiskRejected, msg))
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
