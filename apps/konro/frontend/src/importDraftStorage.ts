import type {ImportDraft} from './api';
export const storageKey='konro.import-draft';
const indexKey='konro.import-drafts';
export type DraftRef={id:string;url:string;title:string};
const validID=(id:unknown):id is string=>typeof id==='string'&&/^[0-9a-f]{32}$/.test(id);
export function readDraftRefs():DraftRef[]{
 let refs:DraftRef[]=[];
 try {const value=JSON.parse(localStorage.getItem(indexKey)??'[]');if(Array.isArray(value))refs=value.filter(r=>r&&validID(r.id)&&typeof r.url==='string'&&typeof r.title==='string');}catch{/* legacy pointer remains usable */}
 const last=localStorage.getItem(storageKey);
 if(validID(last)&&!refs.some(r=>r.id===last))refs.unshift({id:last,url:'',title:'以前の下書き'});
 return refs;
}
export function rememberDraft(d:ImportDraft):DraftRef[]{
 const refs=[{id:d.id,url:d.source.url??'',title:d.content.title||'無題の下書き'},...readDraftRefs().filter(r=>r.id!==d.id)];
 localStorage.setItem(indexKey,JSON.stringify(refs));localStorage.setItem(storageKey,d.id);return refs;
}
export function clearImportDraft(id?:string){
 const current=localStorage.getItem(storageKey);
 if(id){localStorage.setItem(indexKey,JSON.stringify(readDraftRefs().filter(r=>r.id!==id)));}
 if(!id||current===id)localStorage.removeItem(storageKey);
}
