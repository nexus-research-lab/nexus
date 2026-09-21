// INPUT: Pairing/Channel 变更事务与旧的 automation delivery route。
// OUTPUT: 在授权世代变化前删除 session-scoped 与 legacy last route 能力。
// POS: IM pairing 生命周期与投递路由之间的事务清理边界。
package channels

import (
	"context"
	"database/sql"
	"strings"
)

// deletePairingDeliveryRoutesTx removes both session-scoped and legacy
// agent-scoped remembered routes for one pairing. Remembered routes are a
// delivery capability, so leaving an empty-session "last" row behind after a
// pairing rebind would let a later automation run rediscover the old target.
// The caller must invoke this before deleting or changing the pairing row.
func (s *ControlService) deletePairingDeliveryRoutesTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	pairingID string,
) error {
	ownerUserID = strings.TrimSpace(ownerUserID)
	pairingID = strings.TrimSpace(pairingID)
	keyQuery := `
DELETE FROM automation_delivery_routes
WHERE agent_id = (
    SELECT agent_id
    FROM im_pairings
    WHERE owner_user_id = ` + s.bind(1) + ` AND pairing_id = ` + s.bind(2) + `
)
  AND (
      session_key IN (
          SELECT session_key
          FROM im_pairings
          WHERE owner_user_id = ` + s.bind(3) + ` AND pairing_id = ` + s.bind(4) + `
      )
      OR session_key IN (
          SELECT session_key
          FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(5) + ` AND pairing_id = ` + s.bind(6) + `
      )
      OR target_session_key IN (
          SELECT session_key
          FROM im_pairings
          WHERE owner_user_id = ` + s.bind(7) + ` AND pairing_id = ` + s.bind(8) + `
      )
      OR target_session_key IN (
          SELECT session_key
          FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(9) + ` AND pairing_id = ` + s.bind(10) + `
      )
  )`
	if _, err := tx.ExecContext(ctx, keyQuery,
		ownerUserID, pairingID, ownerUserID, pairingID, ownerUserID, pairingID,
		ownerUserID, pairingID, ownerUserID, pairingID,
	); err != nil {
		return err
	}

	targetQuery := `
DELETE FROM automation_delivery_routes
WHERE COALESCE(session_key, '') = ''
  AND (
      EXISTS (
          SELECT 1
          FROM im_pairings p
          WHERE p.owner_user_id = ` + s.bind(1) + `
            AND p.pairing_id = ` + s.bind(2) + `
            AND p.agent_id = automation_delivery_routes.agent_id
            AND COALESCE(automation_delivery_routes.channel, '') = p.channel_type
            AND COALESCE(automation_delivery_routes."to", '') = p.external_ref
            AND COALESCE(automation_delivery_routes.account_id, '') = p.account_id
            AND COALESCE(automation_delivery_routes.thread_id, '') = p.thread_id
      )
      OR EXISTS (
          SELECT 1
          FROM im_pairing_sessions ps
          JOIN im_pairings p ON p.owner_user_id = ps.owner_user_id AND p.pairing_id = ps.pairing_id
          WHERE ps.owner_user_id = ` + s.bind(3) + `
            AND ps.pairing_id = ` + s.bind(4) + `
            AND p.agent_id = automation_delivery_routes.agent_id
            AND COALESCE(automation_delivery_routes.channel, '') = ps.channel_type
            AND COALESCE(automation_delivery_routes."to", '') = ps.external_ref
            AND COALESCE(automation_delivery_routes.account_id, '') = ps.account_id
            AND COALESCE(automation_delivery_routes.thread_id, '') = ps.thread_id
      )
  )`
	if _, err := tx.ExecContext(ctx, targetQuery, ownerUserID, pairingID, ownerUserID, pairingID); err != nil {
		return err
	}
	return nil
}

