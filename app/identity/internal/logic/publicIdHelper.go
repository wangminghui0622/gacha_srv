package logic

import (
	"gacha_srv/pkg/model_nocache/gacha_identity"
	"gacha_srv/pkg/utils/publicid"
)

// publicUserIdOf 数字用户 ID → 对外字符串 ID；编码失败返回空串（展示层兜底）。
func publicUserIdOf(userID int64) string {
	return publicid.MustEncodeUserID(userID)
}

// publicUserIdOfUser 优先返回库中已持久化的 public_id；
// 历史数据尚未回填时回退到数字 ID 编码。
func publicUserIdOfUser(u *gacha_identity.UserUsers) string {
	if u == nil {
		return ""
	}
	if u.PublicId != nil && *u.PublicId != "" {
		return *u.PublicId
	}
	return publicUserIdOf(u.Id)
}
