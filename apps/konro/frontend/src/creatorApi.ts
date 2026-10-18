export type FavoriteCreator={id:string;name:string;url:string;additional:boolean};
export type CreatorSettings={revision:string;creators:FavoriteCreator[]};
export type CreatorMutation=CreatorSettings&{undo?:string};
export class CreatorError extends Error{constructor(message:string,public status:number){super(message)}}
export async function creatorRequest<T>(path='',method='GET',body?:unknown):Promise<T>{
 const response=await fetch('/api/v1/creators'+path,{method,headers:{'Content-Type':'application/json'},body:body===undefined?undefined:JSON.stringify(body)});
 if(!response.ok)throw new CreatorError((await response.text()).trim()||'作り手の設定を変更できませんでした',response.status);
 return response.json();
}
export function canFollow(id?:string){return !!id&&(/^(youtube:UC[A-Za-z0-9_-]{22}|kurashiru:chef:[1-9][0-9]{0,9})$/).test(id)}
export function creatorSite(id:string){return id.startsWith('youtube:')?'YouTube':'クラシルの料理人'}
export function creatorURL(id:string){if(!canFollow(id))return '';return id.startsWith('youtube:')?'https://www.youtube.com/channel/'+id.slice('youtube:'.length):'https://www.kurashiru.com/chefs/'+id.slice('kurashiru:chef:'.length)}
