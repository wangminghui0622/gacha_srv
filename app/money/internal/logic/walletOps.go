package logic

import (
	"context"
	"errors"
	"time"

	"gacha_srv/pkg/model_nocache/gacha_money"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ══════════════ 钱包公共操作（阶段2 M7）══════════════
// 资金一致性铁律沿用 v1（参考 dtm_shop payment 服务）：
// FOR UPDATE 行锁 + 原子盲更新 + 确定性 log_no 幂等；钱包表全 nocache。

// 钱包类型（wallet_accounts.wallet_kind）
const (
	walletKindCash         int64 = 1 // 现金
	walletKindBonus        int64 = 2 // 奖励金
	walletKindRedeemPoint  int64 = 3 // 兑换积分
	walletKindLoyaltyPoint int64 = 4 // 忠诚点（积分抵现）
)

// Hold 状态（wallet_holds.status）
const (
	holdStatusPending int64 = 1 // 已冻结
	holdStatusPosted  int64 = 2 // 已入账（捕获完成）
	holdStatusVoided  int64 = 3 // 已释放（撤单/补偿）
	holdStatusExpired int64 = 4 // 已过期
)

// 资产状态（assets.state）
const (
	assetStateAvailable int64 = 1 // 可用
	assetStateReserved  int64 = 2 // 已预留
	assetStateConsumed  int64 = 3 // 已消耗
	assetStateDelivered int64 = 4 // 已发放
	assetStateCanceled  int64 = 5 // 已撤销（SAGA 补偿）
)

// errWalletDuplicated 事务内哨兵：uk 撞键（同单号重复请求，先到者已入账）
var errWalletDuplicated = errors.New("duplicated wallet request")

// ensureWalletAccount 账户行不存在则惰性开户（INSERT IGNORE 无操作），
// 随后由调用方 FOR UPDATE 锁行。新用户只有迁移的 CASH 行，
// BONUS/积分等钱包在首次变动时开户。
func ensureWalletAccount(ctx context.Context, tx *gorm.DB, model gacha_money.WalletAccountsModel, userId, kind int64) error {
	err := model.Insert(ctx, tx, &gacha_money.WalletAccounts{
		UserId:     userId,
		WalletKind: kind,
		Currency:   walletCurrency(kind),
		Available:  0,
		Held:       0,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	if err != nil && !errors.Is(err, gorm.ErrDuplicatedKey) {
		return err
	}
	return nil
}

// lockWalletAccount FOR UPDATE 锁账户行。
// 铁律：事务持行锁期间不再走模型默认查库方法（会从连接池取新连接）。
func lockWalletAccount(tx *gorm.DB, userId, kind int64) (*gacha_money.WalletAccounts, error) {
	var wa gacha_money.WalletAccounts
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND wallet_kind = ?", userId, kind).First(&wa).Error; err != nil {
		return nil, err
	}
	return &wa, nil
}

// walletCurrency CASH 用 CNY，积分类用 POINT
func walletCurrency(kind int64) string {
	if kind == walletKindCash {
		return "CNY"
	}
	return "POINT"
}

// creditWalletTx 在既有事务内执行钱包入账（CreditWallet/Recharge 复用）：
// 先插流水（uk_log_no 兜底并发重复请求）→ 惰性开户 → 锁行 → available 盲加。
// 返回 errWalletDuplicated 表示同单号已入账（先到者），调用方幂等返回当前余额。
func creditWalletTx(ctx context.Context, db *gorm.DB, accountModel gacha_money.WalletAccountsModel,
	ledgerModel gacha_money.WalletLedgersModel, userId, kind, fen int64, logNo, bizType, bizId string) error {
	if err := ledgerModel.Insert(ctx, db, &gacha_money.WalletLedgers{
		LogNo:      logNo,
		UserId:     userId,
		WalletKind: kind,
		Amount:     fen,
		BizType:    bizType,
		BizId:      bizId,
		Status:     "success",
		CreatedAt:  time.Now(),
	}); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return errWalletDuplicated
		}
		return err
	}
	if err := ensureWalletAccount(ctx, db, accountModel, userId, kind); err != nil {
		return err
	}
	if _, err := lockWalletAccount(db, userId, kind); err != nil {
		return err
	}
	result := db.Model(&gacha_money.WalletAccounts{}).
		Where("user_id = ? AND wallet_kind = ?", userId, kind).
		UpdateColumn("available", gorm.Expr("available + ?", fen))
	return result.Error
}
