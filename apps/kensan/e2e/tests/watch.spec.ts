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

async function metrics(page:Page,current=true){
 await page.route("**/api/v1/watch/metrics",r=>r.fulfill({json:{version:1,repo:"sample/repo",zenn:"writer",publications:{data:{month:"2026-10",items:[{title:"公開した記事",url:"https://zenn.dev/writer/articles/one",at:"2026-10-01T00:00:00+09:00"}]},updatedAt:"2026-10-30T12:00:00+09:00",currentPeriod:current,stale:!current},development:{data:{week:"2026-10-26",items:[{title:"実装を改善する",url:"https://github.com/sample/repo/pull/1",createdAt:"2026-10-28T10:00:00+09:00",mergedAt:"2026-10-28T12:00:00+09:00"}],days:[0,0,1,0,0,0,0],medianHours:2},updatedAt:"2026-10-30T12:00:00+09:00",currentPeriod:current,stale:!current}}}));
 await page.route("**/api/v1/projects/kensan-lab/metrics",r=>r.fulfill({json:{metrics:[{id:"github-stars",current:19,updatedAt:"2026-10-30T07:15:00+09:00",stale:false,delta:{days30:2}}]}}));
}
async function changes(page:Page){
 const long="詳しい検証と変更の経緯。".repeat(80);
 await page.route("**/api/v1/watch",r=>r.fulfill({json:{snapshot:{hash:"new",projects:["sample","learning","writing"].map(name=>({name,path:`projects/${name}/README.md`,date:"2026-10-30",text:`**基盤の更新を完了**\n\n${long}`,next:"動作を確認する"}))},state:{status:"failed",checkedAt:"2026-10-30T12:00:00+09:00"},result:{hash:"old",generatedAt:"2026-10-30T10:00:00+09:00",items:[{project:"sample",headline:"以前のAI見出し",change:"前回成功した要約",meaning:"根拠を確認できる",decision:"",sourceIds:["project:sample"]}],sources:[{id:"project:sample",project:"sample",date:"2026-10-29"}]},stale:true}}));
}

