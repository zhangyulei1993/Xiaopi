export const defaultShareCard = () => ({ titlePrefix: '', titleFields: ['community', 'layout'], posterFields: ['rentMonthly', 'depositPayment', 'community', 'areaSqm', 'floorInfo', 'availableDate'] })
export const defaultListingTitle = () => ({ marketingPhrases: ['好房快抢', '捡漏不等人'], customerAllowedFields: ['community', 'floor', 'layout', 'rentType', 'listingSource', 'fee', 'area'], enabledFields: ['community', 'layout', 'rentType', 'fee'], customerDetailFields: ['community', 'layout', 'rentType', 'fee'], customerShareFields: ['community', 'layout', 'rentType', 'fee'], customerPosterFields: ['community', 'layout', 'rentType', 'fee'], adminEnabledFields: ['unit'], listTagLayout: 'grouped' as const, detailTagLayout: 'wrap' as const, listTagOrder: ['rentType', 'listingSource', 'layout', 'fee', 'floor', 'area'], listTagTopFields: ['rentType', 'listingSource'], detailTagOrder: ['listingSource', 'rentType', 'fee', 'layout', 'area', 'floor'], detailTagTopFields: ['listingSource', 'rentType', 'fee'] })
export const defaultPropertyDetail = () => ({
  modules: [
    { key: 'overview', title: '房屋概况', enabled: true }, { key: 'location', title: '位置与周边', enabled: true },
    { key: 'facilities', title: '房屋设施', enabled: true }, { key: 'description', title: '房源简介', enabled: true }, { key: 'nearby', title: '附近房源', enabled: true },
  ],
  nearby: { radiusKm: 3, relation: 'distance' as const, sort: 'distance' as const, limit: 4 },
  facilities: [
    { key: 'refrigerator', label: '冰箱', group: '家具家电', enabled: true }, { key: 'washer', label: '洗衣机', group: '家具家电', enabled: true },
    { key: 'air_conditioner', label: '空调', group: '家具家电', enabled: true }, { key: 'water_heater', label: '热水器', group: '家具家电', enabled: true },
    { key: 'television', label: '电视', group: '家具家电', enabled: true }, { key: 'broadband', label: '宽带', group: '家具家电', enabled: true },
    { key: 'gas', label: '燃气', group: '厨房', enabled: true }, { key: 'hood', label: '油烟机', group: '厨房', enabled: true },
    { key: 'sofa', label: '沙发', group: '家具家电', enabled: true }, { key: 'wardrobe', label: '衣柜', group: '家具家电', enabled: true },
    { key: 'desk', label: '书桌', group: '家具家电', enabled: true }, { key: 'bed', label: '床', group: '家具家电', enabled: true },
  ],
})
