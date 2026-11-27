package gameimpl

import (
	"context"
	"errors"

	"gacha_srv/app/play/internal/playpool"
	"gacha_srv/app/play/internal/svc"
	"gacha_srv/app/play/play"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

// CreatePlayPoolDraft 创建/更新玩法池。名称保留是为了兼容旧接口；
// 新模型没有版本：poolId=0 新建，poolId>0 更新当前配置（仅 DRAFT/OFFLINE 可改）。
func CreatePlayPoolDraft(ctx context.Context, svcCtx *svc.ServiceContext, in *play.CreatePlayPoolDraftReq) (*play.CreatePlayPoolDraftResp, error) {
	if err := validatePlayPoolDraft(in); err != nil {
		return nil, err
	}
	configJSON := in.ConfigJson
	if configJSON == "" {
		configJSON = "{}"
	}
	adminJSON := in.AdminJson
	if adminJSON == "" {
		adminJSON = "{}"
	}
	tagsJSON, err := encodePoolTags(in.Tags)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "标签格式错误")
	}
	hash := hashPlayPoolDraft(in)

	var outPoolID int64
	err = svcCtx.DB.Transaction(func(tx *gorm.DB) error {
		if in.IdempotencyKey != "" {
			req, err := playpool.GetRequest(tx, in.IdempotencyKey)
			if err == nil {
				if req.RequestHash != hash {
					return status.Error(codes.InvalidArgument, "幂等键冲突")
				}
				outPoolID = req.PoolID
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}

		pool := &playpool.Pool{
			Mode:       in.Mode,
			Name:       in.Name,
			PriceFen:   in.PriceFen,
			IPID:       int64(in.IpId),
			TagsJSON:   tagsJSON,
			Weight:     int(in.Weight),
			AdminJSON:  adminJSON,
			ConfigJSON: configJSON,
			Prizes:     playPoolPrizesToDomain(in.Mode, in.Prizes),
		}
		if in.PoolId > 0 {
			existing, err := playpool.Get(tx, in.PoolId)
			if err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return status.Error(codes.InvalidArgument, "赏池不存在")
				}
				return err
			}
			if existing.Mode != in.Mode {
				return status.Error(codes.InvalidArgument, "赏池玩法不一致")
			}
			pool.ID = in.PoolId
			pool.Status = existing.Status
			if err := playpool.Update(tx, pool); err != nil {
				return status.Error(codes.InvalidArgument, err.Error())
			}
			outPoolID = in.PoolId
		} else {
			if err := playpool.Create(tx, pool); err != nil {
				return err
			}
			outPoolID = pool.ID
		}
		if in.IdempotencyKey != "" {
			if err := playpool.PutRequest(tx, in.IdempotencyKey, outPoolID, hash); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &play.CreatePlayPoolDraftResp{
		PoolId:  outPoolID,
		Version: 1,
		Status:  "DRAFT",
	}, nil
}

func playPoolPrizesToDomain(mode string, prizes []*play.PlayPoolPrize) []*playpool.Prize {
	out := make([]*playpool.Prize, 0, len(prizes))
	for _, p := range prizes {
		if p == nil {
			continue
		}
		prizeType := int(p.PrizeType)
		if prizeType == 0 {
			prizeType = 1
		}
		parentIndex := -1
		if p.HasParent {
			parentIndex = int(p.ParentIndex)
		}
		totalPairs := -1
		if mode == "memorymatch" {
			totalPairs = int(p.TargetValue)
		}
		extra := p.ExtraJson
		if extra == "" {
			extra = "{}"
		}
		out = append(out, &playpool.Prize{
			Index:            int(p.PrizeIndex),
			Name:             p.Name,
			PrizeGrade:       p.PrizeGrade,
			PrizeType:        prizeType,
			SkuID:            p.SkuId,
			ChildPoolID:      p.ChildPoolId,
			CashFen:          p.CashFen,
			PointsAmount:     p.PointsAmount,
			CouponTemplateNo: p.CouponTemplateNo,
			Weight:           p.Weight,
			Quantity:         int(p.Quantity),
			IsPrize:          p.IsPrize,
			ImageURL:         p.ImageUrl,
			CostFen:          p.CostFen,
			SpecialKind:      p.SpecialKind,
			TargetValue:      int(p.TargetValue),
			ParentIndex:      parentIndex,
			TotalPairCount:   totalPairs,
			ExtraJSON:        extra,
		})
	}
	return out
}
