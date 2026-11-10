// 共享 API 类型：与后端 .api / types.go 对齐（规范 05-api-layer 第 9 条）
// 字段 camelCase；金额一律字符串两位小数；时间为 unix 秒

// 统一信封：{ code, msg, data }，code===0 成功（api-conventions.md）
export interface ApiResponse<T> {
  code: number
  msg: string
  data: T
}

// ── 用户域 ──

// 与后端 RegisterReq / LoginReq 对齐
export interface RegisterRequest {
  username: string
  password: string
  nickname?: string
}

export interface LoginRequest {
  username: string
  password: string
}

// 与后端 LoginResp 对齐
export interface AuthResponse {
  userId: number
  token: string
}

// M15a 手机号/微信登录（与后端 PhoneCodeReq / PhoneLoginReq / WeChatLoginReq 对齐）
export interface PhoneCodeRequest {
  tenantId: number
  phone: string
}

export interface PhoneCodeResponse {
  expiresAtUnix: number
}

export interface PhoneLoginRequest {
  tenantId: number
  phone: string
  code: string
  deviceId?: number
  clientFingerprint?: string
}

export interface WeChatLoginRequest {
  tenantId: number
  code: string
  scene: number // 1=App 2=小程序
  deviceId?: number
  clientFingerprint?: string
}

// 与后端 UserProfile 对齐
export interface UserProfile {
  userId: number
  username: string
  nickname: string
  balance: string
}

// 与后端 RechargeResp 对齐
export interface RechargeResponse {
  balance: string
}

// 与后端 BalanceResp 对齐
export interface BalanceResponse {
  balance: string
}

// ── 游戏域 ──

// 与后端 BoxItem 对齐
export interface BoxItem {
  boxId: number
  boxName: string
  price: string
  description: string
  soldOut: boolean
}

// 与后端 BoxListResp 对齐
export interface BoxListResponse {
  list: BoxItem[]
}

// 与后端 PoolEntryItem 对齐（probability 为公示百分比字符串，如 "5.00%"）
export interface PoolEntryItem {
  entryId: number
  name: string
  weight: number
  remaining: number
  isPrize: boolean
  probability: string
}

// 与后端 BoxDetailResp 对齐
export interface BoxDetailResponse {
  boxId: number
  boxName: string
  price: string
  description: string
  soldOut: boolean
  entries: PoolEntryItem[]
}

// 与后端 BuyBoxReq / BuyBoxResp 对齐（M10 起可选券/积分抵扣选择项）
export interface BuyBoxRequest {
  boxId: number
  userCouponId?: number
  useLoyaltyOffset?: boolean
}

export interface BuyBoxResponse {
  requestId: string
  status: string
}

// 与后端 DrawRecordItem 对齐
export interface DrawRecordItem {
  recordId: number
  requestId: string
  boxId: number
  boxName: string
  entryName: string
  isPrize: boolean
  price: string
  createdAt: number
}

// 与后端 RecordsListResp 对齐（分页固定结构 list/total）
export interface RecordsListResponse {
  list: DrawRecordItem[]
  total: number
}

// 与后端 WarehouseItem 对齐
export interface WarehouseItem {
  itemId: number
  recordId: number
  boxId: number
  boxName: string
  itemName: string
  createdAt: number
}

// 与后端 WarehouseListResp 对齐
export interface WarehouseListResponse {
  list: WarehouseItem[]
}

// ── 玩法域（M10 券/报价）──

// 与后端 BoxQuoteResp 对齐（金额元字符串；报价仅预览，购买时后端重算）
export interface BoxQuoteResponse {
  requestId: string
  quoteToken: string
  gross: string
  couponDiscount: string
  couponNo: string
  loyaltyPointsDebit: number
  loyalty: string
  payable: string
  cash: string
  loyaltyBalance: string
  loyaltyRate: number
  loyaltyMax: string
}

// 与后端 CouponItem 对齐
export interface CouponItem {
  id: number
  couponNo: string
  templateNo: string
  name: string
  kind: number // 1 FIXED_MINOR 2 PERCENT_BPS 3 FREE
  value: number // 面值（分/bps）
  minGross: string
  status: number // 1 AVAILABLE 2 LOCKED 3 USED 4 EXPIRED
  expiresAt: string
  usable: boolean
}