test("常設は意図だけを表示し、表示で既読や原本を書き換えない",async({page})=>{
 await diary(page);await feed(page);await metrics(page);const writes:string[]=[];
 page.on("request",r=>{if(r.method()!=="GET")writes.push(r.method()+" "+r.url())});
 await page.goto("/watch");
 await expect(page.getByRole("heading",{name:"今日と、最近の変化"})).toBeVisible();
 await expect(page.getByText("常設画面を実機で試す",{exact:true})).toBeVisible();
 await expect(page.getByText("PRIVATE DIARY MUST STAY HIDDEN")).toHaveCount(0);
 await expect(page.getByText("まだ確認していない依頼",{exact:true})).toBeVisible();
 await expect(page.getByText("確認済みの依頼",{exact:true})).toHaveCount(0);
 await expect(page.getByTestId("app-sidebar")).toHaveCount(0);
 await expect(page.getByRole("link",{name:"日記を開く"})).toHaveAttribute("href","/daily");
 expect(writes).toEqual([]);
});
test("変化は短い見出しで表示し、遷移先で最新記録と古いAI要約を区別する",async({page})=>{
 await diary(page);await feed(page);await metrics(page);await changes(page);await page.goto("/watch");
 await expect(page.getByText("詳しい検証と変更の経緯。",{exact:false})).toHaveCount(0);
 await expect(page.getByText("以前のAI見出し",{exact:true})).toHaveCount(0);
 await page.getByRole("link",{name:/sample.*基盤の更新を完了/}).click();
 await expect(page).toHaveURL(/\/watch\/changes\/sample$/);
 await expect(page.getByRole("heading",{name:"最新の元の記録"})).toBeVisible();
 await expect(page.getByText("詳しい検証と変更の経緯。",{exact:false})).toBeVisible();
 await expect(page.getByText("前回成功した要約",{exact:true})).toBeVisible();
 await expect(page.getByText("AI要約のあとに記録が変わっています。上の元の記録が最新です。")).toBeVisible();
 await expect(page.getByRole("link",{name:/根拠：sample/})).toHaveAttribute("href","/#project-sample");
});
test("公開数・Stars・週次PRから集計の詳細へ遷移する",async({page})=>{
 await diary(page);await feed(page);await metrics(page);await page.goto("/watch");
 const pub=page.getByRole("link",{name:/今月公開した記事/});await expect(pub).toContainText("1 本");
 await expect(page.getByRole("link",{name:/kensan-lab Stars/})).toContainText("19");
 await expect(page.getByRole("link",{name:/今週のマージPR/})).toContainText("1 件");
 await expect(page.getByRole("group",{name:"曜日別のマージPR数"})).toBeVisible();
 await expect(page.getByRole("img",{name:"10/28（水）: 1件"})).toBeVisible();
 await pub.click();await expect(page).toHaveURL(/\/watch\/activity#publications$/);
 await expect(page.getByRole("link",{name:"公開した記事 ↗"})).toHaveAttribute("href","https://zenn.dev/writer/articles/one");
 await expect(page.getByRole("link",{name:"実装を改善する ↗"})).toHaveAttribute("href","https://github.com/sample/repo/pull/1");
 await expect(page.getByLabel("曜日別のマージPR数")).toBeVisible();
});
test("前月・前週の値を当期の件数として表示しない",async({page})=>{
 await diary(page);await feed(page);await metrics(page,false);await page.goto("/watch");
 await expect(page.getByRole("link",{name:/今月公開した記事/})).toContainText("— 本");
 await expect(page.getByRole("link",{name:/今週のマージPR/})).toContainText("— 件");
 await expect(page.getByText("今期分は未取得")).toHaveCount(2);
});
for(const mode of ["light","dark"] as const){
 test(`${mode}: 見出し3件が1069pxで見え390pxでも横溢れしない`,async({page},info)=>{
  await diary(page);await feed(page);await metrics(page);await changes(page);await page.setViewportSize({width:1069,height:900});await page.goto("/watch");
  await page.evaluate(dark=>document.documentElement.classList.toggle("dark",dark),mode==="dark");
  const last=page.getByRole("link",{name:/writing.*基盤の更新を完了/});await expect(last).toBeVisible();
  const bounds=await last.boundingBox();expect(bounds!.y+bounds!.height).toBeLessThan(900);
  await page.screenshot({path:info.outputPath(`watch-${mode}.png`),fullPage:true});
  await page.setViewportSize({width:390,height:844});expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
  await expect(page.getByText("PRIVATE DIARY MUST STAY HIDDEN")).toHaveCount(0);
 });
}
test("確認済みの取得失敗を未確認0件と表示しない",async({page})=>{
 await diary(page);await feed(page);await metrics(page);
 await page.route("**/api/v1/feeds/acknowledgements",route=>route.fulfill({status:503,json:{error:"temporarily unavailable"}}));await page.goto("/watch");
 await expect(page.getByText("確認済み状態を取得できません。未確認の件数は確定していません。")).toBeVisible({timeout:15000});await expect(page.getByText("件数未確認",{exact:true})).toBeVisible();
});

async function seedDashboardDay(page:Page,date:string,content:string){
 const path=`daily/${date.replaceAll("-","/")}.md`;
 const created=await page.request.post("/api/v1/files",{data:{path,content}});expect(created.status()).toBe(201);
 return (await (await page.request.get(`/api/v1/files/${path}`)).json()) as {content:string;doc:{mtime:string}};
}

test("スマホで今日・明日の予定と一行日記を保存・編集し原本を保つ",async({page})=>{
 await page.clock.install({time:new Date("2030-12-31T13:00:00Z")});await page.setViewportSize({width:390,height:844});await feed(page);await metrics(page);
 const date="2030-12-31",tomorrow="2031-01-01";const before=await seedDashboardDay(page,date,"---\ntype: daily\n---\n## 今日やりたいこと\n\n今日の予定\n\n## 日記\n\nPRIVATE KEEP\n");
 await page.goto("/watch");await expect(page.getByText("今日の予定",{exact:true})).toBeVisible();
 const tabs=await page.getByRole("tablist",{name:"記入する内容"}).boundingBox();const metric=await page.getByRole("link",{name:/今月公開した記事/}).boundingBox();expect(tabs!.y).toBeLessThan(metric!.y);
 await page.getByRole("tab",{name:"明日やりたいことを書く"}).click();await page.getByRole("button",{name:"明日やりたいことを書く",exact:true}).click();
 await page.getByLabel("明日やりたいこと（ダッシュボードに表示）").fill("新年は読書する");await page.getByRole("button",{name:"保存する",exact:true}).click();await expect(page.getByRole("button",{name:"明日やりたいことを編集"})).toBeVisible();
 const nextPath=`daily/${tomorrow.replaceAll("-","/")}.md`;expect((await (await page.request.get(`/api/v1/files/${nextPath}`)).json()).content).toContain("新年は読書する");
 expect((await (await page.request.get("/api/v1/files/daily/2030/12/31.md")).json()).content).toBe(before.content);
 await page.getByRole("button",{name:"明日やりたいことを編集"}).click();await page.getByRole("textbox",{name:/やりたいこと（ダッシュボードに表示）/}).fill("新年は散歩する");await page.getByRole("button",{name:"保存する",exact:true}).click();await expect(page.getByRole("button",{name:"明日やりたいことを編集"})).toBeVisible();
 await page.getByRole("textbox",{name:"一行日記",exact:true}).fill("一年を振り返った");await page.getByRole("button",{name:"日記に残す"}).click();await page.getByRole("button",{name:"今の一行を編集"}).click();await page.getByRole("textbox",{name:"一行日記",exact:true}).fill("今年はよく学べた");await page.getByRole("button",{name:"変更を保存"}).click();await expect(page.getByRole("button",{name:"今の一行を編集"})).toBeVisible();
 const saved=(await (await page.request.get("/api/v1/files/daily/2030/12/31.md")).json()).content;expect(saved).toContain("PRIVATE KEEP");expect(saved).toContain("今日の予定");expect(saved).toContain("今年はよく学べた");expect(saved).not.toContain("一年を振り返った");
 await expect(page.getByText("PRIVATE KEEP",{exact:false})).toHaveCount(0);expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(390);
 await page.reload();await expect(page.getByText("今日の予定",{exact:true})).toBeVisible();await expect(page.getByText("今年はよく学べた",{exact:false})).toHaveCount(0);
});
test("日付が変わっても編集中の予定を別の日へ保存しない",async({page})=>{
 await page.clock.install({time:new Date("2031-03-31T14:59:50Z")});await feed(page);await metrics(page);await seedDashboardDay(page,"2031-03-31","# 2031-03-31\n");
 await page.goto("/watch");await page.getByRole("button",{name:"今日やりたいことを書く",exact:true}).click();await page.getByRole("textbox",{name:/やりたいこと（ダッシュボードに表示）/}).fill("日付をまたぐ入力");
 await page.clock.fastForward(40_000);await expect(page.getByText("日付が変わりました。入力中の 2031-03-31 に保存します。")).toBeVisible();await page.getByRole("button",{name:"保存する",exact:true}).click();
 await expect(page.getByRole("button",{name:"今日やりたいことを書く",exact:true})).toBeVisible();expect((await (await page.request.get("/api/v1/files/daily/2031/03/31.md")).json()).content).toContain("日付をまたぐ入力");expect((await page.request.get("/api/v1/files/daily/2031/04/01.md")).status()).toBe(404);
});
test("記録の読取り警告がプロジェクト一覧から消えない",async({page})=>{
 await diary(page);await feed(page);await metrics(page);
 await page.route("**/api/v1/watch",r=>r.fulfill({json:{snapshot:{hash:"a",projects:[],warnings:["記録を読み取れません: example"]},state:{status:"not_configured"},result:null,stale:false}}));
 await page.goto("/watch");await expect(page.getByText("一部の記録を取得できません",{exact:true})).toBeVisible();await page.getByText("一部の記録を取得できません",{exact:true}).click();await expect(page.getByText("記録を読み取れません: example",{exact:true})).toBeVisible();
});

test("日記画面から追記した本文を常設に出さず予定編集後も保持する",async({page})=>{
 await page.clock.install({time:new Date("2032-05-01T00:00:00Z")});await feed(page);await metrics(page);
 await seedDashboardDay(page,"2032-05-01","---\ntype: daily\n---\n# 2032-05-01\n\n## 今日やりたいこと\n\n散歩する\n");
 await page.goto("/daily");await page.getByRole("textbox",{name:"日記（1 行）"}).fill("PRIVATE JOURNAL ENTRY");await page.getByRole("button",{name:"残す",exact:true}).click();
 await expect(page.getByLabel("今日の日記の本文")).toContainText("PRIVATE JOURNAL ENTRY");
 await page.goto("/watch");await expect(page.getByLabel("今日やりたいことの本文")).toHaveText("散歩する");await expect(page.getByText("PRIVATE JOURNAL ENTRY",{exact:false})).toHaveCount(0);
 await page.getByRole("button",{name:"今日やりたいことを編集"}).click();await page.getByRole("textbox",{name:/やりたいこと（ダッシュボードに表示）/}).fill("本を読む");await page.getByRole("button",{name:"保存する",exact:true}).click();await expect(page.getByLabel("今日やりたいことの本文")).toHaveText("本を読む");
 const saved=(await (await page.request.get("/api/v1/files/daily/2032/05/01.md")).json()).content;expect(saved).toContain("## 日記");expect(saved).toContain("PRIVATE JOURNAL ENTRY");
 await page.getByRole("textbox",{name:"一行日記",exact:true}).fill("PRIVATE SECOND ENTRY");await page.getByRole("button",{name:"日記に残す"}).click();await expect(page.getByRole("textbox",{name:"一行日記",exact:true})).toHaveValue("");await expect(page.getByLabel("今日やりたいことの本文")).toHaveText("本を読む");await expect(page.getByText("PRIVATE SECOND ENTRY",{exact:false})).toHaveCount(0);
});
