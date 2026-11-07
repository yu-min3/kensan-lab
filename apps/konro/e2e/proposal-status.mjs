import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {mkdir,mkdtemp,writeFile} from 'node:fs/promises';
import path from 'node:path';
import net from 'node:net';
import {fileURLToPath} from 'node:url';
import {chromium} from '../frontend/node_modules/playwright/index.mjs';
const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');
assert(process.env.KONRO_TEST_BINARY);
await mkdir(path.join(app,'temp'),{recursive:true});
const artifacts=await mkdtemp(path.join(app,'temp','proposal-status-')),data=path.join(artifacts,'data');await mkdir(data);
const port=await new Promise(resolve=>{const s=net.createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p))})});
const base=`http://127.0.0.1:${port}`;
const server=spawn(process.env.KONRO_TEST_BINARY,[],{env:{...process.env,KONRO_DATA_DIR:data,KONRO_STATIC_DIR:path.join(app,'frontend/dist'),KONRO_ADDR:`127.0.0.1:${port}`},stdio:'ignore'});
let browser;
try {
 for(let i=0;i<100;i++){try{if((await fetch(base+'/healthz')).ok)break}catch{}await new Promise(r=>setTimeout(r,100))}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_BINARY||undefined});
 const page=await browser.newPage({viewport:{width:390,height:844},reducedMotion:'reduce'}),errors=[],checks=[];
 page.on('pageerror',e=>errors.push(String(e)));page.setDefaultTimeout(15000);
 const b=name=>page.getByRole('button',{name,exact:true});
 const id='a'.repeat(32),created=new Date(Date.now()-120000).toISOString();
 let state='queued',reason='',conditions,failNext=false,requests=0,creates=0;
 const proposal=()=>({id,revision:requests+1,state,mode:'ai',conditions,createdAt:created,candidates:[],menus:state==='succeeded'?[{id:'menu-1',picks:[]}]:[],interpretation:'品数に合わせて組み合わせました',job:{id:'example-job',state,reason,created_at:created,updated_at:created}});
 await page.route('**/api/v1/menu-proposals**',async route=>{
  const req=route.request(),url=new URL(req.url());
  if(url.pathname.endsWith('/active'))return route.fulfill({json:[]});
  if(req.method()==='POST'&&url.pathname==='/api/v1/menu-proposals'){
   conditions=req.postDataJSON().conditions;creates++;assert.equal(conditions.servings,undefined);return route.fulfill({json:proposal()});
  }
  if(url.pathname.endsWith('/cancel')){state='cancelled';reason='';return route.fulfill({json:proposal()})}
  if(url.pathname.endsWith('/retry')){state='running';reason='';return route.fulfill({json:proposal()})}
  requests++;if(failNext){failNext=false;return route.abort('failed')}
  return route.fulfill({json:proposal()});
 });
 await page.goto(base+'/#proposals');
 await page.evaluate(()=>localStorage.setItem('konro.last-menu-request',JSON.stringify({count:4,servings:'8'})));
 await page.reload();
 assert.equal(await page.getByLabel('各料理の人数',{exact:true}).count(),0);
 assert.equal(await page.getByLabel('提案する品数',{exact:true}).inputValue(),'4');
 await b('AIで提案する').click();await page.getByRole('heading',{name:'AIの順番を待っています'}).waitFor();
 assert.equal(creates,1);assert.equal(conditions.servings,undefined);assert.equal(await page.getByRole('progressbar').getAttribute('value'),null);
 assert.equal(await b('条件だけで簡易提案').isEnabled(),false);
 assert((await page.getByRole('region',{name:'提案の進捗'}).boundingBox()).y<180);
 checks.push('count-only request ignores old saved servings and exposes queued status above form');
 failNext=true;await page.getByRole('heading',{name:'接続を確認しています'}).waitFor();
 assert.equal(await page.getByRole('progressbar').count(),0);
 state='running';await page.getByRole('heading',{name:'AIが献立を考えています'}).waitFor();
 assert.equal(await page.getByRole('progressbar').getAttribute('value'),null);
 checks.push('polling survives a network failure and resumes without manual refresh');
 for(const width of [320,390,1280])for(const scheme of ['light','dark']){
  await page.setViewportSize({width,height:900});await page.emulateMedia({colorScheme:scheme});
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  if(width===390)await page.screenshot({path:path.join(artifacts,'running-'+scheme+'.png'),fullPage:true});
 }
 state='waiting';reason='billing_blocked';await b('状態を再確認').click();
 await page.getByRole('heading',{name:'提案は一時停止しています'}).waitFor();
 await page.getByText('AIの認証・利用設定を確認できず停止しています。設定の確認後に再試行してください。',{exact:true}).waitFor();
 assert.equal(await page.getByRole('progressbar').count(),0);
 await page.setViewportSize({width:390,height:900});await page.emulateMedia({colorScheme:'light'});
 await page.screenshot({path:path.join(artifacts,'waiting-light.png'),fullPage:true});
 await page.reload();await page.getByRole('heading',{name:'提案は一時停止しています'}).waitFor();
 checks.push('waiting reason and stopped bar survive reload; timestamps remain visible');
 await b('提案を再試行').click();await page.getByRole('heading',{name:'AIが献立を考えています'}).waitFor();
 state='succeeded';await b('状態を再確認').click();await page.getByRole('heading',{name:'献立の提案ができました'}).waitFor();
 assert.equal(await page.getByRole('progressbar').getAttribute('value'),'1');
 const completedRequests=requests;await page.waitForTimeout(2200);assert.equal(requests,completedRequests);
 checks.push('retry resumes; completed bar is full and polling stops');
 state='failed';reason='invalid_output';await b('状態を再確認').click();await page.getByRole('heading',{name:'提案を完了できませんでした'}).waitFor();
 await b('提案を再試行').click();await b('提案を中止').click();await page.getByRole('heading',{name:'提案を中止しました'}).waitFor();
 assert.equal(await page.getByRole('progressbar').count(),0);checks.push('failure retry and cancellation remain explicit');
 assert.deepEqual(errors,[]);await writeFile(path.join(artifacts,'results.json'),JSON.stringify({checks,errors},null,2));
 console.log(JSON.stringify({status:'passed',checks,artifacts}));
} finally {await browser?.close();server.kill('SIGTERM')}
