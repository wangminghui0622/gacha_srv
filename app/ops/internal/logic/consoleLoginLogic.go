package logic

import (
	"context"

	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/repo"
	"gacha_srv/app/ops/internal/svc"
	"gacha_srv/app/ops/ops"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"
	"gacha_srv/pkg/utils/errorx"

	"github.com/zeromicro/go-zero/core/logx"
)

type ConsoleLoginLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewConsoleLoginLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsoleLoginLogic {
	return &ConsoleLoginLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ConsoleLogin 控制台登录（照抄 server_v2 console_totp_login 两步流程）：
//  1. 用户名+密码 → 未绑 TOTP：mode=setup，票据挂待绑密钥，回 challenge{setup_required, totp_secret, totp_uri}
//     已绑 TOTP：mode=verify，回 challenge{login_ticket, setup_required=false}
//  2. login_ticket + otp_code → 验证/绑定通过后签发会话令牌对
//
// 票据 5 分钟 TTL（Redis），一次性消费。SkipTotp（仅本地调试）直接签发。
func (l *ConsoleLoginLogic) ConsoleLogin(in *ops.ConsoleLoginRequest) (*ops.ConsoleLoginReply, error) {
	tenantID := defaultTenant(in.TenantId)
	if trimNonEmpty(in.Username) == "" || in.Password == "" || in.ClientFingerprint == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "用户名/密码/客户端指纹必填")
	}

	// 第一步：口令校验（账号不存在/停用/口令错统一 7008，不泄露细节）
	account, err := l.svcCtx.AccountsModel.FindOneByTenantIdUsername(l.ctx, tenantID, trimNonEmpty(in.Username))
	if err != nil && !repo.IsNotFound(err) {
		return nil, err
	}
	if repo.IsNotFound(err) || account.Status != 1 ||
		!l.svcCtx.SessionMgr.VerifyPassword(account.PasswordHash, in.Password) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleLoginInvalid, "用户名或密码错误")
	}

	// 本地调试：跳过 TOTP 直接签发
	if l.svcCtx.SkipTotp {
		return l.issueSession(account, in.ClientFingerprint)
	}

	// 第二步：票据驱动（有票据=提交验证码；无票据=发起 challenge）
	if in.LoginTicket != "" {
		return l.completeTotp(account, in)
	}
	return l.beginTotp(account, in)
}

// beginTotp 发起两步登录 challenge。
func (l *ConsoleLoginLogic) beginTotp(account *opsmodel.ConsoleAccounts, in *ops.ConsoleLoginRequest) (*ops.ConsoleLoginReply, error) {
	ticket, err := core.RandomToken(24)
	if err != nil {
		return nil, err
	}
	challenge := &ops.ConsoleTotpChallenge{LoginTicket: ticket}
	if account.TotpSecret == "" {
		// setup：生成密钥挂票据，验证通过后绑定
		secret, err := core.GenerateTotpSecret()
		if err != nil {
			return nil, err
		}
		if err = l.svcCtx.Tickets.Save(l.ctx, ticket, repo.LoginTicketPayload{
			TenantID: account.TenantId, AccountID: account.Id, Username: account.Username,
			PendingTotpSecret: secret,
		}); err != nil {
			return nil, err
		}
		challenge.SetupRequired = true
		challenge.TotpSecret = secret
		challenge.TotpUri = core.BuildTotpURI("Gacha Console", account.Username, secret)
	} else {
		// verify：票据只绑定账号，验证码对库中密钥校验
		if err = l.svcCtx.Tickets.Save(l.ctx, ticket, repo.LoginTicketPayload{
			TenantID: account.TenantId, AccountID: account.Id, Username: account.Username,
		}); err != nil {
			return nil, err
		}
	}
	return &ops.ConsoleLoginReply{TotpChallenge: challenge}, nil
}

// completeTotp 校验验证码并完成登录。
func (l *ConsoleLoginLogic) completeTotp(account *opsmodel.ConsoleAccounts, in *ops.ConsoleLoginRequest) (*ops.ConsoleLoginReply, error) {
	payload, err := l.svcCtx.Tickets.Get(l.ctx, in.LoginTicket)
	if err != nil {
		return nil, err
	}
	if payload == nil || payload.TenantID != account.TenantId || payload.AccountID != account.Id ||
		(trimNonEmpty(in.Username) != "" && payload.Username != trimNonEmpty(in.Username)) {
		return nil, errorx.NewCodeError(errorx.CodeConsoleLoginTicketInvalid, "登录票据无效或已过期")
	}
	if in.OtpCode == "" {
		return nil, errorx.NewCodeError(errorx.CodeParamError, "验证码必填")
	}

	if payload.PendingTotpSecret != "" {
		// setup 流程：验证码对票据中的待绑密钥校验，通过后绑定并一次性消费票据
		if !core.ValidateTotpCode(payload.PendingTotpSecret, in.OtpCode, nowTime()) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleTotpInvalid, "验证码无效")
		}
		if err = repo.NewAccountStore(l.svcCtx.DB).EnableTotp(l.ctx, account.TenantId, account.Id, payload.PendingTotpSecret); err != nil {
			return nil, err
		}
	} else {
		// verify 流程：验证码对账号已绑密钥校验
		if !core.ValidateTotpCode(account.TotpSecret, in.OtpCode, nowTime()) {
			return nil, errorx.NewCodeError(errorx.CodeConsoleTotpInvalid, "验证码无效")
		}
	}
	if err = l.svcCtx.Tickets.Delete(l.ctx, in.LoginTicket); err != nil {
		return nil, err
	}
	return l.issueSession(account, in.ClientFingerprint)
}

// issueSession 为账号签发会话令牌对（角色编码服务端实时解析）。
func (l *ConsoleLoginLogic) issueSession(account *opsmodel.ConsoleAccounts, fingerprint string) (*ops.ConsoleLoginReply, error) {
	store := repo.NewAccountStore(l.svcCtx.DB)
	full, err := store.FindByID(l.ctx, account.TenantId, account.Id)
	if err != nil {
		return nil, err
	}
	pair, err := l.svcCtx.SessionMgr.CreateSessionForAccount(l.ctx, full, fingerprint)
	if err == core.ErrInvalidCredentials {
		return nil, errorx.NewCodeError(errorx.CodeConsoleAccountDisabled, "账号已停用")
	}
	if err != nil {
		return nil, err
	}
	return &ops.ConsoleLoginReply{Session: toTokens(pair)}, nil
}
