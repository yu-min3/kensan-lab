import type {useFavoriteCreators} from '../hooks/useFavoriteCreators';
export function CreatorNotice({favorites:f}:{favorites:ReturnType<typeof useFavoriteCreators>}){
 return <>{f.busy&&<p role="status">作り手の設定を保存しています…</p>}{f.error&&<div role="alert" className="notice"><p>{f.error}</p><button className="chip" disabled={f.busy||f.loading} onClick={()=>void f.refresh()}>一覧を読み直す</button></div>}{f.notice&&<div role="status" className="creator-notice"><span>{f.notice}</span>{f.undo&&<button className="chip" disabled={f.busy} onClick={()=>void f.undoLast()}>取り消す</button>}</div>}</>;
}
