import { unified } from "unified";
import remarkParse from "remark-parse";
import { dailySection } from "./dailyIntentions";

export function normalizeQuickLine(text: string) {
  const line = text.replace(/\s+/g, " ").trim();
  if (!line || Array.from(line).length > 500) throw new Error("一行日記は1〜500文字で書いてください。");
  return line;
}
function diarySpan(content: string) {
  const span = dailySection(content, "日記");
  if (!span) return null;
  const heading = unified().use(remarkParse).parse(content.slice(span.start,span.end)).children.find(n=>n.type==="heading");
  return {start:span.start,end:heading?.position?.start?.offset!=null?span.start+heading.position.start.offset:span.end};
}
function entryPositions(content: string) {
  const span=diarySpan(content);if(!span)return [];
  const body=content.slice(span.start,span.end);
  return unified().use(remarkParse).parse(body).children.flatMap(node=>node.type==="list"&&!node.ordered?node.children.map(item=>{
    const start=span.start+item.position!.start.offset!;
    return {start,entry:content.slice(start,span.start+item.position!.end.offset!).split(/\r?\n/)[0]};
  }):[]);
}
export function quickLineWasSaved(before: string, after: string, entry: string, oldEntry?: string) {
  const count=(text:string,value:string)=>entryPositions(text).filter(p=>p.entry===value).length;
  if(oldEntry===entry)return count(after,entry)>=count(before,entry)&&count(after,entry)>0;
  return count(after,entry)>count(before,entry)&&(!oldEntry||count(after,oldEntry)<count(before,oldEntry));
}
export function prepareQuickLine(content: string, value: string, at: string, editingEntry?: string) {
  const text = normalizeQuickLine(value);
  if(!/^(?:[01]\d|2[0-3]):[0-5]\d$/.test(at))throw new Error("記録時刻を確認してください。");
  const entry = `- ${at} ${text}`;
  const span = diarySpan(content);
  let prefix: string, suffix: string;
  if (editingEntry) {
    if (!span) throw new Error("編集する一行を確認できません。日記を開いて確認してください。");
    const positions=entryPositions(content).filter(p=>p.entry===editingEntry);
    if(positions.length!==1)throw new Error("編集する一行を確認できません。日記を開いて確認してください。");
    const start=positions[0].start;
    prefix=content.slice(0,start);suffix=content.slice(start+editingEntry.length);
  } else if (span) {
    const body=content.slice(span.start,span.end);
    const end=span.start+body.trimEnd().length;
    prefix=content.slice(0,end)+"\n\n";suffix=content.slice(end)||"\n";
  } else {
    prefix=content+(content.endsWith("\n\n")?"":content.endsWith("\n")?"\n":"\n\n")+"## 日記\n\n";suffix="\n";
  }
  const updated=prefix+entry+suffix;
  if(!entryPositions(updated).some(p=>p.entry===entry))throw new Error("日記の見出しを確認できません。日記を開いて確認してください。");
  return {content:updated,entry,text,at};
}
export function shiftDailyDate(date: string, delta: number) {
  const next=new Date(`${date}T12:00:00Z`);next.setUTCDate(next.getUTCDate()+delta);return next.toISOString().slice(0,10);
}
