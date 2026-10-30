import { expect, test, type Page } from "@playwright/test";

async function diary(page: Page) {
 await page.route("**/api/v1/files/daily/**", route => route.fulfill({ json: {
  doc: { path: "daily/example.md", mtime: "2026-10-30T00:00:00Z", meta: { type: "daily" } },
  content: "---\ntype: daily\n---\n## 今日やりたいこと\n常設画面を実機で試す\n## 日記\nPRIVATE DIARY MUST STAY HIDDEN",
 } }));
}
async function feed(page: Page) {
 await page.route("**/api/v1/feeds/latest", route => route.fulfill({ json: {
  stale: false, state: null,
  feed: {date:"2026-10-30", title:"Fixture",path:"feeds/2026/10/30.md",generatedAt:"2026-10-30T08:00:00+09:00",content:"## 確認\n### 確認済みの依頼\n出典: https://example.com/done\n受信日時: v1\n### まだ確認していない依頼\n出典: https://example.com/new\n受信日時: v1\n"},
 } }));
 await page.route("**/api/v1/feeds/acknowledgements", route => route.fulfill({ json: {items:[{key:"https://example.com/done",title:"確認済みの依頼",version:"v1"}]} }));
}

test("常設は意図だけを表示し、既存記録をAI未設定でも読める", async ({ page }) => {
 await diary(page); await feed(page); const writes: string[]=[];
 page.on("request",r=>{if(r.method()!=="GET")writes.push(r.method()+" "+r.url())});
 await page.goto("/watch");
 await expect(page.getByRole("heading",{name:"今日と、最近の変化"})).toBeVisible();
 await expect(page.getByText("常設画面を実機で試す",{exact:true})).toBeVisible();
 await expect(page.getByText("PRIVATE DIARY MUST STAY HIDDEN")).toHaveCount(0);
 await expect(page.getByText("AI更新は設定前",{exact:true})).toBeVisible();
 await expect(page.getByText("まだ確認していない依頼",{exact:true})).toBeVisible();
 await expect(page.getByText("確認済みの依頼",{exact:true})).toHaveCount(0);
 await expect(page.getByTestId("app-sidebar")).toHaveCount(0);
 await expect(page.getByRole("link",{name:"日記を開く"})).toHaveAttribute("href","/daily");
 expect(writes).toEqual([]);
});

test("AI失敗でも前回要約と元の記録を保持し、古い要約と区別する", async ({ page }) => {
 await diary(page);await feed(page);
 await page.route("**/api/v1/watch",route=>route.fulfill({json:{snapshot:{hash:"new",projects:[{name:"sample",path:"projects/sample/README.md",date:"2026-10-30",text:"最新の元の記録",next:"実機で試す"}]},state:{status:"failed",checkedAt:"2026-10-30T12:00:00+09:00",reason:"AI更新に失敗しました"},result:{hash:"old",generatedAt:"2026-10-30T10:00:00+09:00",items:[{project:"sample",change:"前回成功した要約",meaning:"根拠を確認できる",decision:"",sourceIds:["project:sample"]}],sources:[{id:"project:sample",project:"sample",path:"projects/sample/README.md",date:"2026-10-29"}]},stale:true}}));
 await page.goto("/watch");
 await page.getByText("前回成功したAI要約",{exact:true}).click();
 await expect(page.getByText("前回成功した要約",{exact:true}).first()).toBeVisible();
 await page.getByText("要約と根拠",{exact:true}).click();
 await expect(page.getByText("最新の元の記録",{exact:true}).first()).toBeVisible();
 await expect(page.getByText("AI要約のあとに記録が変わっています。下の元の記録が最新です。")).toBeVisible();
 await expect(page.getByRole("link",{name:/根拠：sample/})).toHaveAttribute("href","/#project-sample");
});
for(const mode of ["light","dark"] as const) {
 test(`${mode}: 常設は390pxで横にはみ出さない`,async({page})=>{
  await diary(page);await feed(page);await page.setViewportSize({width:390,height:844});await page.goto("/watch");
  await page.evaluate(dark=>document.documentElement.classList.toggle("dark",dark),mode==="dark");
  await expect(page.getByRole("heading",{name:"今日と、最近の変化"})).toBeVisible();
  expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 });
}

// Reproduces nested Markdown expanding past the old clamp on the real watch page.
for (const mode of ["light", "dark"] as const) {
 test(`${mode}: 長い記録とFeedを常設カード内に収める`, async ({page}, testInfo) => {
  await diary(page); await feed(page);
  const long = "最近の進捗と次に確認すること。".repeat(70);
  await page.route("**/api/v1/watch", route => route.fulfill({json:{
   snapshot:{hash:"new",projects:["platform","learning","writing"].map(name=>({name,date:"2026-10-30",text:`**更新の要点** ${long}\n\n- 詳しい変更\n- 検証の記録`,next:"動作を確認する"}))},
   state:{status:"unchanged",checkedAt:"2026-10-30T12:00:00+09:00"},
   result:{hash:"old",generatedAt:"2026-10-30T10:00:00+09:00",items:[{project:"platform",change:"以前のAI要約",meaning:"詳細",sourceIds:[]}],sources:[]},stale:true
  }}));
  await page.route("**/api/v1/feeds/latest",route=>route.fulfill({json:{stale:false,feed:{date:"2026-10-30",generatedAt:"2026-10-30T08:00:00+09:00",content:["## 確認",...[1,2,3].map(n=>`### 確認するお知らせ ${n}\n結論: ${long}\n\n- **ポイント**\n  - 詳細な説明\n- 出典: https://example.com/${n}`)].join("\n")}}}));
  await page.setViewportSize({width:1069,height:900}); await page.goto("/watch");
  await page.evaluate(dark=>document.documentElement.classList.toggle("dark",dark),mode==="dark");
  await expect(page.getByText("記録更新あり",{exact:true})).toBeVisible();
  await expect(page.getByRole("heading",{name:"確認するお知らせ 3"})).toBeVisible();
  const last = await page.getByRole("heading",{name:"確認するお知らせ 3"}).boundingBox();
  expect(last!.y + last!.height).toBeLessThan(900);
  const bounds = await page.locator("p.line-clamp-2").first().boundingBox();
  expect(bounds!.height).toBeLessThanOrEqual(48);
  await expect(page.getByText("以前のAI要約",{exact:true}).first()).not.toBeVisible();
  await expect(page.getByText("詳しい変更",{exact:true}).first()).not.toBeVisible();
  await expect(page.getByText("PRIVATE DIARY MUST STAY HIDDEN")).toHaveCount(0);
  await page.screenshot({path:testInfo.outputPath(`watch-${mode}.png`),fullPage:true});
  await page.getByText("記録の全文",{exact:true}).first().click();
  await expect(page.getByText("詳しい変更",{exact:true}).first()).toBeVisible();
  await page.setViewportSize({width:390,height:844});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 });
}
test("確認済みの取得失敗を未確認0件と表示しない",async({page})=>{
 await diary(page); await feed(page);
 await page.route("**/api/v1/feeds/acknowledgements",route=>route.fulfill({status:503,json:{error:"temporarily unavailable"}}));
 await page.goto("/watch");
 await expect(page.getByText("確認済み状態を取得できません。未確認の件数は確定していません。")).toBeVisible({timeout:15000});
 await expect(page.getByText("件数未確認",{exact:true})).toBeVisible();
});
