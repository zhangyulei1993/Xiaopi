import type { PublicConfig } from '../types/domain'
const placements = ['enabledFields','customerDetailFields','customerShareFields','customerPosterFields'] as const
const shareMap: Record<string,string> = {layout:'layout',roomNo:'unit',listingSource:'listingSource',rentType:'rentType',areaSqm:'area',floorInfo:'floor'}
export type DisabledTagState = {
 allowed: string[]
 placements: Record<(typeof placements)[number],string[]>
 title: string[]
 poster: string[]
 generatedTitle: string[]
 generatedPoster: string[]
 order: {allowed:string[];placements:Record<(typeof placements)[number],string[]>;title:string[];poster:string[]}
}
export function captureDisabledTag(config: PublicConfig, base?: DisabledTagState): DisabledTagState {
 const snapshot={allowed:[...config.listingTitle.customerAllowedFields],placements:Object.fromEntries(placements.map(p=>[p,[...config.listingTitle[p]]])) as DisabledTagState['placements'],title:[...config.shareCard.titleFields],poster:[...config.shareCard.posterFields]}
 return {...snapshot,generatedTitle:[],generatedPoster:[],order:base?.order||snapshot}
}
// Insert only the restored item using surviving neighbours. Other edits retain
// their order, including when several tags were disabled and restored out of order.
function insertRestored(current:string[], before:string[], keys:string[]) {
 const result=[...current]
 for(const key of before.filter(k=>keys.includes(k))) {
  if(result.includes(key))continue
  const index=before.indexOf(key)
  const following=before.slice(index+1).find(k=>result.includes(k))
  if(following)result.splice(result.indexOf(following),0,key)
  else {const previous=before.slice(0,index).reverse().find(k=>result.includes(k));if(previous)result.splice(result.indexOf(previous)+1,0,key);else result.push(key)}
 }
 return result
}
export function restoreDisabledTag(config:PublicConfig,key:string,state:DisabledTagState) {
 const rules=config.listingTitle
 rules.customerAllowedFields=insertRestored(rules.customerAllowedFields,state.order.allowed,[key])
 for(const p of placements)if(state.placements[p].includes(key))rules[p]=insertRestored(rules[p],state.order.placements[p],[key])
 for(const [kind,before,order,generated,min,p] of [
  ['titleFields',state.title,state.order.title,state.generatedTitle,1,'customerShareFields'],
  ['posterFields',state.poster,state.order.poster,state.generatedPoster,3,'customerPosterFields'],
 ] as const){
  if(!rules[p].includes(key))continue
  const removed=before.filter(f=>shareMap[f]===key)
  let restored=insertRestored(config.shareCard[kind].filter(f=>!generated.includes(f)),order,removed)
  if(restored.length<min)restored=insertRestored(config.shareCard[kind],order,removed)
  config.shareCard[kind]=restored
 }
}
