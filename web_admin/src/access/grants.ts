export interface PermissionNode {
  resource: string
  actions?: string[]
  dataScope?: number
}

export interface PermissionTree {
  policyId?: string
  policyVersion?: string
  nodes?: PermissionNode[]
}

export function allows(nodes: PermissionNode[], resource: string, action: string): boolean {
  return nodes.some((node) => {
    const resourceOk = node.resource === '*' || node.resource === resource
    const actions = node.actions ?? []
    const actionOk = actions.includes('*') || actions.includes(action)
    return resourceOk && actionOk
  })
}
