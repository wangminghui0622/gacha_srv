package svc

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"time"

	"gacha_srv/app/ops/internal/config"
	"gacha_srv/app/ops/internal/core"
	"gacha_srv/app/ops/internal/repo"
	opsmodel "gacha_srv/pkg/model_nocache/gacha_ops"

	"github.com/redis/go-redis/v9"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/proc"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type ServiceContext struct {
	Config config.Config
	DB     *gorm.DB
	Rdb    *redis.Client

	AccountsModel opsmodel.ConsoleAccountsModel
	SessionsModel opsmodel.ConsoleSessionsModel
	RolesModel    opsmodel.RolesModel
	RolePermModel opsmodel.RolePermissionsModel
	PrRolesModel  opsmodel.PrincipalRolesModel
	AuditModel    opsmodel.OpsAuditLogsModel

	SessionMgr  *core.SessionManager
	Authz       *core.AuthzEvaluator
	AuthzStore  *repo.AuthzStore
	Tickets     *repo.LoginTicketStore
	Revocations *repo.RevocationPublisher
	TabStore    *repo.TabStore

	// M14e 任务计划/报表存储与结果缓存
	PlanStore      *repo.PlanStore
	ReportingStore *repo.ReportingStore
	ResultCache    *repo.ResultCache

	// SkipTotp 跳过 TOTP（仅本地开发调试）
	SkipTotp bool
}

func NewServiceContext(c config.Config) *ServiceContext {
	db, err := gorm.Open(mysql.Open(c.MySQL.DataSource), &gorm.Config{
		// 开启错误翻译（1062 → gorm.ErrDuplicatedKey）
		TranslateError: true,
	})
	if err != nil {
		panic("failed to connect database: " + err.Error())
	}
	sqlDB, err := db.DB()
	if err != nil {
		panic("failed to get underlying sql.DB: " + err.Error())
	}
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetMaxIdleConns(20)
	sqlDB.SetConnMaxLifetime(time.Hour)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	proc.AddShutdownListener(func() {
		if err := sqlDB.Close(); err != nil {
			logx.Errorf("close sql.DB failed: %v", err)
		}
	})

	// 业务 Redis（登录票据/会话吊销键；模型全 nocache 不走模型缓存）
	if len(c.CacheRedis) == 0 {
		panic("ops-rpc 需要配置 CacheRedis（登录票据/会话吊销键）")
	}
	rdb := redis.NewClient(&redis.Options{
		Addr:     c.CacheRedis[0].Host,
		Password: c.CacheRedis[0].Pass,
		DB:       0,
	})
	proc.AddShutdownListener(func() { _ = rdb.Close() })

	// Ed25519 私钥（签发控制台 JWT；公钥由管理网关持有本地验签）
	priv, err := base64.StdEncoding.DecodeString(c.Console.PrivateKeyBase64)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		panic("Console.PrivateKeyBase64 无效：需要 32 字节 Ed25519 私钥（base64）")
	}

	accessTTL := time.Duration(c.Console.AccessTtlSeconds) * time.Second
	refreshTTL := time.Duration(c.Console.RefreshTtlSeconds) * time.Second
	accounts := repo.NewAccountStore(db)
	sessions := repo.NewSessionStore(db)
	revocations := repo.NewRevocationPublisher(rdb)
	sessionMgr, err := core.NewSessionManager(accounts, sessions, core.DefaultArgon2idHasher(),
		ed25519.PrivateKey(priv), c.Console.Issuer, c.Console.Audience, accessTTL, refreshTTL)
	if err != nil {
		panic("会话管理器初始化失败: " + err.Error())
	}
	sessionMgr.WithRevocationPublisher(revocations)
	authz, err := core.NewAuthzEvaluator()
	if err != nil {
		panic("授权决策器初始化失败: " + err.Error())
	}

	ctx := &ServiceContext{
		Config: c,
		DB:     db,
		Rdb:    rdb,

		AccountsModel: opsmodel.NewConsoleAccountsModel(db),
		SessionsModel: opsmodel.NewConsoleSessionsModel(db),
		RolesModel:    opsmodel.NewRolesModel(db),
		RolePermModel: opsmodel.NewRolePermissionsModel(db),
		PrRolesModel:  opsmodel.NewPrincipalRolesModel(db),
		AuditModel:    opsmodel.NewOpsAuditLogsModel(db),

		SessionMgr:  sessionMgr,
		Authz:       authz,
		AuthzStore:  repo.NewAuthzStore(db),
		Tickets:     repo.NewLoginTicketStore(rdb),
		Revocations: revocations,
		TabStore:    repo.NewTabStore(db),

		PlanStore:      repo.NewPlanStore(db),
		ReportingStore: repo.NewReportingStore(db),
		ResultCache:    repo.NewResultCache(rdb, 0),

		SkipTotp: c.Console.SkipTotp,
	}

	// 首次启动自动开通超管（角色 + 通配权限 + 账号 + 绑定，幂等）
	if err := ctx.provisionInitialAdmin(); err != nil {
		panic("初始管理员开通失败: " + err.Error())
	}
	// 任务计划种子 v1（已发布，幂等；SQL 种子与运行时同源）
	if err := ctx.PlanStore.EnsureBootstrapPlan(context.Background()); err != nil {
		panic("任务计划种子初始化失败: " + err.Error())
	}
	return ctx
}

