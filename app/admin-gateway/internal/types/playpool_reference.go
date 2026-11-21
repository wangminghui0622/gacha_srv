package types

// PlayPoolReferenceItem 引用查询结果行。
type PlayPoolReferenceItem struct {
	PoolId     int64    `json:"poolId"`
	PoolName   string   `json:"poolName"`
	Mode       string   `json:"mode"`
	Periods    int64    `json:"periods"`
	Status     int64    `json:"status"`
	Tags       []string `json:"tags"`
	Source     string   `json:"source"`
	RefType    int64    `json:"refType"`
	RefId      int64    `json:"refId"`
	PrizeGrade string   `json:"prizeGrade"`
	Quantity   int64    `json:"quantity"`
}

type QueryPlayPoolReferencesReq struct {
	RefType  int64 `form:"refType,optional"`
	RefId    int64 `form:"refId,optional"`
	Page     int64 `form:"page,optional"`
	PageSize int64 `form:"pageSize,optional"`
}

type QueryPlayPoolReferencesResp struct {
	Items    []PlayPoolReferenceItem `json:"items"`
	Total    int64                   `json:"total"`
	Page     int64                   `json:"page"`
	PageSize int64                   `json:"pageSize"`
}