export interface CouponListResponse {
  list: CouponItem[]
}

export interface ClaimCouponResponse {
  couponNo: string
}

// 与后端 CouponTemplateItem 对齐
export interface CouponTemplateItem {
  id: number
  templateNo: string
  name: string
  kind: number
  value: number
  minGross: string
  claimed: boolean
}

export interface CouponTemplateListResponse {
  list: CouponTemplateItem[]
}

// ── 活动域（M11 campaign：签到/兑换码/转盘 + 活动宝箱资格）──

// 与后端 CampaignPrizeItem 对齐
export interface CampaignPrizeItem {
  kind: number // 1 CASH 2 POINTS 3 COUPON 4 GACHA_BOX
  amount: string
  points: number
  couponTemplateNo: string
  couponName: string
  boxId: number
  boxName: string
}

// 与后端 CampaignItem 对齐
export interface CampaignItem {
  campaignKey: string
  name: string
  kind: number // 1 签到 2 兑换码 3 转盘
  startAt: number
  endAt: number
  inWindow: boolean
  signedToday: boolean
  streak: number
  totalDays: number
  spinsLeft: number // -1=非转盘
  spinDailyLimit: number // -1=非转盘
}

export interface CampaignListResponse {
  list: CampaignItem[]
}

// 与后端 CampaignWheelSlot 对齐
export interface CampaignWheelSlot {
  slotIndex: number
  label: string
  weight: number
  remaining: number
  prize: CampaignPrizeItem
}

// 与后端 CampaignPreviewResp 对齐
export interface CampaignPreviewResponse {
  campaignKey: string
  kind: number
  alreadyParticipated: boolean
  streak: number
  nextStreak: number
  spinsLeft: number
  spinDailyLimit: number
  redemptionTimes: number
  redemptionUserLimit: number
  previewPrizes: CampaignPrizeItem[]
  slots: CampaignWheelSlot[]
}

// 与后端 CampaignParticipateResp 对齐
export interface CampaignParticipateResponse {
  requestId: string
  replayed: boolean
  streak: number
  spinsLeft: number
  prizes: CampaignPrizeItem[]
}

// 与后端 EntitlementItem 对齐（活动宝箱资格）
export interface EntitlementItem {
  entitlementNo: string
  boxId: number
  boxName: string
  status: number // 1 AVAILABLE 2 LOCKED 3 USED
  grantedAt: number
}

export interface EntitlementListResponse {
  list: EntitlementItem[]
}

// 活动宝箱开箱（M11c）：提交 SAGA 后 pending（轮询 /draw/records）；success=同 Key 重放命中
export interface OpenCampaignBoxResponse {
  requestId: string
  status: string
}

// ---------- 挖宝箱（M11b，与后端 DigBox* 对齐） ----------

export interface DigBoxItem {
  boxId: number
  name: string
  price: string // 单格价格（元字符串）
  cellCount: number
}

export interface DigBoxListResponse {
  items: DigBoxItem[]
}

export interface DigBoxSessionResponse {
  hasSession: boolean
  sessionNo: string
  occupiedBitmap: number // bit=1 已挖（cell_index 为位号）
  totalCellsPaid: number
  status: number // 1 PLAYING 2 COMPLETED 3 CANCELED
  cellCount: number
}

export interface DigQuoteResponse {
  boxId: number
  cellCount: number
  count: number
  unitPrice: string
  gross: string
  payable: string
}

export interface DigBuyResponse {
  requestId: string
  status: string // pending / success
}

export interface DigPrizeItem {
  kind: number // 1 CASH 2 POINTS 3 COUPON
  amount: string
  points: number
  couponTemplateNo: string
  couponName: string
}

export interface DigRecordItem {
  recordId: number
  requestId: string
  configId: number
  cellsChosen: string // 逗号分隔（0 起）
  prizes: DigPrizeItem[]
  price: string
  status: number // 1 正常 2 已撤销
  createdAt: number
}

export interface DigRecordsResponse {
  items: DigRecordItem[]
  total: number
}