// provisionInitialAdmin 幂等开通初始超管（server_v2 console provisioning 语义：
// 角色/权限/账号/绑定 缺一补一，存在则跳过）。
func (s *ServiceContext) provisionInitialAdmin() error {
	conf := s.Config.Console
	if conf.InitialAdminUsername == "" || conf.InitialAdminRoleCode == "" {
		return nil
	}
	ctx := context.Background()
	tenantID := uint64(1)

	// 1. 超管角色
	role, err := s.RolesModel.FindOneByTenantIdCode(ctx, tenantID, conf.InitialAdminRoleCode)
	if repo.IsNotFound(err) {
		if err = s.RolesModel.Insert(ctx, nil, &opsmodel.Roles{
			TenantId: tenantID, Code: conf.InitialAdminRoleCode, Name: conf.InitialAdminDisplayName,
			Enabled: 1, Version: 1, CreatedBy: 0,
		}); err != nil {
			return err
		}
		role, err = s.RolesModel.FindOneByTenantIdCode(ctx, tenantID, conf.InitialAdminRoleCode)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// 2. 通配权限（resource/action = *，数据范围 ALL）
	_, err = s.RolePermModel.FindOneByRoleIdResourceAction(ctx, role.Id, "*", "*")
	if repo.IsNotFound(err) {
		err = s.RolePermModel.Insert(ctx, nil, &opsmodel.RolePermissions{
			RoleId: role.Id, Resource: "*", Action: "*", DataScope: 6, ApprovalMode: 1,
			ConditionExpr: "", AllowedFields: "[]", DeniedFields: "[]",
		})
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// 3. 超管账号（口令 Argon2id，同 server_v2 参数）
	account, err := s.AccountsModel.FindOneByTenantIdUsername(ctx, tenantID, conf.InitialAdminUsername)
	if repo.IsNotFound(err) {
		hash, hashErr := core.DefaultArgon2idHasher().Hash(conf.InitialAdminPassword)
		if hashErr != nil {
			return hashErr
		}
		if err = s.AccountsModel.Insert(ctx, nil, &opsmodel.ConsoleAccounts{
			TenantId: tenantID, Username: conf.InitialAdminUsername, PasswordHash: hash,
			DisplayName: conf.InitialAdminDisplayName, Status: 1, RoleVersion: 1, TotpSecret: "",
		}); err != nil {
			return err
		}
		account, err = s.AccountsModel.FindOneByTenantIdUsername(ctx, tenantID, conf.InitialAdminUsername)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	// 4. 账号 ↔ 角色绑定
	_, err = s.PrRolesModel.FindOneByAccountIdRoleId(ctx, account.Id, role.Id)
	if repo.IsNotFound(err) {
		if err = s.PrRolesModel.Insert(ctx, nil, &opsmodel.PrincipalRoles{
			AccountId: account.Id, RoleId: role.Id,
		}); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	logx.Infof("初始管理员就绪：%s@%d（角色 %s）", conf.InitialAdminUsername, tenantID, conf.InitialAdminRoleCode)
	return nil
}
