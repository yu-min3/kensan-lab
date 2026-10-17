import type { HTMLAttributes } from "react";

export function Panel(props: HTMLAttributes<HTMLElement>) {
 return <section {...props} className={`editor-panel ${props.className ?? ""}`} />;
}
export function Field({id,label,value,onChange,rows,type="text",maxLength}: {id:string;label:string;value:string;onChange:(value:string)=>void;rows?:number;type?:string;maxLength?:number}) {
 return <label htmlFor={id}><span id={`${id}-label`}>{label}</span>
 {rows ? <textarea id={id} aria-labelledby={`${id}-label`} rows={rows} value={value} onChange={(e)=>onChange(e.target.value)} /> : <input id={id} type={type} min={type==="number"?0:undefined} max={type==="number"?100000:undefined} step={type==="number"?"any":undefined} maxLength={maxLength} value={value} onChange={(e)=>onChange(e.target.value)} />}
 </label>;
}
