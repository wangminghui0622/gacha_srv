package main

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func main() {
	body, err := os.ReadFile("script/split/13_gacha_report.sql")
	if err != nil { panic(err) }
	db, err := sql.Open("mysql", "root:we7root@tcp(192.168.197.128:3306)/?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true&timeout=10s")
	if err != nil { panic(err) }
	defer db.Close()
	if _, err := db.Exec(string(body)); err != nil { panic(err) }
	var n int64
	_ = db.QueryRow("SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA='gacha_report' AND TABLE_NAME='revenue_daily'").Scan(&n)
	fmt.Println("revenue_daily table:", n)
}
