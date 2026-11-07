import {useEffect, useState} from 'react';
import {CheckCircle2, CircleAlert, Clock, LoaderCircle, PauseCircle, XCircle} from 'lucide-react';
import type {Proposal} from '../proposals';
import {isActiveProposal} from '../proposals';
import {Panel} from './Editor';

const reasons: Record<string, string> = {
  billing_blocked: 'AIの認証・利用設定を確認できず停止しています。設定の確認後に再試行してください。',
  quota_wait: 'AIの利用枠が空くのを待っています。利用できるようになったら再試行してください。',
  auth_required: 'AIへの再ログインが必要です。ログイン後に再試行してください。',
  authentication_required: 'AIへの再ログインが必要です。ログイン後に再試行してください。',
  invalid_output: '提案の形式を確認できませんでした。再試行できます。',
  invalid_conditions: '必須条件に合わない案だったため、採用していません。再試行できます。',
};
const timeLabel = (value?: string) => value && Number.isFinite(Date.parse(value))
  ? new Date(value).toLocaleTimeString('ja-JP', {hour:'2-digit', minute:'2-digit', second:'2-digit'}) : '';

export function ProposalProgress({proposal, submitting, pollError, checkedAt, busy, onRefresh, onAction}: {
  proposal: Proposal | null; submitting: boolean; pollError: string; checkedAt: number | null;
  busy: boolean; onRefresh: () => void; onAction: (action: string) => void;
}) {
  const [now, setNow] = useState(Date.now);
  const active = !!proposal && isActiveProposal(proposal.state);
  useEffect(() => {
    if (!active) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [active]);
  if (!proposal && !submitting) return null;
  const state = submitting ? 'submitting' : proposal!.state;
  const disconnected = !!pollError || (!!proposal?.error && active);
  const waiting = state === 'waiting';
  const moving = !disconnected && ['submitting', 'queued', 'running'].includes(state);
  const succeeded = state === 'succeeded';
  const title = disconnected ? '接続を確認しています' : ({
    submitting:'提案を受け付けています', queued:'AIの順番を待っています', running:'AIが献立を考えています',
    waiting:'提案は一時停止しています', succeeded:'献立の提案ができました',
    failed:'提案を完了できませんでした', cancelled:'提案を中止しました', unavailable:'AIを利用できません',
  } as Record<string,string>)[state] ?? '提案の状態を確認してください';
  const Icon = disconnected ? CircleAlert : succeeded ? CheckCircle2 : waiting ? PauseCircle
    : state === 'cancelled' ? XCircle : moving ? (state === 'queued' ? Clock : LoaderCircle) : CircleAlert;
  const reason = proposal?.job?.reason;
  const detail = disconnected ? '最新の実行状態を確認できません。自動で再確認します。'
    : reason && reasons[reason] ? reasons[reason]
    : proposal?.error || (waiting ? 'AI側で待機しています。状態を再確認するか、時間をおいて再試行してください。'
    : state === 'running' ? '保存したレシピから、好みに合う組み合わせを選んでいます。'
    : state === 'queued' ? '受付済みです。実行が始まると表示が切り替わります。'
    : state === 'submitting' ? '条件を送信しています。このままお待ちください。'
    : succeeded ? `${proposal?.menus?.length ?? 0}案を確認して、気に入った献立を選べます。` : '');
  const start = Date.parse(proposal?.job?.created_at || proposal?.createdAt || '');
  const end = active ? now : Date.parse(proposal?.job?.updated_at || '');
  const seconds = Number.isFinite(start) && Number.isFinite(end) ? Math.max(0, Math.floor((end-start)/1000)) : null;
  const duration = seconds === null ? '' : seconds < 60 ? `${seconds}秒` : `${Math.floor(seconds/60)}分${seconds%60}秒`;
  return <Panel className="proposal-progress" aria-label="提案の進捗">
    <div className="proposal-progress-heading" role="status"><Icon aria-hidden="true"/><h2>{title}</h2></div>
    <p>{detail}</p>
    {moving || succeeded
      ? <progress aria-label={succeeded ? '提案完了' : '提案処理中：進捗率は未取得'} max={1} value={succeeded ? 1 : undefined}/>
      : <div className="proposal-progress-paused" aria-label="進捗バー停止中" role="img"/>}
    {moving && <p className="recipe-summary">進捗率・残り時間はまだ分かりません。状態を自動更新しています。</p>}
    <div className="proposal-progress-times">
      {duration && <span>{active ? '受付から' : '受付から最終更新まで'} {duration}</span>}
      {proposal?.job?.updated_at && <span>状態更新 {timeLabel(proposal.job.updated_at)}</span>}
      {checkedAt && <span>最終確認 {new Date(checkedAt).toLocaleTimeString('ja-JP')}</span>}
    </div>
    {reason && !reasons[reason] && <p className="recipe-summary">理由コード: {reason}</p>}
    {proposal && !submitting && <div className="editor-actions">
      {active && <button className="recipe-action" disabled={busy} onClick={()=>onAction('cancel')}>提案を中止</button>}
      {['waiting','failed'].includes(state) && <button className="recipe-action" disabled={busy} onClick={()=>onAction('retry')}>提案を再試行</button>}
      <button className="recipe-action" disabled={busy} onClick={onRefresh}>状態を再確認</button>
    </div>}
  </Panel>;
}
