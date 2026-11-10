// SQL 拆库工具：把旧仓 script/split/*.sql 按新服务库划分重切。
// 规则：
//   - 以「行尾分号」为语句块边界（mysqldump 终态导出格式）。
//   - CREATE TABLE 块按表名映射到新库；DML 块（INSERT/UPDATE/DELETE）按块内第一个表名映射。
//   - dtm_barrier 表定义复制到源库拆出的每个新库文件。
//   - 不拆库的文件原样复制。
// 用法：go run script/split_sql.go <旧仓script/split目录> <新仓script/split目录>
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type rule struct {
	match func(t string) bool
	dest  string
}

type srcSpec struct {
	file   string
	db     string
	split  bool // 是否拆库
	rules  []rule
	def    string // 兜底目标库
	desc   string
}

var srcs = []srcSpec{
	{"02_gacha_user.sql", "gacha_user", true, []rule{
		{func(t string) bool { return t == "user_users" || t == "user_phones" || t == "user_phone_grams" || t == "user_third_parties" }, "gacha_identity"},
	}, "gacha_money", "user"},
	{"03_gacha_draw.sql", "gacha_draw", false, nil, "", "draw"},
	{"04_gacha_dtm.sql", "gacha_dtm", false, nil, "", "dtm"},
	{"05_gacha_payment.sql", "gacha_payment", true, nil, "gacha_money", "payment"},
	{"06_gacha_base.sql", "gacha_base", false, nil, "", "base"},
	{"07_gacha_game.sql", "gacha_game", true, []rule{
		{func(t string) bool { return strings.HasPrefix(t, "campaign_") || t == "campaigns" }, "gacha_marketing"},
		{func(t string) bool { return strings.HasPrefix(t, "coupon_template_") || t == "coupon_templates" || t == "quote_locks" || t == "user_coupons" || t == "loyalty_offset_config" }, "gacha_marketing"},
		{func(t string) bool { return strings.HasPrefix(t, "task_") }, "gacha_task"},
	}, "gacha_play", "game"},
	{"08_gacha_catalog.sql", "gacha_catalog", false, nil, "", "catalog"},
	{"09_gacha_ops.sql", "gacha_ops", false, nil, "", "ops"},
	{"10_gacha_thirdparty.sql", "gacha_thirdparty", false, nil, "", "thirdparty"},
}

var fileNo = map[string]string{
	"gacha_identity": "02", "gacha_money": "03", "gacha_draw": "04",
	"gacha_base": "05", "gacha_marketing": "06", "gacha_play": "07",
	"gacha_task": "08", "gacha_catalog": "09", "gacha_thirdparty": "10",
	"gacha_ops": "11", "gacha_dtm": "12",
}

var libDesc = map[string]string{
	"gacha_identity": "账号域（用户/手机号/第三方身份 + dtm_barrier）",
	"gacha_money": "资金域（钱包/冻结/账本/余额/资产/支付单/退款单/支付路由 + dtm_barrier）",
	"gacha_draw": "抽奖域（宝箱/奖池条目/抽赏记录/吉仓 + dtm_barrier）",
	"gacha_base": "基础域（资格策略 + 风控规则/案例/事件 + dtm_barrier）",
	"gacha_marketing": "营销域（报价锁/券模板/用户券/忠诚度 + 活动/兑换码/签到/转盘/资格 + dtm_barrier）",
	"gacha_play": "玩法域（挖宝/满车/扭蛋/连击链/拼团/记忆翻牌/发奖标记 + dtm_barrier）",
	"gacha_task": "任务域（任务进度/事件/签到/通行证/会员 + dtm_barrier）",
	"gacha_catalog": "商品目录域（SPU/SKU/修订/发布 + dtm_barrier）",
	"gacha_thirdparty": "第三方域（provider 指标 + 腾讯广告点击）",
	"gacha_ops": "运营域（任务计划/控制台/权限/看板 + dtm_barrier）",
	"gacha_dtm": "DTM Server 存储库（kv/trans_branch_op/trans_global）",
}

var (
	createRe = regexp.MustCompile("CREATE TABLE[^`]*`(\\w+)`")
	dmlRe    = regexp.MustCompile(`(?i)(?:INSERT(?:\s+IGNORE)?\s+INTO|UPDATE|DELETE\s+FROM)\s+` + "`(\\w+)`")
)

