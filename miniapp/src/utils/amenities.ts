export const amenityIcons: Record<string, string> = {
  冰箱: "/static/amenities/fridge.svg",
  洗衣机: "/static/amenities/washer.svg",
  空调: "/static/amenities/air-conditioner.svg",
  热水器: "/static/amenities/water-heater.svg",
  电视: "/static/amenities/tv.svg",
  宽带: "/static/amenities/wifi.svg",
  燃气: "/static/amenities/gas.svg",
  油烟机: "/static/amenities/hood.svg",
  沙发: "/static/amenities/sofa.svg",
  衣柜: "/static/amenities/wardrobe.svg",
  书桌: "/static/amenities/desk.svg",
  床: "/static/amenities/bed.svg",
  地铁: "/static/amenities/subway.svg",
  电梯: "/static/amenities/elevator.svg",
  车位: "/static/amenities/parking.svg",
  智能门锁: "/static/amenities/smart-lock.svg",
  阳台: "/static/amenities/balcony.svg",
  独立卫浴: "/static/amenities/bathroom.svg",
  独立卫生间: "/static/amenities/bathroom.svg",
};

export const amenityIcon = (name: string) =>
  amenityIcons[name] || "/static/amenities/generic.svg";
