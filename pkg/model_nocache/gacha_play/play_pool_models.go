package gacha_play

import "time"

// PlayPoolHead 统一玩赏池头（所有玩法共用）。
type PlayPoolHead struct {
	Id          int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Mode        string    `gorm:"column:mode"`
	Name        string    `gorm:"column:name"`
	NextVersion int64     `gorm:"column:next_version"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

func (PlayPoolHead) TableName() string { return "play_pool_heads" }

// PlayPoolVersion 统一玩赏池版本。
type PlayPoolVersion struct {
	PoolId      int64      `gorm:"column:pool_id;primaryKey"`
	Version     int64      `gorm:"column:version;primaryKey"`
	Status      int64      `gorm:"column:status"`
	Mode        string     `gorm:"column:mode"`
	Name        string     `gorm:"column:name"`
	PriceFen    int64      `gorm:"column:price_fen"`
	ConfigJson  string     `gorm:"column:config_json"`
	ConfigHash  string     `gorm:"column:config_hash"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
}

func (PlayPoolVersion) TableName() string { return "play_pool_versions" }

// PlayPoolPrize 统一玩赏品。
type PlayPoolPrize struct {
	Id               int64     `gorm:"column:id;primaryKey;autoIncrement"`
	PoolId           int64     `gorm:"column:pool_id"`
	Version          int64     `gorm:"column:version"`
	PrizeIndex       int32     `gorm:"column:prize_index"`
	PrizeGrade       string    `gorm:"column:prize_grade"`
	SpecialKind      string    `gorm:"column:special_kind"`
	PrizeType        int32     `gorm:"column:prize_type"`
	Name             string    `gorm:"column:name"`
	SkuId            int64     `gorm:"column:sku_id"`
	ChildPoolId      int64     `gorm:"column:child_pool_id"`
	Weight           int64     `gorm:"column:weight"`
	Quantity         int32     `gorm:"column:quantity"`
	Remaining        int32     `gorm:"column:remaining"`
	IsPrize          int32     `gorm:"column:is_prize"`
	TargetValue      int32     `gorm:"column:target_value"`
	CashFen          int64     `gorm:"column:cash_fen"`
	PointsAmount     int64     `gorm:"column:points_amount"`
	CouponTemplateNo string    `gorm:"column:coupon_template_no"`
	CostFen          int64     `gorm:"column:cost_fen"`
	ImageUrl         string    `gorm:"column:image_url"`
	ParentIndex      int32     `gorm:"column:parent_index"`
	ExtraJson        string    `gorm:"column:extra_json"`
	CreatedAt        time.Time `gorm:"column:created_at"`
}

func (PlayPoolPrize) TableName() string { return "play_pool_prizes" }

// PlayPoolDraft 统一玩赏池草稿幂等记录。
type PlayPoolDraft struct {
	IdempotencyKey string    `gorm:"column:idempotency_key;primaryKey"`
	PoolId         int64     `gorm:"column:pool_id"`
	Version        int64     `gorm:"column:version"`
	RequestHash    string    `gorm:"column:request_hash"`
	CreatedAt      time.Time `gorm:"column:created_at"`
}

func (PlayPoolDraft) TableName() string { return "play_pool_drafts" }
