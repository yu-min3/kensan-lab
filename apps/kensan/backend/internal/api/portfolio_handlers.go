package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yu-min3/kensan-lab/apps/kensan/backend/internal/portfolio"
)

// GET /api/v1/portfolio — ダッシュボード（地図・羅針盤・年報・提案）。開くたびにファイルから組み立てる。
func (s *Server) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	v, err := portfolio.Load(s.ws.Root, time.Now())
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (s *Server) handleProposalAccept(w http.ResponseWriter, r *http.Request) {
	s.decideProposal(w, r, true)
}
func (s *Server) handleProposalReject(w http.ResponseWriter, r *http.Request) {
	s.decideProposal(w, r, false)
}

// POST /api/v1/portfolio/proposals/{accept,reject} {file, line, text}
func (s *Server) decideProposal(w http.ResponseWriter, r *http.Request, accept bool) {
	var req struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" || req.Line < 1 || req.Text == "" {
		writeError(w, http.StatusBadRequest, "file, line, text are required")
		return
	}
	p, err := portfolio.Decide(s.ws, req.File, req.Line, req.Text, accept, time.Now())
	if errors.Is(err, portfolio.ErrMismatch) {
		writeError(w, http.StatusConflict, "提案が変わったか、すでに判定済みです。再読込してください")
		return
	}
	if err != nil {
		writeOpError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"proposal": p})
}
