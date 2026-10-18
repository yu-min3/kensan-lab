import { Star } from 'lucide-react';
import type { SourceInfo } from '../api';
export function SourceCredit({info,showPublisher=true}:{info?:SourceInfo|null;showPublisher?:boolean}){
 if(!info)return null;
 const {creator,publisher,rating}=info;
 const chef=creator?.id?.startsWith('kurashiru:chef:')||creator?.id?.startsWith('nhk:');
 return <div className="source-credit">
  {creator&&<p className="source-creator"><span>{chef?'料理人':'投稿者'}：</span>{creator.name}</p>}
  {showPublisher&&publisher&&publisher.name!==creator?.name&&<p className="recipe-summary">配信：{publisher.name}</p>}
  {rating&&<p className="source-rating" aria-label={`出典の評価 ${rating.value}、${rating.best}点満点${rating.count?`、${rating.count}件`:''}`}><Star aria-hidden="true"/><span className="tnum">{rating.value} / {rating.best}</span>{!!rating.count&&<span>（{rating.count.toLocaleString('ja-JP')}件）</span>}</p>}
 </div>;
}
