// Optional browser check: supply a generated report URL and local Playwright.
const {createRequire}=require('node:module');
const fs=require('node:fs');
const path=require('node:path');
const assert=require('node:assert/strict');
const loader=createRequire(path.resolve(process.env.LLMOPS_PLAYWRIGHT_MODULE||'package.json'));
const {chromium}=loader(process.env.LLMOPS_PLAYWRIGHT_MODULE||'playwright');
(async()=>{
 const url=process.argv[2];assert(url,'generated report URL required');
 const dir=process.env.LLMOPS_REPORT_SCREENSHOTS||'temp/report-browser';fs.mkdirSync(dir,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.LLMOPS_BROWSER_EXECUTABLE?{executablePath:process.env.LLMOPS_BROWSER_EXECUTABLE}:{})});
 try{
  const page=await browser.newPage();const external=[];
  page.on('request',r=>{if(new URL(r.url()).origin!==new URL(url).origin)external.push(r.url())});
  for(const width of [390,1440]){
   await page.setViewportSize({width,height:1000});
   for(const theme of ['light','dark']){
    await page.emulateMedia({colorScheme:theme});const response=await page.goto(url);assert.equal(response.status(),200);
    assert.equal(await page.locator('.case').count(),4);
    const result=await page.evaluate(()=>({overflow:document.documentElement.scrollWidth>innerWidth,rawRan:window.__rawRan,modelRan:window.__modelRan,active:document.querySelectorAll('script,img,iframe,link').length}));
    assert.equal(result.overflow,false);assert.equal(result.rawRan,undefined);assert.equal(result.modelRan,undefined);assert.equal(result.active,0);
    await page.screenshot({path:path.join(dir,`${width}-${theme}.png`),fullPage:width===1440});
    console.log(`${width} ${theme}: no overflow, no active model/source content`);
   }
  }
  assert.deepEqual(external,[]);console.log('no external resource requests');
 }finally{await browser.close()}
})().catch(e=>{console.error(e);process.exit(1)});
