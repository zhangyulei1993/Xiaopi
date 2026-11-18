import type { PublicConfig } from '../types/domain'
import { pagePresentationOptions } from './navigation'

export type ConfigChange = { id: string; paths: string[][]; page: string; label: string; roles: string; before: unknown; after: unknown }
export type ConfigPublication = { id: string; status?: string; error?: string; operator: string; createdAt: string; revision: string; restoredFrom?: string; before: PublicConfig; after: PublicConfig }
export type ConfigState = { config: PublicConfig; revision: string }
const copyScopes = new Map<string,string>()
export const registerCopyScope = (page:string, field:string, roles:string[]) => copyScopes.set(JSON.stringify([page,field]),roles.join('、'))
const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b)
const titles: Record<string,string> = {pagePresentations:'页面编辑',heroTitle:'品牌名称',contactPhone:'联系电话',relatedPropertyRadiusKm:'附近默认范围',locations:'地区入口',featuredChannels:'精选频道',shareCard:'分享内容',listingTitle:'房源标签规则',propertyDetail:'详情设置',statusLabels:'房源状态名称',title:'标题',subtitle:'说明文案',backgroundImage:'背景图片',operatorTitle:'管理员标题',operatorSubtitle:'管理员说明',searchPlaceholder:'搜索提示',eyebrow:'页眉说明',channelTitle:'频道标题',allChannelsText:'全部频道入口',recommendationTitle:'推荐房源标题',viewAllText:'查看房源入口',nearbyLabel:'附近入口',modules:'详情模块',nearby:'附近推荐',facilities:'房屋设施库',marketingPhrases:'广告短语',draft:'草稿',pendingReview:'待审核',published:'已发布',available:'可租',rented:'已租',offline:'已下架',enabledFields:'公开标签',customerAllowedFields:'可用标签',adminEnabledFields:'管理员额外标签',titlePrefix:'分享标题前缀',titleFields:'好友分享字段',posterFields:'朋友圈字段',radiusKm:'推荐范围',relation:'关联方式',sort:'排序',limit:'展示数量',listTagLayout:'标签布局',primaryFields:'首行标签',secondaryFields:'次行标签'}
export function changeScope(path: string[]): string {
 const [root,page,field,key]=path
 if(root==='pagePresentations'&&field==='copy'&&copyScopes.has(JSON.stringify([page,key])))return copyScopes.get(JSON.stringify([page,key]))!
 if(root==='listingTitle') return page==='adminEnabledFields'||page==='marketingPhrases'?'仅管理员（编辑入口）': '全角色（公开标签）；游客、微信用户（分享约束）'
 if(root==='shareCard')return '游客、微信用户、管理员（分享内容）'
 if(root==='propertyDetail')return '全角色（详情）；管理员（录入编辑）'
 if(root==='statusLabels')return '全角色（状态展示）'
 if(root!=='pagePresentations')return '全角色共用'
 if(page.startsWith('admin')||field.startsWith('operator'))return '仅管理员'
 if(page==='authLogin')return '游客、微信用户（登录流程）'
 if(['requests','publish','footprints','requestHistory'].includes(page))return '游客、微信用户'
 if(field==='copy') {
  if(/登录后|未登录|登录再|登录才能/.test(key||''))return '仅游客（登录提示）'
  if(/管理员|工作台|录入|发布房源/.test(key||''))return '仅管理员'
 }
 if((page==='properties'&&field==='title')||(page==='me'&&field==='subtitle'))return '游客、微信用户'
 return '全角色共用'
}
export function configChanges(before: PublicConfig, after: PublicConfig): ConfigChange[] {
 const rows: ConfigChange[]=[]
 const walk=(a:any,b:any,path:string[])=>{
  if(same(a,b))return
  if(path[path.length - 1]==='copy'&&!Object.keys(a||{}).length&&!Object.keys(b||{}).length)return
  if(a&&b&&typeof a==='object'&&typeof b==='object'&&!Array.isArray(a)&&!Array.isArray(b)){for(const key of new Set([...Object.keys(a),...Object.keys(b)]))walk(a[key],b[key],[...path,key]);return}
  const page=path[0]==='pagePresentations'?path[1]:'global'
  const label=path[0]==='pagePresentations'&&path[2]==='copy'?path[3]:path.map(k=>titles[k]||pagePresentationOptions.find(p=>p.key===k)?.label||k).join(' / ')
  rows.push({id:JSON.stringify(path),paths:[path],page,label,roles:changeScope(path),before:a,after:b})
 }
 walk(before,after,[])
 const grouped: ConfigChange[]=[];const used=new Set<string>()
 for(const row of rows){
  const root=row.paths[0][0];let key=''
  if(root==='locations'||root==='featuredChannels')key='coverage'
  if(root==='propertyDetail'&&row.paths[0][1]==='facilities')key='facilities'
  if(root==='listingTitle'&&row.paths[0][1]!=='marketingPhrases')key='tags'
  // Allowed tags constrain share fields; undo the dependency as a whole.
  if(root==='shareCard'&&rows.some(x=>x.paths[0][0]==='listingTitle'&&x.paths[0][1]!=='marketingPhrases'))key='tags'
  if(!key){grouped.push(row);continue}if(used.has(key))continue;used.add(key)
  const members=rows.filter(x=>key==='coverage'?['locations','featuredChannels'].includes(x.paths[0][0]):key==='facilities'?x.paths[0][0]==='propertyDetail'&&x.paths[0][1]==='facilities':(x.paths[0][0]==='listingTitle'&&x.paths[0][1]!=='marketingPhrases')||x.paths[0][0]==='shareCard')
  grouped.push({id:key,paths:members.flatMap(x=>x.paths),page:'global',label:({coverage:'地区与频道（关联修改）',facilities:'房屋设施库（名称、图标、标识关联修改）',tags:'房源标签与分享约束（关联修改）'} as Record<string,string>)[key],roles:[...new Set(members.map(x=>x.roles))].join('；'),before:members.map(x=>({项目:x.label,内容:x.before??'默认'})),after:members.map(x=>({项目:x.label,内容:x.after??'默认'}))})
 }
 return grouped
}
export function restoreChange(target:PublicConfig, source:PublicConfig, change:ConfigChange){
 for(const path of change.paths){let dest:any=target,src:any=source;for(const key of path.slice(0,-1)){dest=dest[key]??=( {} );src=src?.[key]};const key=path[path.length - 1]!;if(src?.[key]===undefined)delete dest[key];else dest[key]=JSON.parse(JSON.stringify(src[key]))}
}
export const changeValue=(value:unknown)=>value===undefined?'使用默认值':typeof value==='string'?(value||'恢复默认／空值'):JSON.stringify(value,null,2)
