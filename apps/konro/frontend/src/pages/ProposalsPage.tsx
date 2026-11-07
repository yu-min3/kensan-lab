import {useEffect,useRef,useState} from 'react';
import {MealPlan} from '../api';
import {Conditions, Proposal, proposalRequest as request, isActiveProposal} from '../proposals';
import {ProposalProgress} from '../components/ProposalProgress';
import {Panel} from '../components/Editor';
import {MealPreferences} from '../components/MealPreferences';
import {feedbackRequest} from '../mealFeedback';
import {CookHistory} from '../components/CookHistory';
function previous():Partial<Conditions>{try{return JSON.parse(localStorage.getItem('konro.last-menu-request')??'{}')}catch{return {}}}
export function ProposalsPage({id,selected,onBack,onOpen,onAdopt}:{id:string;selected:string[];onBack:()=>void;onOpen:(id:string)=>void;onAdopt:(id:string)=>void}){
 const [initial]=useState(previous),[idea,setIdea]=useState(initial.idea??''),[count,setCount]=useState(initial.count?String(initial.count):'4'),[main,setMain]=useState(initial.main?String(initial.main):''),[side,setSide]=useState(initial.side?String(initial.side):''),[cuisine,setCuisine]=useState(initial.cuisine??''),[avoid,setAvoid]=useState((initial.avoid??[]).join('\n')),[use,setUse]=useState((initial.use??[]).join('\n')),[fixed,setFixed]=useState<string[]>(selected),[omit,setOmit]=useState<string[]>([]),[seed,setSeed]=useState(initial.seed??0),[proposal,setProposal]=useState<Proposal|null>(null),[error,setError]=useState(''),[busy,setBusy]=useState(false);
 const [active,setActive]=useState<Proposal[]>([]);
 const [submitting,setSubmitting]=useState(false),[pollError,setPollError]=useState(''),[checkedAt,setCheckedAt]=useState<number|null>(null);
 const [loading,setLoading]=useState(!!id),[loadAttempt,setLoadAttempt]=useState(0);
 const inProgress=!!proposal&&isActiveProposal(proposal.state);
 const receive=(p:Proposal)=>{setProposal(p);setPollError('');if(!p.error)setCheckedAt(Date.now())};
 useEffect(()=>{let live=true;request<Proposal[]>('/active').then(ps=>{if(live)setActive(ps)}).catch(()=>{});return()=>{live=false}},[id]);
 const [reasons,setReasons]=useState<Record<string,string>>({});
 const mounted=useRef(true),gate=useRef(false),adoption=useRef<Record<string,string>>({});
 useEffect(()=>{mounted.current=true;return()=>{mounted.current=false}},[]);
 useEffect(()=>{if(!id)return;let live=true;setLoading(true);request<Proposal>('/'+id).then(p=>{if(!live)return;receive(p);const c=p.conditions;setIdea(c.idea);setCount(String(c.count));setMain(c.main?String(c.main):'');setSide(c.side?String(c.side):'');setCuisine(c.cuisine??'');setAvoid((c.avoid??[]).join('\n'));setUse((c.use??[]).join('\n'));setFixed(c.fixed??[]);setOmit(c.omit??[]);setSeed(c.seed)}).catch(e=>{if(live)setError(e.message)}).finally(()=>{if(live)setLoading(false)});return()=>{live=false}},[id,loadAttempt]);
 useEffect(()=>{
   if(!proposal||!inProgress||busy)return;
   let live=true,timer=0;
   const proposalID=proposal.id, delay=proposal.state==='waiting'?10000:2000;
   const poll=async()=>{
     let repeat=true;
     try { const p=await request<Proposal>('/'+proposalID); if(live){receive(p);repeat=isActiveProposal(p.state)} }
     catch { if(live)setPollError('最新の実行状態を確認できません。自動で再確認します。'); }
     finally { if(live&&repeat)timer=window.setTimeout(poll,delay); }
   };
   timer=window.setTimeout(poll,delay);
   return()=>{live=false;clearTimeout(timer)};
 },[proposal?.id,proposal?.state,inProgress,busy]);
 const lines=(s:string)=>s.split('\n').map(v=>v.trim()).filter(Boolean);
 const conditions:Conditions={idea,count:Number(count),servings:proposal?.conditions.servings,main:Number(main),side:Number(side),cuisine,avoid:lines(avoid),use:lines(use),fixed,omit,seed:seed+1};
 const valid=Number.isInteger(Number(count))&&Number(count)>0&&Number(count)<=12&&Number.isInteger(Number(main))&&Number(main)>=0&&Number.isInteger(Number(side))&&Number(side)>=0&&((!main&&!side)||Number(main)+Number(side)===Number(count));
 const perform=async(fn:()=>Promise<void>)=>{if(gate.current)return;gate.current=true;setBusy(true);setError('');try{await fn()}catch(e){if(mounted.current){setError(e instanceof Error?e.message:String(e));request<Proposal[]>('/active').then(setActive).catch(()=>{})}}finally{gate.current=false;if(mounted.current)setBusy(false)}};
 const submitTask=async(mode:string,patch:Partial<Conditions>={})=>{
   if(inProgress)throw new Error('進行中の提案が完了するか、中止してから新しく提案してください。');
   setSubmitting(true);
   try {
     const c={...conditions,...patch,servings:undefined};
     const p=await request<Proposal>('','POST',{conditions:c,mode});
     if(!mounted.current)return;
     try{localStorage.setItem('konro.last-menu-request',JSON.stringify(c))}catch{}
     receive(p);setSeed(c.seed);onOpen(p.id);
   } finally {if(mounted.current)setSubmitting(false)}
 };
 const submit=(mode:string,patch:Partial<Conditions>={})=>perform(()=>submitTask(mode,patch));
 const comparable=(c:Conditions)=>JSON.stringify([c.idea??'',c.count,c.servings,c.main,c.side,c.cuisine??'',c.avoid??[],c.use??[],c.fixed??[],c.omit??[]]);
 const conditionChanged=!!proposal&&comparable(conditions)!==comparable(proposal.conditions);
 const action=(name:string)=>perform(async()=>{if(!proposal)return;const p=await request<Proposal>('/'+proposal.id+'/'+name,'POST',{revision:proposal.revision});if(mounted.current)receive(p)});
 return <main className="editor-page proposals-page"><header className="recipe-toolbar"><button className="recipe-action recipe-action-ghost" disabled={busy} onClick={onBack}>一覧へ</button><h1 className="recipe-toolbar-title h-serif">AIで作り置きの献立を提案</h1></header>
 {error&&<p className="notice" role="alert">{error}</p>}
 {loading&&<p role="status">提案の状態を読み込んでいます…</p>}
 {!loading&&id&&!proposal&&<button className="recipe-action" onClick={()=>setLoadAttempt(n=>n+1)}>読み込みを再試行</button>}
 <ProposalProgress proposal={proposal} submitting={submitting} pollError={pollError} checkedAt={checkedAt} busy={busy} onAction={action} onRefresh={()=>void perform(async()=>{if(proposal)receive(await request<Proposal>('/'+proposal.id))})}/>

 {active.filter(p=>p.id!==id).length>0&&<Panel><h2 className="h-serif">進行中の提案</h2><p>この端末以外で始めた提案も、ここから続けられます。</p>{active.filter(p=>p.id!==id).map(p=><button key={p.id} className="recipe-action" disabled={busy} onClick={()=>onOpen(p.id)}>進行中の{p.conditions.count}品の提案を開く</button>)}</Panel>}

 <Panel><h2 className="h-serif">どんな作り置きにする？</h2><fieldset className="proposal-fields" disabled={busy||inProgress||loading}><label>イメージ<textarea aria-label="作り置きのイメージ" rows={3} maxLength={4000} value={idea} disabled={busy} onChange={e=>setIdea(e.target.value)} placeholder="例：和食寄りで野菜多め、手間は少なめ"/></label><p className="recipe-summary">品数を選ぶだけでも提案できます。主菜・副菜や食材のバランスはAIにおまかせ。</p><div className="proposal-basic"><label>品数<input aria-label="提案する品数" type="number" min="1" max="12" value={count} onChange={e=>setCount(e.target.value)}/></label></div><p className="recipe-summary">分量は各レシピの元の量を使います。必要なら買い物リストで料理ごとに調整できます。</p>
 <details><summary>詳しい条件</summary><label>必須の料理系統<select aria-label="必須の料理系統" value={cuisine} onChange={e=>setCuisine(e.target.value)}>{[['','指定しない'],['和食','和食'],['中華','中華'],['洋食','洋食'],['イタリアン、フレンチ','イタリアン、フレンチ']].map(([v,label])=><option value={v} key={v}>{label}</option>)}</select></label><div className="proposal-basic"><label>主菜<input aria-label="主菜の数" type="number" min="0" max="12" value={main} onChange={e=>setMain(e.target.value)}/></label><label>副菜<input aria-label="副菜の数" type="number" min="0" max="12" value={side} onChange={e=>setSide(e.target.value)}/></label></div><label>避ける材料（1行に1つ）<textarea aria-label="避ける材料" value={avoid} onChange={e=>setAvoid(e.target.value)}/></label><label>使いたい材料（1行に1つ）<textarea aria-label="使いたい材料" value={use} onChange={e=>setUse(e.target.value)}/></label></details>
 {fixed.length>0&&<p>固定する料理：{fixed.map(file=>proposal?.candidates.find(c=>c.recipe.file===file)?.recipe.title??file).join('、')} <button className="chip" onClick={()=>setFixed([])}>固定を解除</button></p>}
 <div className="editor-actions"><button className="recipe-action recipe-action-primary" disabled={busy||!valid} onClick={()=>submit('ai')}>{submitting?'受付中…':inProgress?'提案を確認中…':'AIで提案する'}</button><button className="recipe-action" disabled={busy||!valid} onClick={()=>submit('rules')}>条件だけで簡易提案</button></div>{!valid&&<p>1〜12の整数で品数を入力してください。内訳を指定する場合は主菜＋副菜を品数と一致させます。</p>}</fieldset></Panel>
 {conditionChanged&&<p role="status">条件を変更しました。再提案してから採用してください。</p>}
 {proposal&&<><Panel><h2 className="h-serif">条件と提案の確認</h2><p>{proposal.conditions.count}品{proposal.conditions.servings?` · 各${proposal.conditions.servings}人分（保存時の条件）`:' · 各レシピの元の分量'}{proposal.conditions.main+proposal.conditions.side>0&&` · 主菜${proposal.conditions.main}＋副菜${proposal.conditions.side}`}</p>{proposal.interpretation&&<p>{proposal.interpretation}</p>}<p className="recipe-summary">{proposal.conditions.servings?'保存時の人数条件で選んだ候補です。新しく提案すると各レシピの元の分量を使います。':'材料・手順が登録された料理から選びます。人数未登録のレシピも含めます。'}</p></Panel>
 {proposal.menus?.map((menu,index)=><Panel key={menu.id}><h2 className="h-serif">案 {index+1}</h2><ul className="proposal-picks">{menu.picks.map(pick=>{const candidate=proposal.candidates.find(c=>c.recipe.file===pick.file);return <li key={pick.file}><strong>{candidate?.recipe.title??pick.file}</strong><p className="recipe-summary">{pick.reason}</p><label>入れ替える理由（任意）<select aria-label={`${candidate?.recipe.title??pick.file}を替える理由`} value={reasons[pick.file]??''} onChange={e=>setReasons(prev=>({...prev,[pick.file]:e.target.value}))}><option value="">指定しない</option>{['今日は気分でない','材料がない','手間が多い','量が多い','好みでない'].map(reason=><option key={reason}>{reason}</option>)}</select></label><div className="editor-actions"><button className={`chip ${fixed.includes(pick.file)?'chip-on':''}`} aria-pressed={fixed.includes(pick.file)} disabled={busy} onClick={()=>void perform(async()=>{await feedbackRequest('meal-feedback','POST',{id:crypto.randomUUID(),revision:0,proposalId:proposal.id,file:pick.file,version:candidate?.recipe.version,type:'fixed',reason:fixed.includes(pick.file)?'固定を解除':'固定する'});setFixed(prev=>prev.includes(pick.file)?prev.filter(f=>f!==pick.file):[...prev,pick.file])})}>{fixed.includes(pick.file)?'固定済み・解除':'この料理を固定'}</button><button className="chip" disabled={busy||fixed.includes(pick.file)} onClick={()=>void perform(async()=>{await feedbackRequest('meal-feedback','POST',{id:crypto.randomUUID(),revision:0,proposalId:proposal.id,file:pick.file,version:candidate?.recipe.version,type:'swap',reason:reasons[pick.file]??''});const kept=menu.picks.filter(p=>p.file!==pick.file).map(p=>p.file);setFixed(kept);setOmit([pick.file]);await submitTask(proposal.mode,{fixed:kept,omit:[pick.file]})})}>この1品を別案へ</button></div></li>})}</ul><button className="recipe-action recipe-action-primary" disabled={busy||conditionChanged} onClick={()=>perform(async()=>{const op=adoption.current[menu.id]??crypto.randomUUID();adoption.current[menu.id]=op;const p=await request<MealPlan>('/'+proposal.id+'/adopt','POST',{revision:proposal.revision,menuId:menu.id},op);if(mounted.current)onAdopt(p.id)})}>この案で買い物リストを作る</button></Panel>)}</>}
 <MealPreferences/><CookHistory/>
 </main>;
}
