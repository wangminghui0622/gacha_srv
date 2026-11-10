// Package txutil 提供 barrier 事务内使用 GORM 的辅助函数
package txutil

import (
	"context"
	"database/sql"

	"gorm.io/gorm"
)

// WithTx 在 barrier 事务(tx)内创建 GORM 会话。
//
// 用法：
//
//	barrier.CallWithDB(rawDB, func(tx *sql.Tx) error {
//	    db := txutil.WithTx(l.ctx, l.svcCtx.DB, tx)
//	    ...
//	})
//
// 两个要点缺一不可：
//  1. Session(NewDB: true) 克隆独立会话，避免给共享的连接池句柄挂上事务状态；
//  2. 把 ConnPool 替换成 barrier 的 *sql.Tx，后续所有 GORM 操作都走该事务。
//
// 注意：事务连接持有行锁期间，不要再通过模型默认方法查库——
// 那些方法走连接池取新连接，高并发下会把连接池耗尽。
//
// Statement.ConnPool 是 gorm 未导出的内部字段，属社区标准 workaround；
// go.mod 已 pin gorm v1.31.2，升级 gorm 大版本前必须回归本函数的单测。
func WithTx(ctx context.Context, db *gorm.DB, tx *sql.Tx) *gorm.DB {
	gormTx := db.WithContext(ctx).Session(&gorm.Session{NewDB: true})
	gormTx.Statement.ConnPool = tx
	return gormTx
}
