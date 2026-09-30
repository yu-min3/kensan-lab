package core

import (
	"errors"
	"time"
)

// LeaseTTL は 1 回の attempt が provider を押さえていられる既定の時間。
// worker が落ちたのではなく固まった場合、controller からは「running のまま
// 何も起きない」に見える。provider ごと最大 1 実行なので、これを放置すると
// その provider の配車が永久に止まる。
//
// 走り切る前に切れないよう、1 ターンの最長より十分長く取る。生きている worker は
// RenewLease で伸ばす。
const LeaseTTL = 45 * time.Minute

// MaxAttempts は 1 agent が同じ工程を試せる回数。超えたら配車しない。
// 無限 retry を避けるための上限であり、成功回数ではない。
const MaxAttempts = 3

// RenewLease は生きている worker が lease を伸ばす。running でなければ拒否する。
func (s *Store) RenewLease(attemptID string, now time.Time) error {
	return s.update(func(st *State) error {
		a, ok := st.Attempts[attemptID]
		if !ok {
			return errors.New("unknown attempt")
		}
		if a.Status != "running" {
			return errors.New("attempt is not running")
		}
		exp := now.UTC().Add(LeaseTTL)
		a.LeaseExpiresAt = &exp
		st.Attempts[attemptID] = a
		return nil
	})
}

// ExpireLeases は lease の切れた running attempt を interrupted にして、
// 押さえていた provider を解放する。返るのは解放した件数。
//
// 再実行はしない。controller 再起動時の RecoverInterrupted と同じ扱いで、
// worker の出力を人が見るまで止める。lease 切れは「worker が死んだ」とは
// 限らず、生きたまま固まっている可能性があるため、自動で再開すると同じ
// 外部操作を二重に走らせかねない。
func (s *Store) ExpireLeases(now time.Time) (int, error) {
	expired := 0
	err := s.update(func(st *State) error {
		expired = 0
		for id, a := range st.Attempts {
			if a.Status != "running" || a.LeaseExpiresAt == nil || !now.After(*a.LeaseExpiresAt) {
				continue
			}
			t := now.UTC()
			a.Status, a.Reason, a.FinishedAt = "interrupted", "lease expired; inspect worker output before retry", &t
			st.Attempts[id] = a
			agent := st.Agents[a.AgentID]
			agent.Status, agent.UpdatedAt = "interrupted", t
			st.Agents[agent.ID] = agent
			st.Events = append(st.Events, event("attempt_lease_expired", id, agent.ID))
			expired++
		}
		return nil
	})
	return expired, err
}