// deleteChannelDeliveryRoutesTx applies the same capability cleanup when an
// entire external Channel is removed. It deliberately matches the concrete
// target columns for legacy agent-scoped routes, so another IM destination of
// the same Agent survives.
func (s *ControlService) deleteChannelDeliveryRoutesTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	channelType string,
) error {
	query := `
DELETE FROM automation_delivery_routes
WHERE (
      session_key IN (
          SELECT session_key FROM im_pairings
          WHERE owner_user_id = ` + s.bind(1) + ` AND channel_type = ` + s.bind(2) + `
      )
      OR session_key IN (
          SELECT session_key FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(3) + ` AND channel_type = ` + s.bind(4) + `
      )
      OR target_session_key IN (
          SELECT session_key FROM im_pairings
          WHERE owner_user_id = ` + s.bind(5) + ` AND channel_type = ` + s.bind(6) + `
      )
      OR target_session_key IN (
          SELECT session_key FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(7) + ` AND channel_type = ` + s.bind(8) + `
      )
      OR (
          COALESCE(session_key, '') = ''
          AND (
              EXISTS (
                  SELECT 1 FROM im_pairings p
                  WHERE p.owner_user_id = ` + s.bind(9) + ` AND p.channel_type = ` + s.bind(10) + `
                    AND p.agent_id = automation_delivery_routes.agent_id
                    AND COALESCE(automation_delivery_routes.channel, '') = p.channel_type
                    AND COALESCE(automation_delivery_routes."to", '') = p.external_ref
                    AND COALESCE(automation_delivery_routes.account_id, '') = p.account_id
                    AND COALESCE(automation_delivery_routes.thread_id, '') = p.thread_id
              )
              OR EXISTS (
                  SELECT 1
                  FROM im_pairing_sessions ps
                  JOIN im_pairings p ON p.owner_user_id = ps.owner_user_id AND p.pairing_id = ps.pairing_id
                  WHERE ps.owner_user_id = ` + s.bind(11) + ` AND ps.channel_type = ` + s.bind(12) + `
                    AND p.agent_id = automation_delivery_routes.agent_id
                    AND COALESCE(automation_delivery_routes.channel, '') = ps.channel_type
                    AND COALESCE(automation_delivery_routes."to", '') = ps.external_ref
                    AND COALESCE(automation_delivery_routes.account_id, '') = ps.account_id
                    AND COALESCE(automation_delivery_routes.thread_id, '') = ps.thread_id
              )
          )
      )
  )`
	_, err := tx.ExecContext(ctx, query,
		ownerUserID, channelType, ownerUserID, channelType,
		ownerUserID, channelType, ownerUserID, channelType,
		ownerUserID, channelType, ownerUserID, channelType,
	)
	return err
}

func (s *ControlService) deleteChannelAccountDeliveryRoutesTx(
	ctx context.Context,
	tx *sql.Tx,
	ownerUserID string,
	channelType string,
	accountID string,
) error {
	query := `
DELETE FROM automation_delivery_routes
WHERE (
      session_key IN (
          SELECT session_key FROM im_pairings
          WHERE owner_user_id = ` + s.bind(1) + ` AND channel_type = ` + s.bind(2) + ` AND account_id = ` + s.bind(3) + `
      )
      OR session_key IN (
          SELECT session_key FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(4) + ` AND channel_type = ` + s.bind(5) + ` AND account_id = ` + s.bind(6) + `
      )
      OR target_session_key IN (
          SELECT session_key FROM im_pairings
          WHERE owner_user_id = ` + s.bind(7) + ` AND channel_type = ` + s.bind(8) + ` AND account_id = ` + s.bind(9) + `
      )
      OR target_session_key IN (
          SELECT session_key FROM im_pairing_sessions
          WHERE owner_user_id = ` + s.bind(10) + ` AND channel_type = ` + s.bind(11) + ` AND account_id = ` + s.bind(12) + `
      )
      OR (
          COALESCE(session_key, '') = ''
          AND (
              EXISTS (
                  SELECT 1 FROM im_pairings p
                  WHERE p.owner_user_id = ` + s.bind(13) + ` AND p.channel_type = ` + s.bind(14) + ` AND p.account_id = ` + s.bind(15) + `
                    AND p.agent_id = automation_delivery_routes.agent_id
                    AND COALESCE(automation_delivery_routes.channel, '') = p.channel_type
                    AND COALESCE(automation_delivery_routes."to", '') = p.external_ref
                    AND COALESCE(automation_delivery_routes.account_id, '') = p.account_id
                    AND COALESCE(automation_delivery_routes.thread_id, '') = p.thread_id
              )
              OR EXISTS (
                  SELECT 1
                  FROM im_pairing_sessions ps
                  JOIN im_pairings p ON p.owner_user_id = ps.owner_user_id AND p.pairing_id = ps.pairing_id
                  WHERE ps.owner_user_id = ` + s.bind(16) + ` AND ps.channel_type = ` + s.bind(17) + ` AND ps.account_id = ` + s.bind(18) + `
                    AND p.agent_id = automation_delivery_routes.agent_id
                    AND COALESCE(automation_delivery_routes.channel, '') = ps.channel_type
                    AND COALESCE(automation_delivery_routes."to", '') = ps.external_ref
                    AND COALESCE(automation_delivery_routes.account_id, '') = ps.account_id
                    AND COALESCE(automation_delivery_routes.thread_id, '') = ps.thread_id
              )
          )
      )
  )`
	_, err := tx.ExecContext(ctx, query,
		ownerUserID, channelType, accountID, ownerUserID, channelType, accountID,
		ownerUserID, channelType, accountID, ownerUserID, channelType, accountID,
		ownerUserID, channelType, accountID, ownerUserID, channelType, accountID,
	)
	return err
}
