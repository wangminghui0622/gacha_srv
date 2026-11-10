// Package saga 网关统一 SAGA 组装器：7 个购买流程共用（docs/design/service-division.md §4），
// 消除旧仓 draw/sagaOps.go 与各玩法 buyLogic 内联组装的拷贝粘贴。
// 分支错误语义（DTM 协议）：业务失败 Aborted=触发补偿，系统错误 Internal=退避重试。
package saga

import (
	"context"
	"time"

	"gacha_srv/app/dtm/client/dtmgrpc"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/protobuf/proto"
)

// Branch 一个 SAGA 分支：Action/Revert 为 DTM 直连地址（"<direct>/<service.Method>"），Payload 为分支请求。
type Branch struct {
	Action  string
	Revert  string
	Payload proto.Message
}

// Recheck 幂等回查：同 gid 事务已存在（重试成功）返回 nil；未命中返回非 nil。
// 各购买流程用各自的记录查询 RPC 实现（draw 查抽赏记录、玩法查各自 records）。
type Recheck func(ctx context.Context) error

// Builder 通用 SAGA 组装器：分支列表 + 重试上限 + 幂等回查模板。
type Builder struct {
	ctx        context.Context
	log        logx.Logger
	dtmTarget  string
	gid        string
	branches   []Branch
	retryLimit int64
}

// New 创建组装器。gid 为幂等键（HTTP Idempotency-Key 或服务端 uuid），DTM 以 gid 去重。
func New(ctx context.Context, log logx.Logger, dtmTarget, gid string) *Builder {
	return &Builder{ctx: ctx, log: log, dtmTarget: dtmTarget, gid: gid, retryLimit: 10}
}

// Add 追加一个分支（正向/补偿成对定义，Action/Revert 缺省空补偿）。
func (b *Builder) Add(action, revert string, payload proto.Message) *Builder {
	b.branches = append(b.branches, Branch{Action: action, Revert: revert, Payload: payload})
	return b
}

// WithRetryLimit 覆盖重试上限（默认 10，系统错误退避重试封顶）。
func (b *Builder) WithRetryLimit(n int64) *Builder {
	b.retryLimit = n
	return b
}

// Submit 组装并提交事务。failMsg 为提交失败时的用户可见错误文案。
// 返回 alreadyDone=true 表示同 gid 事务已存在（幂等重放，重试成功）；
// 否则返回 CodeDTMError。Submit 立即返回：只代表事务被 DTM 接受，分支异步执行。
func (b *Builder) Submit(failMsg string, recheck Recheck) (alreadyDone bool, err error) {
	saga := dtmgrpc.NewSagaGrpcWithContext(b.ctx, b.dtmTarget, b.gid)
	for _, br := range b.branches {
		saga.Add(br.Action, br.Revert, br.Payload)
	}
	saga.WithRetryLimit(b.retryLimit)

	if err := saga.Submit(); err != nil {
		// 幂等回查：Submit 失败很可能是「同 gid 事务已存在」——上次提交其实成功了、
		// 只是响应丢失，客户端重试打到这里。回查业务记录：命中 → 幂等成功返回；
		// 未命中才是真故障。独立短超时 ctx：客户端重试场景 ctx 可能已近截止（dtm_shop 同款）。
		if recheck != nil {
			fallbackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if recheck(fallbackCtx) == nil {
				b.log.Infof("retry with same idempotency key, returning existing record, gid: %s", b.gid)
				return true, nil
			}
		}
		b.log.Errorf("dtm saga submit failed, gid: %s, err: %v", b.gid, err)
		return false, errorx.NewCodeError(errorx.CodeDTMError, failMsg)
	}
	b.log.Infof("dtm saga submitted, gid: %s", b.gid)
	return false, nil
}