// ── M12 滚动中奖（首页 marquee）──

export interface RollingWinItem {
  requestId: string
  userId: number
  boxId: number
  entryName: string
  displayValueFen: number
  tier: number // 1 NORMAL 2 HIGH_VALUE 3 LIMITED
  wonAt: number
}

export interface RollingRecentWinsResponse {
  wins: RollingWinItem[]
  highValueThresholdFen: number
}

// ── M12b 满车 full_car（与后端 FullCar* 对齐）──

// 座位（userId=0 空位）
export interface FullCarSeatItem {
  seatIndex: number
  userId: number
}

// 整车结算奖（DEPARTED 后可见；tier 1 LUCKY 2 NORMAL）
export interface FullCarAwardItem {
  seatIndex: number
  userId: number
  prize: DigPrizeItem
  tier: number
}

// 与后端 FullCarDetailResp 对齐（status 1 OPEN 2 DEPARTED）
export interface FullCarDetailResponse {
  configId: number
  name: string
  price: string // 单抽（占一座）价格（元）
  seatCount: number
  seats: FullCarSeatItem[]
  awards: FullCarAwardItem[]
  remainingTickets: number
  status: number
}

// 与后端 FullCarBuyResp 对齐（pending=已受理，轮询记录；success=同 Key 重放命中）
export interface FullCarBuyResponse {
  requestId: string
  status: string
}

// 与后端 FullCarRecordItem 对齐（status 1 正常 2 已撤销）
export interface FullCarRecordItem {
  recordId: number
  requestId: string
  configId: number
  seatIndex: number
  entryName: string
  isPrize: boolean
  price: string
  status: number
  createdAt: number
}

export interface FullCarRecordsResponse {
  items: FullCarRecordItem[]
  total: number
}

// ── M12c 扭蛋 gashapon（与后端 Gashapon* 对齐）──

// 特别奖配置视图（specialKind LAST / MVP / LUCKY；stageOrder 0=不分阶段，LUCKY 专用）
export interface GashaponSpecialPrizeItem {
  prizeIndex: number
  specialKind: string
  quantity: number
  sent: number
  stageOrder: number
  prize: DigPrizeItem
}

// regular 奖结果（谢谢参与无资产，仅名字+是否中奖）
export interface GashaponEntryOutcome {
  name: string
  isPrize: boolean
}

// 与后端 GashaponDetailResp 对齐（status 1 OPEN 2 SETTLED）
export interface GashaponDetailResponse {
  configId: number
  name: string
  price: string // 单抽价格（元）
  remainingTickets: number
  totalTickets: number
  participants: number
  status: number
  specialPrizes: GashaponSpecialPrizeItem[]
}

// 与后端 GashaponBuyResp 对齐（pending=已受理，轮询记录；success=同 Key 重放命中）
export interface GashaponBuyResponse {
  requestId: string
  status: string
}

// 与后端 GashaponRecordItem 对齐（status 1 正常 2 已撤销）
export interface GashaponRecordItem {
  recordId: number
  requestId: string
  configId: number
  count: number
  entries: GashaponEntryOutcome[]
  price: string
  status: number
  createdAt: number
}

export interface GashaponRecordsResponse {
  items: GashaponRecordItem[]
  total: number
}

// ── M13 连击链 combo_chain（与后端 ComboChain* 对齐）──

// 共享奖配置视图（kind COMBO_TIER / EMPTY_STREAK / WIN_STREAK；连击奖 remaining=-1 无限量）
export interface ComboChainSharedPrizeItem {
  kind: string
  quantity: number
  remaining: number
  tierWeight: number
  concentrateThreshold: number
  streakThreshold: number
  prize: DigPrizeItem
}

// 与后端 ComboChainDetailResp 对齐（status 1 OPEN 2 SETTLED）
export interface ComboChainDetailResponse {
  configId: number
  name: string
  price: string // 单票价格（元）
  ticketsPerCar: number
  totalCars: number
  carIndex: number
  participants: number
  remainingTickets: number
  status: number
  globalSlots: DigPrizeItem[]
  sharedPrizes: ComboChainSharedPrizeItem[]
  emptyStreak: number
  winStreak: number
}

