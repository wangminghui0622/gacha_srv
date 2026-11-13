// 商品 typed service：方法名与参考 ProductEditor 调用点一致
// （Get/GetRevision/Create/SaveRevision/SaveVariantRevision/Publish/PublishRevision），
// axios 直连本仓 /catalog 路由。
import { http } from '@/utils/request'

export interface Money {
  amountMinor?: string | number
  currency?: string
}

export interface OptionValueView {
  optionValueId?: string
  optionId?: string
  value?: string
  sortOrder?: number
}

export interface ProductOptionView {
  optionId?: string
  name?: string
  sortOrder?: number
  values?: OptionValueView[]
}

export interface SkuSelectionView {
  optionId?: string
  optionValueId?: string
}

export interface SkuView {
  skuId?: string
  productId?: string
  skuCode?: string
  title?: string
  status?: string | number
  fulfillmentMode?: string | number
  salePrice?: Money
  referencePrice?: Money
  selections?: SkuSelectionView[]
  externalCode?: string
}

export interface Product {
  productId?: string
  productCode?: string
  title?: string
  status?: string | number
  options?: ProductOptionView[]
  skus?: SkuView[]
  version?: string
}

export interface CreateOptionValueInput {
  value: string
  sortOrder: number
}

export interface CreateProductOptionInput {
  name: string
  sortOrder: number
  values: CreateOptionValueInput[]
}

export interface CreateSkuSelectionInput {
  optionIndex: number
  valueIndex: number
}

export interface CreateSkuInput {
  skuCode: string
  title: string
  fulfillmentMode: string | number
  salePrice: Money
  referencePrice: Money
  selections: CreateSkuSelectionInput[]
  externalCode: string
}

export interface CreateProductInput {
  productCode: string
  title: string
  options: CreateProductOptionInput[]
  skus: CreateSkuInput[]
}

export interface ReviseSkuInput {
  skuId: string
  title: string
  externalCode: string
  fulfillmentMode: string | number
  salePrice: Money
  referencePrice: Money
}

export interface IpItem {
  ipId?: string
  name?: string
  sortWeight?: number
}

export interface ListIpsReply {
  ips?: IpItem[]
  total?: string
  page?: number
  pageSize?: number
}

export const catalogIpClient = {
  List({ keyword, page, pageSize }: { keyword: string; page: number; pageSize: number }): Promise<ListIpsReply> {
    return http.get('/console/v2/api/catalog/ips', { params: { keyword, page, pageSize } })
  },
  Create(payload: { name: string; sortWeight: number }): Promise<IpItem> {
    return http.post('/console/v2/api/catalog/ips', payload)
  },
  Update(ipId: string, payload: { name: string; sortWeight: number }): Promise<IpItem> {
    return http.put(`/console/v2/api/catalog/ips/${encodeURIComponent(ipId)}`, payload)
  },
}

export interface BrandItem {
  brandId?: string
  name?: string
  logo?: string
}

export interface ListBrandsReply {
  brands?: BrandItem[]
  total?: string
  page?: number
  pageSize?: number
}

export const catalogBrandClient = {
  List({ keyword, page, pageSize }: { keyword: string; page: number; pageSize: number }): Promise<ListBrandsReply> {
    return http.get('/console/v2/api/catalog/brands', { params: { keyword, page, pageSize } })
  },
  Create(payload: { name: string; logo: string }): Promise<BrandItem> {
    return http.post('/console/v2/api/catalog/brands', payload)
  },
  Update(brandId: string, payload: { name: string; logo: string }): Promise<BrandItem> {
    return http.put(`/console/v2/api/catalog/brands/${encodeURIComponent(brandId)}`, payload)
  },
}

export const catalogClient = {
  Get({ productId }: { productId: string }): Promise<Product> {
    return http.get(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}`)
  },
  GetRevision({ productId }: { productId: string }): Promise<Product> {
    return http.get(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}/revision`)
  },
  Create(payload: CreateProductInput): Promise<Product> {
    return http.post('/console/v2/api/catalog/products', payload)
  },
  SaveRevision(payload: { productId: string; expectedVersion: string | number; title: string; skus: ReviseSkuInput[] }): Promise<Product> {
    const { productId, ...body } = payload
    return http.post(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}/revision`, body)
  },
  SaveVariantRevision(payload: { productId: string; expectedVersion: string | number; product: CreateProductInput }): Promise<Product> {
    const { productId, ...body } = payload
    return http.post(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}/variant-revision`, body)
  },
  Publish({ productId, expectedVersion }: { productId: string; expectedVersion: string | number }): Promise<Product> {
    return http.post(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}/publish`, { expectedVersion })
  },
  PublishRevision({ productId, expectedVersion }: { productId: string; expectedVersion: string | number }): Promise<Product> {
    return http.post(`/console/v2/api/catalog/products/${encodeURIComponent(productId)}/revision/publish`, { expectedVersion })
  },
}
