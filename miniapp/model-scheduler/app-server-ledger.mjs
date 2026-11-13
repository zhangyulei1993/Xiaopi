import {createHash} from 'node:crypto'
import {link,mkdir,mkdtemp,rm,writeFile} from 'node:fs/promises'
import {join} from 'node:path'

export async function recordTurnUsage(directory, usage) {
  const event = 'terminal-turn-usage'
  const key = createHash('sha256').update(JSON.stringify([usage.threadId,usage.turnId,event])).digest('hex')
  await mkdir(directory,{recursive:true,mode:0o700})
  const temporary = await mkdtemp(join(directory,'.pending-'))
  try {
    const entry=join(temporary,'entry')
    await writeFile(entry,JSON.stringify({...usage,event})+'\n',{mode:0o600})
    try {
      await link(entry,join(directory,key+'.json'))
      return true
    } catch(error) {
      if(error.code==='EEXIST') return false
      throw error
    }
  } finally {
    await rm(temporary,{recursive:true,force:true})
  }
}

export async function persistObservedUsage(directory, usage) {
  const unavailable = {
    turnTokens:'UNAVAILABLE',
    eventDelta:'UNAVAILABLE',
    actualCost:'UNAVAILABLE',
  }
  if (!usage) return {...unavailable, threadTotal:'UNAVAILABLE', status:'UNAVAILABLE'}
  const result = {...unavailable, threadTotal:usage}
  if (!directory) return {...result,status:'DISABLED'}
  try {
    const recorded = await recordTurnUsage(directory,usage)
    return {...result,status:recorded?'RECORDED':'ALREADY_RECORDED'}
  } catch (error) {
    return {...result,status:'FAILED',error:error.message}
  }
}