// 与后端 ComboChainBuyResp 对齐（pending=已受理，轮询记录；success=同 Key 重放命中）
export interface ComboChainBuyResponse {
  requestId: string
  status: string
}

// 与后端 ComboChainRecordItem 对齐（status 1 正常 2 已撤销）
export interface ComboChainRecordItem {
  recordId: number
  requestId: string
  configId: number
  carIndex: number
  awards: DigPrizeItem[]
  price: string
  status: number
  createdAt: number
}

export interface ComboChainRecordsResponse {
  items: ComboChainRecordItem[]
  total: number
}

// ── M13 拼团抽 group_buy（与后端 GroupBuy* 对齐）──

// 幸运奖配置视图（含已发进度）
export interface GroupBuyLuckyPrizeItem {
  prizeIndex: number
  quantity: number
  sent: number
  prize: DigPrizeItem
}

// 与后端 GroupBuyDetailResp 对齐（status 1 OPEN 2 SETTLED）
export interface GroupBuyDetailResponse {
  configId: number
  name: string
  price: string // 单注价格（元）
  targetBetCount: number
  perBetAwardLimit: number
  remainingBets: number
  myBets: number
  participants: number
  status: number
  luckyPrizes: GroupBuyLuckyPrizeItem[]
}

// 与后端 GroupBuyBuyResp 对齐（pending=已受理，轮询记录；success=同 Key 重放命中）
export interface GroupBuyBuyResponse {
  requestId: string
  status: string
}

// 与后端 GroupBuyRecordItem 对齐（status 1 正常 2 已撤销）
export interface GroupBuyRecordItem {
  recordId: number
  requestId: string
  configId: number
  count: number
  awards: DigPrizeItem[]
  price: string
  status: number
  createdAt: number
}

export interface GroupBuyRecordsResponse {
  items: GroupBuyRecordItem[]
  total: number
}

// ── M15b 记忆翻牌 memory_match（与后端 MemoryMatch* 对齐）──

// 正面图案（patternId 1..9）
export interface MemoryMatchFaceItem {
  patternId: number
  name: string
  imageUrl: string
}

// 正面预设（恰 9 图案）
export interface MemoryMatchFacePreset {
  presetId: number
  name: string
  faces: MemoryMatchFaceItem[]
}

// 背面预设
export interface MemoryMatchBackPreset {
  presetId: number
  name: string
  backImageUrl: string
  faceBackgroundImageUrl: string
}

// 奖档（谢谢参与 prize.kind=0）
export interface MemoryMatchEntryItem {
  entryId: number
  name: string
  totalPairCount: number
  prize: DigPrizeItem
  remaining: number
}

// 擂主信息
export interface MemoryMatchChampionInfo {
  exists: boolean
  userId: number
  totalPairCount: number
  pendingIncome: string
}

// 外观视图
export interface MemoryMatchAppearanceView {
  facePresetId: number
  backPresetId: number
  ownedFacePresetIds: number[]
}

// 玩法配置
export interface MemoryMatchConfigDetail {
  configId: number
  name: string
  roundsPerPurchase: number
  maxPairCount: number
  purchasePrice: string
  walletKind: number
  secondDestinyEnabled: boolean
  deckStrategy: number
  patternWeights: number[]
  exactMinPairCount: number
  exactMaxPairCount: number
  exactAttemptLimit: number
  facePresets: MemoryMatchFacePreset[]
  backPresets: MemoryMatchBackPreset[]
  defaultFacePresetId: number
  defaultBackPresetId: number
  championMinTotalPairs: number
  championShareBps: number
  status: number
  entries: MemoryMatchEntryItem[]
}

export interface MemoryMatchConfigResponse {
  config: MemoryMatchConfigDetail
  champion: MemoryMatchChampionInfo
  appearance: MemoryMatchAppearanceView
}

// 购买响应（网关轮询 SAGA 至会话建立后返回；state 2=待选命运 3=待选第二命运 4=游玩中 6=已完成）
export interface MemoryMatchPurchaseResponse {
  sessionId: number
  purchasedRounds: number
  paidMinor: number
  state: number
  facePresetId: number
  backPresetId: number
}

