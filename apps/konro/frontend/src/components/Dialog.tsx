import {ReactNode, useEffect, useId, useRef} from 'react';

export function Dialog({title,onDismiss,children}:{title:string;onDismiss:()=>void;children:ReactNode}) {
  const ref=useRef<HTMLDialogElement>(null), id=useId();
  useEffect(()=>{const node=ref.current!;node.showModal();return()=>node.close();},[]);
  return <dialog ref={ref} className="editor-panel cooking-dialog" aria-labelledby={id} onCancel={e=>{e.preventDefault();onDismiss()}}>
    <h2 id={id} className="h-serif">{title}</h2>{children}
  </dialog>;
}
