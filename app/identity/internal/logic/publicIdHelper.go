package logic

import "gacha_srv/pkg/utils/publicid"

// publicUserIdOf 数字用户 ID → 对外字符串 ID；编码失败返回空串（展示层兜底）。
func publicUserIdOf(userID int64) string {
	return publicid.MustEncodeUserID(userID)
}
