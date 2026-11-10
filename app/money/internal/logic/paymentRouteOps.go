package logic

import (
	"gacha_srv/app/money/internal/repo"
	"gacha_srv/app/money/money"

	"google.golang.org/protobuf/encoding/protojson"
)

// policyRecordOf 策略行 → proto
func policyRecordOf(row *repo.PaymentRoutePolicyRow) *money.PaymentRoutePolicy {
	return &money.PaymentRoutePolicy{
		PolicyId:                uint64(row.ID),
		TenantId:                uint64(row.TenantID),
		Channel:                 money.PaymentChannel(row.Channel),
		Scene:                   money.PaymentScene(row.Scene),
		Mode:                    money.PaymentRouteMode(row.Mode),
		PinnedProvider:          money.PaymentProvider(row.PinnedProvider),
		PinnedMerchantAccountId: row.PinnedMerchantAccountID,
		Version:                 uint64(row.Version),
	}
}

// candidateRecordsOf 候选行列表 → proto
func candidateRecordsOf(rows []repo.PaymentRouteCandidateRow) []*money.PaymentRouteCandidate {
	out := make([]*money.PaymentRouteCandidate, 0, len(rows))
	for _, c := range rows {
		out = append(out, &money.PaymentRouteCandidate{
			CandidateId:       uint64(c.ID),
			PolicyId:          uint64(c.PolicyID),
			Provider:          money.PaymentProvider(c.Provider),
			MerchantAccountId: c.MerchantAccountID,
			Priority:          uint32(c.Priority),
			Weight:            uint32(c.Weight),
			Enabled:           c.Enabled,
		})
	}
	return out
}

// routeReplyOf 策略行+候选行 → Get/Put 回复
func routeReplyOf(row *repo.PaymentRoutePolicyRow, rows []repo.PaymentRouteCandidateRow) *money.GetPaymentRouteReply {
	return &money.GetPaymentRouteReply{
		Policy:     policyRecordOf(row),
		Candidates: candidateRecordsOf(rows),
	}
}

// snapshotOf 审计快照：policy+candidates 整包 protojson（server_v2 old/new_snapshot 同义，
// 本仓快照统一 protojson 编码——catalog repo 先例）。
func snapshotOf(row *repo.PaymentRoutePolicyRow, rows []repo.PaymentRouteCandidateRow) (string, error) {
	blob, err := protojson.Marshal(routeReplyOf(row, rows))
	if err != nil {
		return "", err
	}
	return string(blob), nil
}
