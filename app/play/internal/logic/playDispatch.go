package logic

import (
	"context"

	"gacha_srv/app/play/internal/gameimpl"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// ══════════════ 通用 SAGA 分支分发 ═══════════════
// payload = 对应 PlayMode 的 typed 分支请求（proto 序列化）。
// 这里只做反序列化 + 公共字段覆盖 + 调用 gameimpl 原有实现。

func payloadErr(err error) error {
	return status.Error(codes.Aborted, "code=400 msg=payload 解析失败: "+err.Error())
}

func unmarshalPayload(data []byte, msg proto.Message) error {
	if len(data) == 0 {
		return nil
	}
	return proto.Unmarshal(data, msg)
}

func marshalPayload(msg proto.Message) []byte {
	if msg == nil {
		return nil
	}
	b, _ := proto.Marshal(msg)
	return b
}

func doPlayLock(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PlayLockReq) (*play.PlayLockResp, error) {
	switch in.Mode {
	case play.PlayMode_PLAY_MODE_GASHAPON:
		req := &play.LockGashaponReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.Count, req.TotalPriceFen = in.UserId, in.Count, in.TotalPriceFen
		resp, err := gameimpl.NewLockGashaponLogic(ctx, svcCtx).LockGashapon(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_FULL_CAR:
		req := &play.LockFullCarReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewLockFullCarLogic(ctx, svcCtx).LockFullCar(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_COMBO_CHAIN:
		req := &play.LockComboChainReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewLockComboChainLogic(ctx, svcCtx).LockComboChain(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_DIG_BOX:
		req := &play.LockDigBoxReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.TotalPriceFen = in.TotalPriceFen
		resp, err := gameimpl.NewLockDigBoxLogic(ctx, svcCtx).LockDigBox(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_GROUP_BUY:
		req := &play.LockGroupBuyReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.Count, req.TotalPriceFen = in.UserId, in.Count, in.TotalPriceFen
		resp, err := gameimpl.NewLockGroupBuyLogic(ctx, svcCtx).LockGroupBuy(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_MEMORY_MATCH:
		req := &play.LockMemoryMatchReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewLockMemoryMatchLogic(ctx, svcCtx).LockMemoryMatch(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayLockResp{Payload: marshalPayload(resp)}, nil
	}
	return nil, status.Error(codes.Aborted, "code=400 msg=未知玩法")
}

func doPlayRelease(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PlayLockReq) (*play.PlayReleaseResp, error) {
	switch in.Mode {
	case play.PlayMode_PLAY_MODE_GASHAPON:
		req := &play.ReleaseGashaponReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewReleaseGashaponLogic(ctx, svcCtx).ReleaseGashapon(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_FULL_CAR:
		req := &play.ReleaseFullCarReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewReleaseFullCarLogic(ctx, svcCtx).ReleaseFullCar(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_COMBO_CHAIN:
		req := &play.ReleaseComboChainReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewReleaseComboChainLogic(ctx, svcCtx).ReleaseComboChain(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_DIG_BOX:
		req := &play.ReleaseDigBoxReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		resp, err := gameimpl.NewReleaseDigBoxLogic(ctx, svcCtx).ReleaseDigBox(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_GROUP_BUY:
		req := &play.ReleaseGroupBuyReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewReleaseGroupBuyLogic(ctx, svcCtx).ReleaseGroupBuy(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_MEMORY_MATCH:
		req := &play.LockMemoryMatchReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.QuoteToken, req.ConfigId = in.RequestId, in.QuoteToken, in.ConfigId
		req.UserId, req.TotalPriceFen = in.UserId, in.TotalPriceFen
		resp, err := gameimpl.NewLockMemoryMatchRevertLogic(ctx, svcCtx).LockMemoryMatchRevert(req)
		if err != nil {
			return nil, err
		}
		return &play.PlayReleaseResp{Payload: marshalPayload(resp)}, nil
	}
	return nil, status.Error(codes.Aborted, "code=400 msg=未知玩法")
}

func doPlaySettle(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PlaySettleReq, revert bool) (*play.PlaySettleResp, error) {
	switch in.Mode {
	case play.PlayMode_PLAY_MODE_GASHAPON:
		req := &play.SettleGashaponReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		req.Count, req.Price = in.Count, in.Price
		var resp *play.SettleGashaponResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleGashaponRevertLogic(ctx, svcCtx).SettleGashaponRevert(req)
		} else {
			resp, err = gameimpl.NewSettleGashaponLogic(ctx, svcCtx).SettleGashapon(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_FULL_CAR:
		req := &play.SettleFullCarReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		req.Price = in.Price
		var resp *play.SettleFullCarResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleFullCarRevertLogic(ctx, svcCtx).SettleFullCarRevert(req)
		} else {
			resp, err = gameimpl.NewSettleFullCarLogic(ctx, svcCtx).SettleFullCar(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_COMBO_CHAIN:
		req := &play.SettleComboChainReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		req.Price = in.Price
		var resp *play.SettleComboChainResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleComboChainRevertLogic(ctx, svcCtx).SettleComboChainRevert(req)
		} else {
			resp, err = gameimpl.NewSettleComboChainLogic(ctx, svcCtx).SettleComboChain(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_DIG_BOX:
		req := &play.SettleDigBoxReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		req.Price = in.Price
		var resp *play.SettleDigBoxResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleDigBoxRevertLogic(ctx, svcCtx).SettleDigBoxRevert(req)
		} else {
			resp, err = gameimpl.NewSettleDigBoxLogic(ctx, svcCtx).SettleDigBox(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_GROUP_BUY:
		req := &play.SettleGroupBuyReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		req.Count, req.Price = in.Count, in.Price
		var resp *play.SettleGroupBuyResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleGroupBuyRevertLogic(ctx, svcCtx).SettleGroupBuyRevert(req)
		} else {
			resp, err = gameimpl.NewSettleGroupBuyLogic(ctx, svcCtx).SettleGroupBuy(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_MEMORY_MATCH:
		req := &play.SettleMemoryMatchReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.SettleMemoryMatchResp
		var err error
		if revert {
			resp, err = gameimpl.NewSettleMemoryMatchRevertLogic(ctx, svcCtx).SettleMemoryMatchRevert(req)
		} else {
			resp, err = gameimpl.NewSettleMemoryMatchLogic(ctx, svcCtx).SettleMemoryMatch(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlaySettleResp{Payload: marshalPayload(resp)}, nil
	}
	return nil, status.Error(codes.Aborted, "code=400 msg=未知玩法")
}

func doPlayGrant(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PlayGrantReq, revert bool) (*play.PlayGrantResp, error) {
	switch in.Mode {
	case play.PlayMode_PLAY_MODE_GASHAPON:
		req := &play.GrantGashaponPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.GrantGashaponPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantGashaponPrizesRevertLogic(ctx, svcCtx).GrantGashaponPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantGashaponPrizesLogic(ctx, svcCtx).GrantGashaponPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_FULL_CAR:
		req := &play.GrantFullCarPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.GrantFullCarPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantFullCarPrizesRevertLogic(ctx, svcCtx).GrantFullCarPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantFullCarPrizesLogic(ctx, svcCtx).GrantFullCarPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_COMBO_CHAIN:
		req := &play.GrantComboChainPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.GrantComboChainPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantComboChainPrizesRevertLogic(ctx, svcCtx).GrantComboChainPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantComboChainPrizesLogic(ctx, svcCtx).GrantComboChainPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_DIG_BOX:
		req := &play.GrantDigPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId = in.RequestId, in.UserId
		var resp *play.GrantDigPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantDigPrizesRevertLogic(ctx, svcCtx).GrantDigPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantDigPrizesLogic(ctx, svcCtx).GrantDigPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_GROUP_BUY:
		req := &play.GrantGroupBuyPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.GrantGroupBuyPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantGroupBuyPrizesRevertLogic(ctx, svcCtx).GrantGroupBuyPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantGroupBuyPrizesLogic(ctx, svcCtx).GrantGroupBuyPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil

	case play.PlayMode_PLAY_MODE_MEMORY_MATCH:
		req := &play.GrantMemoryMatchPrizesReq{}
		if err := unmarshalPayload(in.Payload, req); err != nil {
			return nil, payloadErr(err)
		}
		req.RequestId, req.UserId, req.ConfigId = in.RequestId, in.UserId, in.ConfigId
		var resp *play.GrantMemoryMatchPrizesResp
		var err error
		if revert {
			resp, err = gameimpl.NewGrantMemoryMatchPrizesRevertLogic(ctx, svcCtx).GrantMemoryMatchPrizesRevert(req)
		} else {
			resp, err = gameimpl.NewGrantMemoryMatchPrizesLogic(ctx, svcCtx).GrantMemoryMatchPrizes(req)
		}
		if err != nil {
			return nil, err
		}
		return &play.PlayGrantResp{Payload: marshalPayload(resp)}, nil
	}
	return nil, status.Error(codes.Aborted, "code=400 msg=未知玩法")
}

func doPlayListRecords(ctx context.Context, svcCtx *svc.ServiceContext, in *play.PlayListRecordsReq) (*play.PlayListRecordsResp, error) {
	switch in.Mode {
	case play.PlayMode_PLAY_MODE_GASHAPON:
		resp, err := gameimpl.NewListGashaponRecordsLogic(ctx, svcCtx).ListGashaponRecords(&play.ListGashaponRecordsReq{
			RequestId: in.RequestId, UserId: in.UserId, Page: int64(in.Page), PageSize: int64(in.PageSize),
		})
		if err != nil {
			return nil, err
		}
		return &play.PlayListRecordsResp{Payload: marshalPayload(resp)}, nil
	case play.PlayMode_PLAY_MODE_FULL_CAR:
		resp, err := gameimpl.NewListFullCarRecordsLogic(ctx, svcCtx).ListFullCarRecords(&play.ListFullCarRecordsReq{
			RequestId: in.RequestId, UserId: in.UserId, Page: int64(in.Page), PageSize: int64(in.PageSize),
		})
		if err != nil {
			return nil, err
		}
		return &play.PlayListRecordsResp{Payload: marshalPayload(resp)}, nil
	case play.PlayMode_PLAY_MODE_COMBO_CHAIN:
		resp, err := gameimpl.NewListComboChainRecordsLogic(ctx, svcCtx).ListComboChainRecords(&play.ListComboChainRecordsReq{
			RequestId: in.RequestId, UserId: in.UserId, Page: int64(in.Page), PageSize: int64(in.PageSize),
		})
		if err != nil {
			return nil, err
		}
		return &play.PlayListRecordsResp{Payload: marshalPayload(resp)}, nil
	case play.PlayMode_PLAY_MODE_DIG_BOX:
		resp, err := gameimpl.NewListDigRecordsLogic(ctx, svcCtx).ListDigRecords(&play.ListDigRecordsReq{
			RequestId: in.RequestId, UserId: in.UserId, Page: int64(in.Page), PageSize: int64(in.PageSize),
		})
		if err != nil {
			return nil, err
		}
		return &play.PlayListRecordsResp{Payload: marshalPayload(resp)}, nil
	case play.PlayMode_PLAY_MODE_GROUP_BUY:
		resp, err := gameimpl.NewListGroupBuyRecordsLogic(ctx, svcCtx).ListGroupBuyRecords(&play.ListGroupBuyRecordsReq{
			RequestId: in.RequestId, UserId: in.UserId, Page: int64(in.Page), PageSize: int64(in.PageSize),
		})
		if err != nil {
			return nil, err
		}
		return &play.PlayListRecordsResp{Payload: marshalPayload(resp)}, nil
	}
	return &play.PlayListRecordsResp{}, nil
}
