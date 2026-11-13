export function executionPlan({full, rolesOnly, roleRetry, visual}) {
  return [
    'build-and-security',
    ...(!rolesOnly && !full ? [] : roleRetry ? ['role-ui'] : ['role-api', 'role-ui']),
    ...(!rolesOnly ? ['business-e2e'] : []),
    ...(full || visual ? ['core-ui'] : []),
    ...(full ? ['extended-ui'] : []),
  ]
}

export async function executePlan(plan, execute, onError) {
  for (const name of plan) {
    try {
      await execute(name)
    } catch (error) {
      onError(name, error)
      if (name === 'build-and-security') break
    }
  }
}

export function requiredExit(plan, suites) {
  return plan.every(name => suites[name]?.status === 'PASS') ? 0 : 1
}

export function permissionRows(initial, followup, key) {
  const newer = new Map(followup.filter(row => !row.carried_forward).map(row => [key(row), row]))
  return initial.map(row => newer.get(key(row)) ?? row)
}

export function evidenceIsCurrent(stats, started) {
  return stats.isFile() && stats.birthtimeMs >= started - 1 && stats.mtimeMs >= started - 1
}

export function testRecord(row, result, testId, executionId) {
  const status = (row.status ?? 'NOT_RUN').replaceAll(' ', '_')
  const current = Boolean(result?.startedAt && status !== 'NOT_RUN' && status !== 'NOT_APPLICABLE' && !row.carried_forward)
  const executed = status === 'PASS' || status === 'FAIL'
  const assertion = row.assertion_status ?? (status === 'PASS' ? 'PASS' :
    status === 'FAIL' && row.failureType === 'ASSERTION' ? 'FAIL' : 'NOT_RUN')
  return {
    test_id: testId, execution_id: executionId,
    execution_status: current ? row.execution_status ?? (executed ? 'COMPLETED' : 'INCOMPLETE') : 'NOT_RUN',
    assertion_status: current ? assertion : 'NOT_RUN',
    status: current ? status : status === 'NOT_APPLICABLE' ? status : 'NOT_RUN',
    started_at: current ? result.startedAt : null,
    finished_at: current ? result.finishedAt : null,
    evidence: current ? result.evidence ?? result.log ?? null : null,
    from_current_execution: current,
  }
}
