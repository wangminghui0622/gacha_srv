import { http } from '@/utils/request'

export interface ShipmentOrderItem {
  id?: string
  warehouseItemId?: string
  boxId?: string
  itemName?: string
  imageUrl?: string
  quantity?: number
  createdAtUnixMillis?: string
}

export type ShipmentOrderStatus =
  | 'SHIPMENT_ORDER_STATUS_PENDING_PAYMENT'
  | 'SHIPMENT_ORDER_STATUS_PENDING_FINAL'
  | 'SHIPMENT_ORDER_STATUS_PENDING_SHIPMENT'
  | 'SHIPMENT_ORDER_STATUS_PENDING_RECEIPT'
  | 'SHIPMENT_ORDER_STATUS_COMPLETED'
  | 'SHIPMENT_ORDER_STATUS_CLOSED'
  | 'SHIPMENT_ORDER_STATUS_USER_CLOSED'

export interface ShipmentOrder {
  id?: string
  orderNo?: string
  userId?: string
  status?: ShipmentOrderStatus
  createdAtUnixMillis?: string
  paidAtUnixMillis?: string
  submittedDays?: number
  shippedAtUnixMillis?: string
  completedAtUnixMillis?: string
  remark?: string
  receiverName?: string
  receiverPhone?: string
  receiverAddress?: string
  logisticsCompany?: string
  trackingNo?: string
  items?: ShipmentOrderItem[]
  itemCount?: number
}

export interface ListShipmentOrdersReply {
  items?: ShipmentOrder[]
  total?: string
}

export interface UpdateShipmentOrderPayload {
  status: number
  remark: string
  receiverName: string
  receiverPhone: string
  receiverAddress: string
  logisticsCompany: string
  trackingNo: string
  shippedAtUnixMillis: number
  completedAtUnixMillis: number
}

export function shipmentClient() {
  return {
    List(params: {
      keyword?: string
      status?: number
      page?: number
      pageSize?: number
    }): Promise<ListShipmentOrdersReply> {
      return http.get('/console/v2/api/shipment-orders', {
        params: {
          keyword: params.keyword ?? '',
          status: params.status ?? 0,
          page: params.page ?? 1,
          pageSize: params.pageSize ?? 15,
        },
      })
    },
    Get(id: string | number): Promise<ShipmentOrder> {
      return http.get(`/console/v2/api/shipment-orders/${encodeURIComponent(String(id))}`)
    },
    Update(id: string | number, payload: UpdateShipmentOrderPayload): Promise<ShipmentOrder> {
      return http.put(`/console/v2/api/shipment-orders/${encodeURIComponent(String(id))}`, payload)
    },
    Close(id: string | number, status = 6): Promise<ShipmentOrder> {
      return http.post(`/console/v2/api/shipment-orders/${encodeURIComponent(String(id))}/close`, { status })
    },
  }
}
