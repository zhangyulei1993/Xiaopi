import fs from 'node:fs'
import path from 'node:path'
import ts from 'typescript'
import { fileURLToPath } from 'node:url'

const projectRoot=path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const expectedDeclarations={request:path.join(projectRoot,'src/api/request-url.mjs'), wrapper:path.join(projectRoot,'src/api/client.ts'), upload:path.join(projectRoot,'src/api/upload-url.mjs'), download:path.join(projectRoot,'src/api/download-url.mjs')}
const sameFile=(actual, expected)=>{ try { return fs.realpathSync(actual) === fs.realpathSync(expected) } catch { return false } }
const sameRuntimeModule=(actual, expected)=>sameFile(actual, expected) || (actual.endsWith('.d.mts') && sameFile(actual.slice(0,-6)+'.mjs', expected))

export function auditSource(file, source) {
  const blocks = file.endsWith('.vue') ? [...source.matchAll(/<script\b[^>]*>([\s\S]*?)<\/script\s*>/gi)].map(m => ({text:m[1], offset:m.index + m[0].indexOf(m[1])})) : [{text:source,offset:0}]
  const calls=[]
  for (const block of blocks) {
    const virtualFile=path.resolve(file.endsWith('.vue') ? `${file}.ts` : file)
    const options={allowJs:true,checkJs:false,module:ts.ModuleKind.ESNext,moduleResolution:ts.ModuleResolutionKind.Bundler,target:ts.ScriptTarget.ES2020,noEmit:true,skipLibCheck:true}
    const host=ts.createCompilerHost(options)
    const originalGetSourceFile=host.getSourceFile.bind(host)
    host.getSourceFile=(name,languageVersion,onError,shouldCreateNewSourceFile)=>path.resolve(name)===virtualFile
      ? ts.createSourceFile(virtualFile,block.text,languageVersion,true,ts.ScriptKind.TS)
      : originalGetSourceFile(name,languageVersion,onError,shouldCreateNewSourceFile)
    const program=ts.createProgram([virtualFile],options,host)
    const checker=program.getTypeChecker()
    const ast=program.getSourceFile(virtualFile)
    if (!ast) throw new Error(`TypeScript could not parse ${file}`)
    function visit(node) {
      if (ts.isCallExpression(node) || ts.isNewExpression(node)) {
        const callee=node.expression.getText(ast)
        if (/^(?:uni|wx)\.(?:request|uploadFile|downloadFile|connectSocket)$/.test(callee) || callee === 'fetch' || callee === 'XMLHttpRequest') {
          let arg=node.arguments?.[0]
          while (arg && (ts.isAsExpression(arg) || ts.isParenthesizedExpression(arg) || ts.isTypeAssertionExpression(arg))) arg=arg.expression
          const url=arg && ts.isObjectLiteralExpression(arg) ? arg.properties.find(p=>ts.isPropertyAssignment(p)&&p.name.getText(ast)==='url') : undefined
          const expression=url && ts.isPropertyAssignment(url) ? url.initializer.getText(ast) : undefined
          const line=source.slice(0,block.offset+node.getStart(ast)).split('\n').length
          const expected=callee === 'uni.request' ? 'safeRequestUrl' : callee === 'uni.uploadFile' ? 'safeUploadUrl' : callee === 'uni.downloadFile' ? 'safeDownloadUrl' : null
          const direct=url && ts.isPropertyAssignment(url) && ts.isCallExpression(url.initializer) && ts.isIdentifier(url.initializer.expression) && url.initializer.expression.text===expected
          const spreadAfter=url && ts.isObjectLiteralExpression(arg) && arg.properties.slice(arg.properties.indexOf(url)+1).some(p=>ts.isSpreadAssignment(p) || (ts.isPropertyAssignment(p) && p.name.getText(ast)==='url'))
          const ref=direct ? url.initializer.expression : undefined
          const symbol=ref && checker.getSymbolAtLocation(ref)
          let reassigned=false
          if (symbol) {
            const findWrites = candidate => {
              if (ts.isBinaryExpression(candidate) && candidate.operatorToken.kind >= ts.SyntaxKind.FirstAssignment && candidate.operatorToken.kind <= ts.SyntaxKind.LastAssignment && ts.isIdentifier(candidate.left) && checker.getSymbolAtLocation(candidate.left) === symbol) reassigned=true
              if ((ts.isPrefixUnaryExpression(candidate) || ts.isPostfixUnaryExpression(candidate)) && ts.isIdentifier(candidate.operand) && checker.getSymbolAtLocation(candidate.operand) === symbol) reassigned=true
              ts.forEachChild(candidate,findWrites)
            }
            findWrites(ast)
          }
          let resolved=symbol
          if (symbol && (symbol.flags & ts.SymbolFlags.Alias)) {
            try { resolved=checker.getAliasedSymbol(symbol) } catch { resolved=undefined }
          }
          const declarations=(resolved?.declarations || []).map(d=>({file:d.getSourceFile().fileName,kind:ts.SyntaxKind[d.kind],name:resolved.getName()}))
          let wrapperBinding = null
          if ((callee === 'uni.uploadFile' || callee === 'uni.downloadFile') && resolved?.declarations?.length === 1) {
            const declaration = resolved.declarations[0]
            const init = ts.isVariableDeclaration(declaration) ? declaration.initializer : undefined
            const body = init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init)) ? init.body : undefined
            const returned = body && ts.isCallExpression(body) ? body : undefined
            const target = returned && ts.isIdentifier(returned.expression) ? checker.getSymbolAtLocation(returned.expression) : undefined
            let actual = target
            if (target && (target.flags & ts.SymbolFlags.Alias)) { try { actual=checker.getAliasedSymbol(target) } catch { actual=undefined } }
            wrapperBinding = { directReturn:!!returned, importAlias:!!(target && (target.flags & ts.SymbolFlags.Alias)), declarations:(actual?.declarations || []).map(d=>({file:d.getSourceFile().fileName,kind:ts.SyntaxKind[d.kind],name:actual.getName()})) }
          }
          const originVerified=declarations.length===1 && (callee==='uni.request'
            ? sameRuntimeModule(declarations[0].file,expectedDeclarations.request)
            : callee==='uni.uploadFile'
              ? sameFile(declarations[0].file,expectedDeclarations.wrapper) && wrapperBinding?.directReturn && wrapperBinding.importAlias && wrapperBinding.declarations.length===1 && sameRuntimeModule(wrapperBinding.declarations[0].file,expectedDeclarations.upload)
              : callee==='uni.downloadFile' && sameFile(declarations[0].file,expectedDeclarations.wrapper) && wrapperBinding?.directReturn && wrapperBinding.importAlias && wrapperBinding.declarations.length===1 && sameRuntimeModule(wrapperBinding.declarations[0].file,expectedDeclarations.download))
          calls.push({file,line,callee,urlExpression:expression ?? null,directGuard:!!direct&&!spreadAfter,reassigned,importAlias:!!(symbol && (symbol.flags & ts.SymbolFlags.Alias)),resolvedDeclarations:declarations,wrapperBinding,originVerified:!!originVerified,status:'BLOCKED',reason:direct&&!spreadAfter&&!reassigned&&originVerified?'Resolved declaration files match; guard implementation, cross-file writes and built output require verification':'URL binding, declaration origin or reassignment could not be verified'})
        }
      }
      ts.forEachChild(node,visit)
    }
    visit(ast)
  }
  return calls
}
export function auditTree(dir) {
  const found=[]
  function walk(d) { for(const e of fs.readdirSync(d,{withFileTypes:true})) {const f=path.join(d,e.name); if(e.isDirectory()) walk(f); else if(/\.(?:vue|ts|js|mjs)$/.test(e.name)) found.push(...auditSource(f,fs.readFileSync(f,'utf8'))) } }
  walk(dir); return found
}
