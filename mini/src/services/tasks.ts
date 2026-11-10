import { get, post } from './api'

import type {
  ClaimMemberResponse,
  ClaimPassResponse,
  ClaimTaskResponse,
  ListTasksResponse,
  SignInTaskResponse,
} from './types'

// M15c 任务玩家体系：services/ 唯一 API 层（规范 05-api-layer）

// 任务列表（公开，可选登录绑定用户进度；period 过滤可选）
export function listTasks(params?: {
  period?: number
  pageSize?: number
  pageToken?: string
}): Promise<ListTasksResponse> {
  return get('/tasks', params)
}

// 每日签到（老项目 BindUser 语义：用户身份以令牌为准）
export function signInTask(taskId: number, version: number): Promise<SignInTaskResponse> {
  return post(`/tasks/${taskId}/sign-in`, { version })
}

// 任务奖励领取（requestId 幂等重放）
export function claimTask(
  taskId: number,
  version: number,
  requestId: string,
): Promise<ClaimTaskResponse> {
  return post(`/tasks/${taskId}/claim`, { version, requestId })
}

// 通行证奖励领取（requestId 幂等重放；track 1 FREE/2 PREMIUM）
export function claimPassReward(
  passId: number,
  level: number,
  track: number,
  requestId: string,
): Promise<ClaimPassResponse> {
  return post(`/tasks/passes/${passId}/claim`, { level, track, requestId })
}

// 会员奖励领取（requestId 幂等重放）
export function claimMemberReward(level: number, requestId: string): Promise<ClaimMemberResponse> {
  return post('/membership/claim', { level, requestId })
}
