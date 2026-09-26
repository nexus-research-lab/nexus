package channels

import (
	"context"
	"fmt"
	"time"

	"github.com/nexus-research-lab/nexus/internal/infra/duework"
)

const ingressRecoveryBatchSize = 100

// RunRecovery 随宿主启动和关闭；补齐已有输入的回执，不重新执行未知副作用。
func (s *IngressService) RunRecovery(ctx context.Context) error {
	if s.control == nil || s.readRoundIndex == nil {
		return nil
	}
	cursor := ingressMessageRow{}
	return s.recovery.Run(ctx, func(ctx context.Context, _ time.Time) (duework.Result, error) {
		result, err := s.recoverIngressBatch(ctx, &cursor)
		if err != nil {
			s.loggerFor(ctx).Warn("读取入站恢复账本失败", "err", err)
		}
		return result, err
	})
}

func (s *IngressService) recoverIngressBatch(ctx context.Context, cursor *ingressMessageRow) (duework.Result, error) {
	rows, err := s.control.pendingIngressMessages(ctx, *cursor)
	if err != nil {
		return duework.Result{}, err
	}
	for _, row := range rows {
		if err := ctx.Err(); err != nil {
			return duework.Result{}, err
		}
		request := normalizedIngressRequest{ownerUserID: row.OwnerUserID, channelStored: row.Channel,
			accountID: row.AccountID, reqID: row.ReqID, agentID: row.AgentID, sessionKey: row.SessionKey, roundID: row.RoundID}
		recovered, err := s.recoverIngress(ctx, request)
		if err != nil {
			s.loggerFor(ctx).Warn("核验入站持久证据失败", "req_id", row.ReqID, "err", err)
		}
		if recovered && err == nil {
			s.notifyExternalSessionUpdated(contextWithIngressOwner(ctx, row.OwnerUserID), request)
		}
		// 缺失或损坏的早期证据不能挡住后续消息，下一轮低频审计再核验。
		*cursor = row
	}
	if len(rows) == ingressRecoveryBatchSize {
		return duework.Result{HasMore: true}, nil
	}
	*cursor = ingressMessageRow{}
	return duework.Result{}, nil
}

func (s *ControlService) pendingIngressMessages(ctx context.Context, cursor ingressMessageRow) ([]ingressMessageRow, error) {
	query := fmt.Sprintf(`SELECT owner_user_id, channel_type, account_id, req_id, agent_id, session_key, round_id
 FROM im_ingress_messages WHERE status='processing' AND dispatch_phase='dispatching'
 AND (owner_user_id, channel_type, account_id, req_id) > (%s)
 ORDER BY owner_user_id, channel_type, account_id, req_id LIMIT 100`, s.bindList(4))
	rows, err := s.db.QueryContext(ctx, query, cursor.OwnerUserID, cursor.Channel, cursor.AccountID, cursor.ReqID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]ingressMessageRow, 0, ingressRecoveryBatchSize)
	for rows.Next() {
		var item ingressMessageRow
		if err := rows.Scan(&item.OwnerUserID, &item.Channel, &item.AccountID, &item.ReqID, &item.AgentID, &item.SessionKey, &item.RoundID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
