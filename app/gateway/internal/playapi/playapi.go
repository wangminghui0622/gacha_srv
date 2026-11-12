// Package playapi 网关侧通用 play RPC 封装：
// 把 PlayListRecords 的 payload 反序列化成各玩法 typed 响应，避免每个调用点重复。
package playapi

import (
	"context"

	"gacha_srv/app/gateway/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/protobuf/proto"
)

func list(ctx context.Context, svcCtx *svc.ServiceContext, mode play.PlayMode,
	requestID string, userID, page, pageSize int64, out proto.Message) error {
	resp, err := svcCtx.PlayRpc.PlayListRecords(ctx, &play.PlayListRecordsReq{
		Mode:      mode,
		RequestId: requestID,
		UserId:    userID,
		Page:      int32(page),
		PageSize:  int32(pageSize),
	})
	if err != nil {
		return err
	}
	if len(resp.GetPayload()) == 0 {
		return nil
	}
	return proto.Unmarshal(resp.GetPayload(), out)
}

func ListGashaponRecords(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID, page, pageSize int64) (*play.ListGashaponRecordsResp, error) {
	out := &play.ListGashaponRecordsResp{}
	if err := list(ctx, svcCtx, play.PlayMode_PLAY_MODE_GASHAPON, requestID, userID, page, pageSize, out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListFullCarRecords(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID, page, pageSize int64) (*play.ListFullCarRecordsResp, error) {
	out := &play.ListFullCarRecordsResp{}
	if err := list(ctx, svcCtx, play.PlayMode_PLAY_MODE_FULL_CAR, requestID, userID, page, pageSize, out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListComboChainRecords(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID, page, pageSize int64) (*play.ListComboChainRecordsResp, error) {
	out := &play.ListComboChainRecordsResp{}
	if err := list(ctx, svcCtx, play.PlayMode_PLAY_MODE_COMBO_CHAIN, requestID, userID, page, pageSize, out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListDigRecords(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID, page, pageSize int64) (*play.ListDigRecordsResp, error) {
	out := &play.ListDigRecordsResp{}
	if err := list(ctx, svcCtx, play.PlayMode_PLAY_MODE_DIG_BOX, requestID, userID, page, pageSize, out); err != nil {
		return nil, err
	}
	return out, nil
}

func ListGroupBuyRecords(ctx context.Context, svcCtx *svc.ServiceContext, requestID string, userID, page, pageSize int64) (*play.ListGroupBuyRecordsResp, error) {
	out := &play.ListGroupBuyRecordsResp{}
	if err := list(ctx, svcCtx, play.PlayMode_PLAY_MODE_GROUP_BUY, requestID, userID, page, pageSize, out); err != nil {
		return nil, err
	}
	return out, nil
}
