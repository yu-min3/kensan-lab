package web

import (
	"github.com/yu-min3/kensan-lab/apps/sense-dev/internal/core"
	"sort"
)

type taskView struct {
	core.Task
	Phase      string
	SourceTeam core.Team
}

type feedbackView struct {
	core.FeedbackLoop
	Phase, Summary, Reason    string
	SourceTaskID, SourceTitle string
}

func humanCategoryLabel(category string) string {
	switch category {
	case "auth":
		return "認証・認可"
	case "secret":
		return "secret・権限"
	case "publication":
		return "公開"
	case "control":
		return "統制そのもの"
	case "data_destruction":
		return "データ破壊"
	default:
		return "要確認: " + category
	}
}

func feedbackPhase(status string) string {
	switch status {
	case "pending_decision":
		return "Platform 判定待ち"
	case "adopted", "improving":
		return "採用・Platform 改善中"
	case "awaiting_deployment":
		return "Platform 改善の配備待ち"
	case "retest_ready":
		return "改善配備済み・App 再確認待ち"
	case "retesting":
		return "App が同じ操作を再確認中"
	case "completed", "resolved":
		return "App 再確認完了"
	case "rejected":
		return "却下"
	case "deferred":
		return "保留"
	case "needs_human", "decision_wait":
		return "Yu の判断待ち"
	default:
		return "状況の確認が必要: " + status
	}
}

func developmentTask(st core.State, task core.Task) taskView {
	view := taskView{Task: task, SourceTeam: st.Tasks[task.SourceTaskID].Team, Phase: task.Status}
	if task.Kind == "change" {
		switch task.Status {
		case "publish_wait":
			receipt := st.Deployments[task.ID]
			if receipt.HeadSHA == task.HeadSHA && receipt.Status == "healthy" {
				view.Phase = "配備済み・App 動作確認待ち"
				for _, acceptance := range st.Tasks {
					if acceptance.Kind == "acceptance" && acceptance.SourceTaskID == task.ID && acceptance.HeadSHA == task.HeadSHA && acceptance.Status == "done" {
						view.Phase = "App 動作確認完了"
						break
					}
				}
			} else {
				view.Phase = "Release Gate・配備待ち"
			}
		case "decision_wait":
			view.Phase = "Yu の判断待ち"
		default:
			roleLabels := map[string]string{"requirements": "要件整理", "design_review": "設計レビュー", "implementation": "実装", "verification": "独立検証", "implementation_review": "実装レビュー"}
			stages := make([]core.Agent, 0)
			for _, a := range st.Agents {
				if a.TaskID == task.ID {
					stages = append(stages, a)
				}
			}
			sort.Slice(stages, func(i, j int) bool { return agentOrder(stages[i].Role) < agentOrder(stages[j].Role) })
			for _, a := range stages {
				if a.Status != "completed" {
					if label, ok := roleLabels[a.Role]; ok {
						view.Phase = label + " · " + a.Status
					}
					break
				}
			}
		}
	} else if task.Kind == "acceptance" {
		switch task.Status {
		case "done":
			view.Phase = "App 動作確認完了"
		case "revision_wait":
			view.Phase = teamLabel(view.SourceTeam) + " 修正・再配備待ち"
		case "decision_wait":
			view.Phase = "受入証拠の確認待ち"
		default:
			if task.HeadSHA == "" {
				view.Phase = teamLabel(view.SourceTeam) + " の変更・配備待ち"
			} else {
				view.Phase = "App が利用者経路を確認中"
			}
		}
	}
	if task.FeedbackMessageID != "" {
		if loop, ok := st.FeedbackLoops[task.FeedbackMessageID]; ok {
			view.Phase = feedbackPhase(loop.Status)
		}
	}
	return view
}

func (s *Server) feedbackViews(st core.State, taskID string) []feedbackView {
	views := make([]feedbackView, 0, len(st.FeedbackLoops))
	for _, loop := range st.FeedbackLoops {
		message := st.Messages[loop.FeedbackID]
		if taskID != "" && message.SourceTask != taskID && loop.AnalysisTaskID != taskID && loop.ImprovementTaskID != taskID && loop.RetestTaskID != taskID {
			continue
		}
		view := feedbackView{FeedbackLoop: loop, Phase: feedbackPhase(loop.Status), SourceTaskID: message.SourceTask, SourceTitle: st.Tasks[message.SourceTask].Title}
		feedback, err := s.store.ReadPlatformFeedback(loop.FeedbackID)
		if err != nil {
			view.Summary = "feedback 証拠を確認できません"
		} else {
			view.Summary = feedback.Summary
		}
		if loop.DecisionID != "" {
			decision, err := s.store.ReadPlatformDecision(loop.DecisionID)
			if err != nil {
				view.Reason = "判定証拠を確認できません"
			} else {
				view.Reason = decision.Reason
			}
		}
		views = append(views, view)
	}
	sort.Slice(views, func(i, j int) bool { return views[i].FeedbackID < views[j].FeedbackID })
	return views
}
