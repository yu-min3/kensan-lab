// Real browser + Go server + live public site search/import. No AI inference.
import assert from 'node:assert/strict';
import {spawn} from 'node:child_process';
import {mkdir,mkdtemp,writeFile} from 'node:fs/promises';
import path from 'node:path';import net from 'node:net';import {fileURLToPath} from 'node:url';
import {chromium} from '../frontend/node_modules/playwright/index.mjs';
const app=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..');assert(process.env.KONRO_TEST_BINARY);
await mkdir(path.join(app,'temp'),{recursive:true});const artifacts=await mkdtemp(path.join(app,'temp','discovery-e2e-'));const data=path.join(artifacts,'recipes');await mkdir(data);
const port=await new Promise(resolve=>{const s=net.createServer();s.listen(0,'127.0.0.1',()=>{const p=s.address().port;s.close(()=>resolve(p))})});const base=`http://127.0.0.1:${port}`;
const server=spawn(process.env.KONRO_TEST_BINARY,[],{env:{...process.env,KONRO_DATA_DIR:data,KONRO_STATIC_DIR:path.join(app,'frontend/dist'),KONRO_ADDR:`127.0.0.1:${port}`},stdio:'ignore'});
let browser;const checks=[];
try{
 for(let i=0;i<100;i++){try{if((await fetch(base+'/healthz')).ok)break}catch{}await new Promise(r=>setTimeout(r,100))}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROME_BINARY||undefined});const page=await browser.newPage({viewport:{width:390,height:844}});const errors=[];page.on('pageerror',e=>errors.push(String(e)));
 const button=name=>page.getByRole('button',{name,exact:true});
 for(const scheme of ['light','dark']){
  await page.emulateMedia({colorScheme:scheme});await page.goto(base);await button('Webで探す').click();
  await page.getByLabel('検索するサイト').selectOption('delish');await page.getByLabel('料理名・材料').fill('肉じゃが');
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:path.join(artifacts,`search-${scheme}.png`),fullPage:true});await button('一覧へ').click();
 }
 checks.push('light/dark Web search form and mobile width');
 await button('Webで探す').click();await button('検索する').click();
 await page.getByRole('heading',{name:'「肉じゃが」の検索結果',exact:true}).waitFor({timeout:30000});
 assert((await page.locator('.web-results > li').count())>0);
 const first=page.locator('.web-results > li').first();assert((await first.locator('h3').innerText()).includes('肉じゃが'));
 const url=await first.getByRole('link',{name:'元のページを見る'}).getAttribute('href');assert(new URL(url).hostname==='delishkitchen.tv');
 await page.waitForFunction(()=>{const img=document.querySelector('.web-recipe-photo img');return img?.naturalWidth>0});
 await page.screenshot({path:path.join(artifacts,'results-dark.png')});
 await first.getByRole('button',{name:/を取り込む$/}).click();assert.equal(await page.getByLabel('レシピのURL').count(),0);
 await page.getByRole('group',{name:'レシピを確認・編集'}).waitFor({timeout:20000});
 assert((await page.getByLabel('材料（1行に1項目）',{exact:true}).inputValue()).length>0);assert((await page.getByLabel('手順（1行に1つの手順）',{exact:true}).inputValue()).length>0);
 checks.push('native Delish search → selected canonical URL → real fetch → structured recipe draft');
 await page.getByLabel('タイトル',{exact:true}).fill('検索から直した下書き');await button('下書きを保存して戻る').click();
 await page.getByRole('heading',{name:'「肉じゃが」の検索結果',exact:true}).waitFor();await page.locator('.web-results > li').first().getByRole('button',{name:/を取り込む$/}).click();
 await page.getByRole('group',{name:'レシピを確認・編集'}).waitFor();assert.equal(await page.getByLabel('タイトル',{exact:true}).inputValue(),'検索から直した下書き');
 checks.push('same URL selection restores edited draft and search results');
 const firstDraftID=await page.evaluate(()=>localStorage.getItem('konro.import-draft'));
 await button('検索結果へ').click();await page.locator('.web-results > li').nth(1).getByRole('button',{name:/を取り込む$/}).click();
 await page.getByRole('group',{name:'レシピを確認・編集'}).waitFor({timeout:20000});await button('別のレシピを取り込む').click();
 const draftID=firstDraftID;let failRestore=true;
 await page.route(`**/api/v1/import/drafts/${draftID}`,route=>{if(failRestore&&route.request().method()==='GET'){failRestore=false;return route.fulfill({status:503,body:'復元を一時的に取得できません'})}return route.continue()});
 await page.getByText(/保存した下書き/).click();await button('検索から直した下書き').click();
 await page.getByRole('alert').getByText('復元を一時的に取得できません',{exact:true}).waitFor();
 await page.getByText(/保存した下書き/).click();await button('検索から直した下書き').click();
 await page.getByRole('group',{name:'レシピを確認・編集'}).waitFor();assert.equal(await page.getByLabel('タイトル',{exact:true}).inputValue(),'検索から直した下書き');
 await page.unroute(`**/api/v1/import/drafts/${draftID}`);
 checks.push('another URL retains the saved draft; retrying the same draft after GET failure succeeds');
 await page.getByRole('checkbox',{name:'分量・時間・温度と手順を確認しました'}).check();await button('レシピとして保存').click();await page.getByRole('button',{name:'検索から直した下書きを開く',exact:true}).waitFor();
 const recipes=await(await fetch(base+'/api/v1/recipes')).json();assert.equal(recipes.length,1);assert.equal(recipes[0].source,url);
 checks.push('confirmed search recipe saves with original source URL');
 await button('Webで探す').click();await page.route('**/api/v1/discovery/search?**',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({results:[],warnings:['試験サイトを検索できませんでした。URLから取り込めます']})}));
 await page.getByLabel('料理名・材料').fill('見つからない料理');await button('検索する').click();await page.getByText('候補が見つかりませんでした。言葉やサイトを変えるか、URLから取り込んでください。',{exact:true}).waitFor();
 await page.getByText('試験サイトを検索できませんでした。URLから取り込めます',{exact:true}).waitFor();
 checks.push('empty and unavailable sources are visible with URL fallback');
 await page.unroute('**/api/v1/discovery/search?**');
 await page.route('**/api/v1/discovery/search?**',async route=>{const q=new URL(route.request().url()).searchParams.get('q');await new Promise(r=>setTimeout(r,q==='先の検索'?500:20));try{await route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({results:[{title:q,url:'https://delishkitchen.tv/recipes/123',site:'delish',siteName:'デリッシュキッチン'}],warnings:[]})})}catch{}});
 await page.getByLabel('料理名・材料').fill('先の検索');await button('検索する').click();await page.getByLabel('料理名・材料').fill('後の検索');await button('検索し直す').click();
 await page.getByRole('heading',{name:'後の検索',exact:true}).waitFor();await page.waitForTimeout(700);assert.equal(await page.getByRole('heading',{name:'先の検索',exact:true}).count(),0);
 checks.push('late response cannot replace a newer search');
 await page.unroute('**/api/v1/discovery/search?**');
 await page.route('**/api/v1/discovery/search?**',route=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify({results:[{title:'写真がない料理',url:'https://delishkitchen.tv/recipes/567',site:'delish',siteName:'デリッシュ'},{title:'とても長い料理名'.repeat(8),url:'https://delishkitchen.tv/recipes/568',site:'delish',siteName:'デリッシュ',image:'https://image.invalid/missing.jpg'}],warnings:[]})}));
 for(const scheme of ['light','dark']) {
  await page.emulateMedia({colorScheme:scheme});await page.setViewportSize({width:320,height:700});
  await page.getByLabel('料理名・材料').fill('写真なし');await button('検索する').click();await page.getByRole('heading',{name:'「写真なし」の検索結果',exact:true}).waitFor();
  await page.waitForFunction(()=>document.querySelectorAll('.web-recipe-no-photo').length===2);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth),false);
  await page.screenshot({path:path.join(artifacts,`fallback-${scheme}.png`),fullPage:true});
 }
 checks.push('missing/broken photos retain selectable cards and long titles fit 320px in both themes');
 assert.deepEqual(errors,[]);await writeFile(path.join(artifacts,'results.json'),JSON.stringify({status:'passed',checks,browserErrors:errors},null,2));console.log(JSON.stringify({status:'passed',checks:checks.length,artifacts}));
}finally{await browser?.close();server.kill('SIGTERM');console.error('Artifacts:',artifacts)}
