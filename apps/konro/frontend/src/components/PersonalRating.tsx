import {Star} from 'lucide-react';
export function PersonalRating({rating=0}:{rating?:number}){
 if(!rating)return <span className="personal-rating-empty">未評価</span>;
 return <span className="personal-rating" aria-label={`自分の評価 ${rating}、5点満点`}><span className="personal-rating-stars" aria-hidden="true">{[1,2,3,4,5].map(n=><Star key={n} fill={n<=rating?'currentColor':'none'} className={n<=rating?'rated-star':'unrated-star'}/>)}</span><strong className="tnum">{rating} / 5</strong></span>;
}