// splitBlocks 按行尾分号切块，返回块文本与块内第一个表名（无表名语句返回空表名）。
func splitBlocks(text string) []struct{ text, table string } {
	var blocks []struct{ text, table string }
	var buf []string
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		buf = append(buf, line)
		if strings.HasSuffix(strings.TrimRight(line, " \t\r"), ";") {
			block := strings.Join(buf, "\n")
			buf = nil
			table := ""
			if m := createRe.FindStringSubmatch(block); m != nil {
				table = m[1]
			} else if m := dmlRe.FindStringSubmatch(block); m != nil {
				table = m[1]
			}
			blocks = append(blocks, struct{ text, table string }{block, table})
		}
	}
	if len(buf) > 0 {
		blocks = append(blocks, struct{ text, table string }{strings.Join(buf, "\n"), ""})
	}
	return blocks
}

func tableDest(s srcSpec, table string) string {
	for _, r := range s.rules {
		if r.match(table) {
			return r.dest
		}
	}
	return s.def
}

func main() {
	oldDir := "C:/Users/Administrator/Desktop/Gacha/script/split"
	newDir := "C:/Users/Administrator/Desktop/gacha_srv/script/split"
	if len(os.Args) > 1 {
		oldDir = os.Args[1]
	}
	if len(os.Args) > 2 {
		newDir = os.Args[2]
	}
	os.MkdirAll(newDir, 0o755)

	out := map[string][]string{} // dest db -> blocks
	seen := map[string]map[string]bool{}
	addBlock := func(dest, text string) {
		if seen[dest] == nil {
			seen[dest] = map[string]bool{}
		}
		if seen[dest][text] {
			return // 同库去重：多源（如 user+payment）各带的 dtm_barrier 只落一份
		}
		seen[dest][text] = true
		out[dest] = append(out[dest], text)
	}
	for _, s := range srcs {
		data, err := os.ReadFile(filepath.Join(oldDir, s.file))
		if err != nil {
			fmt.Println("读取失败:", s.file, err)
			os.Exit(1)
		}
		text := string(data)

		// 不拆库：整体复制
		if !s.split {
			no := fileNo[s.db]
			p := filepath.Join(newDir, no+"_"+s.db+".sql")
			if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
				fmt.Println("写出失败:", p, err)
				os.Exit(1)
			}
			fmt.Printf("复制 %s → %s_%s.sql\n", s.file, no, s.db)
			continue
		}

		// 拆库：逐块分发；dtm_barrier 复制到每个目标库
		splitDests := map[string]bool{s.def: true}
		for _, r := range s.rules {
			splitDests[r.dest] = true
		}
		for _, b := range splitBlocks(text) {
			if b.table == "" {
				continue // 杂项语句（USE/SET/注释）不搬
			}
			if b.table == "dtm_barrier" {
				for d := range splitDests {
					addBlock(d, b.text)
				}
				continue
			}
			addBlock(tableDest(s, b.table), b.text)
		}
	}

	// 拆库文件写出（目标库按编号排序）
	names := make([]string, 0, len(out))
	for d := range out {
		names = append(names, d)
	}
	sort.Strings(names)
	for _, dest := range names {
		blocks := out[dest]
		no := fileNo[dest]
		header := fmt.Sprintf(`-- ============================================================
-- %s_%s.sql - %s（%s 库终态快照）
-- 表结构 = 本机 MySQL 终态导出；种子 = 历史里程碑脚本按序归并（幂等，可重复执行）。
-- 本文件由 script/split_sql.go 从旧仓 script/split/ 拆库生成。
-- 执行方式：mysql -u root < script/split/%s_%s.sql
-- ============================================================

SET NAMES utf8mb4;

USE `+"`%s`"+`;

SET FOREIGN_KEY_CHECKS = 0;

`, no, dest, libDesc[dest], dest, no, dest, dest)
		body := strings.Join(blocks, "\n\n")
		p := filepath.Join(newDir, fmt.Sprintf("%s_%s.sql", no, dest))
		content := header + body + "\n\nSET FOREIGN_KEY_CHECKS = 1;\n"
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			fmt.Println("写出失败:", p, err)
			os.Exit(1)
		}
		creates := 0
		for _, b := range blocks {
			if strings.HasPrefix(b, "CREATE") {
				creates++
			}
		}
		fmt.Printf("生成 %s_%s.sql：%d 表 / %d 条种子语句\n", no, dest, creates, len(blocks)-creates)
	}
}