// 回合简报
export interface MemoryMatchRoundBrief {
  roundNo: number
  state: number
  pairCount: number
}

// 终局结算视图
export interface MemoryMatchSettlementView {
  totalPairCount: number
  finalHash: string
  prize: DigPrizeItem
  status: number
}

// 会话进度（state 见购买响应注释；settlement 终局后出现）
export interface MemoryMatchSessionResponse {
  sessionId: number
  configId: number
  state: number
  purchasedRounds: number
  completedRounds: number
  currentRound: number
  destinyPatternId: number
  secondDestinyEnabled: boolean
  secondDestinyPatternId: number
  rounds: MemoryMatchRoundBrief[]
  settlement?: MemoryMatchSettlementView
  facePresetId: number
  backPresetId: number
}

// 回合视图
export interface MemoryMatchRoundResponse {
  roundNo: number
  state: number
  handCardIds: number[]
  drawPosition: number
  pairCount: number
  maxPairCount: number
  availableDestiny: number
  destinyDraws: number
  seedCommitment: string
  actions: string[]
  terminal: boolean
  resultHash: string
  destinyPatternId: number
  secondDestinyPatternId: number
}

// 命运选择响应
export interface MemoryMatchActionResponse {
  patternId: number
  status: string
}

// 配对/命运抽牌响应
export interface MemoryMatchRoundActionResponse {
  round: MemoryMatchRoundResponse
}

// 完成回合响应（末回合含终局结算）
export interface MemoryMatchCompleteResponse {
  round: MemoryMatchRoundResponse
  sessionState: number
  settlement?: MemoryMatchSettlementView
}

// 跳过响应
export interface MemoryMatchSkipResponse {
  sessionId: number
  sessionState: number
  settlement?: MemoryMatchSettlementView
}

// ===== M15c 任务玩家体系：任务/签到/通行证/会员 =====

// 任务项（period 1 ONCE/2 DAILY/3 WEEKLY；metric 1 DRAW_COUNT/2 DRAW_SPEND_MINOR/3 LOGIN/4 RECHARGE_MINOR/5 SIGN_IN）
export interface TaskItem {
  taskId: number
  version: number
  name: string
  period: number
  metric: number
  target: number
  cycleLimit: number
  rewardTemplateId: string
  rewardCount: number
  progress: number
  claimed: number
  state: number // 1 IN_PROGRESS 2 CLAIMABLE 3 FINISHED
}

// 通行证奖励槽（track 1 FREE/2 PREMIUM）
export interface PassRewardItem {
  level: number
  track: number
  rewardTemplateId: string
  rewardCount: number
}

// 通行证赛季视图（premiumLine 1 PAYMENT 付费解锁/2 MEMBER 会员解锁）
export interface PassViewItem {
  passId: number
  version: number
  name: string
  xpPerLevel: number
  maxLevel: number
  xpPerSpendMinor: number
  premiumLine: number
  premiumMemberLevel: number
  rewards: PassRewardItem[]
  xp: number
  level: number
  premium: boolean
}

// 会员档位
export interface MemberTierItem {
  level: number
  spendMinor: number
  rewardTemplateId: string
  rewardCount: number
}

// 会员视图
export interface MemberView {
  spendMinor: number
  level: number
  tiers: MemberTierItem[]
}

// 任务列表响应（含通行证/会员扩展视图）
export interface ListTasksResponse {
  tasks: TaskItem[]
  nextPageToken?: string
  passes?: PassViewItem[]
  member?: MemberView
}

// 签到响应
export interface SignInTaskResponse {
  streak: number
  calendarDate: string
  alreadySigned: boolean
}

// 任务领取响应
export interface ClaimTaskResponse {
  rewardTemplateId: string
  rewardCount: number
  claimed: number
}

// 通行证领取响应
export interface ClaimPassResponse {
  level: number
  track: number
  rewardTemplateId: string
  rewardCount: number
}

// 会员领取响应
export interface ClaimMemberResponse {
  rewardTemplateId: string
  rewardCount: number
  claimed: number
}
