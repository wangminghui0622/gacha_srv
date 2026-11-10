package core

import (
	"crypto/sha256"
	"fmt"
	"strings"

	"gacha_srv/app/draw/draw"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// M14b 赏池草稿/版本/发布（pool console）。
// 语义移植自 server_v2 gacha 服务池管理（biz/pool.go + data/pool.go）：
//   - config_hash = SHA256(确定性 proto 序列化，密封后含 version)；
//   - request_hash（幂等键校验）= SHA256(未密封草稿，version=0 无 hash)；
//   - 发布前置：版本 DRAFT + hash 一致 + 审批行 APPROVED 且 approver≠requester；
//   - 已发布版本再发布 → 幂等返回。
// 差异（本仓模型适配，接口形状不变）：pool_id=0 的草稿由服务端分配新池 id
// （draw_pool_heads AUTO_INCREMENT 1000000+），server_v2 由调用方携带非零 pool_id。

var (
	deterministicMarshal = proto.MarshalOptions{Deterministic: true}
	draftJSONMarshal     = protojson.MarshalOptions{UseProtoNames: false, EmitUnpopulated: false}
	draftJSONUnmarshal   = protojson.UnmarshalOptions{DiscardUnknown: true}
)

// ValidateDraft 草稿字段校验（server_v2 CreateDraft 校验的同构子集，按本仓
// draw_boxes/draw_pool_entries 模型）。返回中文错误信息；通过返回 nil。
func ValidateDraft(box *draw.BoxDraft) error {
	if box == nil {
		return fmt.Errorf("草稿配置不能为空")
	}
	if strings.TrimSpace(box.BoxName) == "" || len(box.BoxName) > 64 {
		return fmt.Errorf("宝箱名称必填且不超过 64 字符")
	}
	if box.Price < 0 {
		return fmt.Errorf("单抽价格不能为负")
	}
	if len(box.Entries) == 0 {
		return fmt.Errorf("奖池条目不能为空")
	}
	totalWeight := int64(0)
	for _, e := range box.Entries {
		if e == nil || strings.TrimSpace(e.Name) == "" || len(e.Name) > 64 {
			return fmt.Errorf("条目名称必填且不超过 64 字符")
		}
		if e.Weight < 0 {
			return fmt.Errorf("条目 %s 权重不能为负", e.Name)
		}
		if e.Quantity == 0 || e.Quantity < -1 {
			return fmt.Errorf("条目 %s 库存非法：0 或小于 -1", e.Name)
		}
		if e.DisplayValueFen < 0 {
			return fmt.Errorf("条目 %s 展示价不能为负", e.Name)
		}
		totalWeight += int64(e.Weight)
	}
	if totalWeight == 0 {
		return fmt.Errorf("条目权重合计不能为 0")
	}
	// 滚动规则：非空则必须可解析（percent 越界同 M12 ParseRollingRules）
	if _, err := ParseRollingRules(box.RollingRulesJson); err != nil {
		return err
	}
	return nil
}

// DraftRequestHash 幂等键载荷哈希：未密封草稿（version=0、status 统一 DRAFT、
// config_hash 空、published_at 清零）的确定性 SHA256。同 Idempotency-Key
// 重放时比对：相同 → 返回既有草稿；不同 → 幂等键冲突。
func DraftRequestHash(box *draw.BoxDraft) ([]byte, error) {
	clone := proto.Clone(box).(*draw.BoxDraft)
	clone.Version = 0
	clone.Status = draw.PoolStatus_POOL_STATUS_DRAFT
	clone.ConfigHash = nil
	clone.PublishedAtUnixMillis = 0
	blob, err := deterministicMarshal.Marshal(clone)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(blob)
	return sum[:], nil
}

// SealPoolDraft 密封草稿：写入版本号 + 计算 config_hash。
// 前置：version>0、status=DRAFT、config_hash 为空（与 server_v2 SealPoolDraft 同构）。
func SealPoolDraft(box *draw.BoxDraft, version int64) error {
	if box == nil || version <= 0 || box.Version != 0 ||
		box.Status != draw.PoolStatus_POOL_STATUS_DRAFT || len(box.ConfigHash) != 0 {
		return fmt.Errorf("草稿密封前置不满足")
	}
	box.Version = version
	blob, err := deterministicMarshal.Marshal(box)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(blob)
	box.ConfigHash = sum[:]
	return nil
}

// MarshalDraft 草稿 → 版本行 config_json（protojson，与接口线格式一致）。
func MarshalDraft(box *draw.BoxDraft) (string, error) {
	blob, err := draftJSONMarshal.Marshal(box)
	if err != nil {
		return "", err
	}
	return string(blob), nil
}

// UnmarshalDraft config_json → 草稿。
func UnmarshalDraft(blob string) (*draw.BoxDraft, error) {
	box := &draw.BoxDraft{}
	if err := draftJSONUnmarshal.Unmarshal([]byte(blob), box); err != nil {
		return nil, err
	}
	return box, nil
}
