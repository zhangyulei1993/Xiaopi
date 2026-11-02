import { request } from './client'

export interface Property { id: string; title: string; community: string; location: string; layout: string; rentMonthly: number; feeText: string; description: string; status: string; media?: { type: string; url: string }[] }
export const listProperties = () => request<Property[]>('/v1/properties')
export const createProperty = (property: Partial<Property>) => request<Property>('/v1/properties', { method: 'POST', data: property })
export const updateProperty = (property: Partial<Property>) => request<Property>('/v1/properties/update', { method: 'PATCH', data: property })
export interface PropertyLead { id: string; sourceName: string; sourcePhone: string; community: string; location: string; layout: string; expectedRentMonthly: number; note: string; status: string }
export const createPropertyLead = (lead: Partial<PropertyLead>) => request<PropertyLead>('/v1/property-leads', { method: 'POST', data: lead })

export interface WechatLoginResult {
  accessToken: string
  isDevMock?: boolean
}

export interface PublicConfig {
  contactPhone: string
}

export const wechatLogin = (code: string) => request<WechatLoginResult>('/v1/auth/wechat/login', {
  method: 'POST',
  data: { code },
})

export const getPublicConfig = () => request<PublicConfig>('/v1/config/public')
