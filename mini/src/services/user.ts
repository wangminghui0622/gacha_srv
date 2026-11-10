import { get, post } from './api'
import type {
  AuthResponse,
  BalanceResponse,
  LoginRequest,
  PhoneCodeRequest,
  PhoneCodeResponse,
  PhoneLoginRequest,
  RechargeResponse,
  RegisterRequest,
  UserProfile,
  WeChatLoginRequest,
} from './types'

// 用户域服务：端点与后端 .api 定义一致（/user/*，规范 05-api-layer 第 6 条）

/** 用户注册（公开） */
export function register(data: RegisterRequest): Promise<{ userId: number }> {
  return post<{ userId: number }>('/user/register', data)
}

/** 用户登录（公开），返回 token 与 userId */
export function login(data: LoginRequest): Promise<AuthResponse> {
  return post<AuthResponse>('/user/login', data)
}

/** 手机号登录验证码（公开，M15a） */
export function requestPhoneCode(data: PhoneCodeRequest): Promise<PhoneCodeResponse> {
  return post<PhoneCodeResponse>('/auth/phone/code', data)
}

/** 手机号验证码登录（公开，M15a） */
export function phoneLogin(data: PhoneLoginRequest): Promise<AuthResponse> {
  return post<AuthResponse>('/auth/phone/login', data)
}

/** 微信登录（公开，M15a；生产由小程序/App 取 code，web 调试用） */
export function wechatLogin(data: WeChatLoginRequest): Promise<AuthResponse> {
  return post<AuthResponse>('/auth/wechat/login', data)
}

/** 当前用户信息（含余额） */
export function getProfile(): Promise<UserProfile> {
  return get<UserProfile>('/user/profile')
}

/** 查询余额 */
export function getBalance(): Promise<string> {
  return get<BalanceResponse>('/user/balance').then((r) => r.balance)
}

/** 充值（mock，直接加款）；Idempotency-Key 头保证重试只生效一次 */
export function recharge(amount: string, idempotencyKey: string): Promise<RechargeResponse> {
  return post<RechargeResponse>('/user/recharge', { amount }, {
    headers: { 'Idempotency-Key': idempotencyKey },
  })
}
