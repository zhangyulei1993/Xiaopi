const digitValues: Record<string, number> = { 零: 0, 〇: 0, 一: 1, 二: 2, 两: 2, 三: 3, 四: 4, 五: 5, 六: 6, 七: 7, 八: 8, 九: 9 };

function normalizeChineseNumeral(value: string): string {
  if (!/[十百千万亿]/u.test(value)) return [...value].map(char => String(digitValues[char] ?? char)).join('')
  let total = 0, section = 0, number = 0
  for (const char of value) {
    if (char in digitValues) { number = digitValues[char]; continue }
    const unit = char === '十' ? 10 : char === '百' ? 100 : char === '千' ? 1000 : char === '万' ? 10000 : char === '亿' ? 100000000 : 0
    if (unit >= 10000) { total += (section + number) * unit; section = 0; number = 0 }
    else { section += (number || (unit === 10 ? 1 : 0)) * unit; number = 0 }
  }
  return String(total + section + number)
}

export function normalizeCommunityIdentity(value: string): string {
  return value.trim().toLocaleLowerCase()
    .replace(/[０-９]/gu, digit => String.fromCharCode(digit.charCodeAt(0) - 0xfee0))
    .replace(/[零〇一二两三四五六七八九十百千万亿]+/gu, normalizeChineseNumeral)
    .replace(/[^\p{L}\p{N}]/gu, '')
}
