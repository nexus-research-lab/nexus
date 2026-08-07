// INPUT: owner 作用域内的 channel config、account 或 pairing 精确标识。
// OUTPUT: 对应持久化真相源是否仍存在。
// POS: 配置控制面写后核验使用的窄读取接口，不以聚合视图推断删除结果。
package channels

import (
	"context"
	"strings"
)

func (s *ControlService) HasChannelConfig(ctx context.Context, ownerUserID string, channelType string) (bool, error) {
	row, err := s.getChannelConfigRow(
		ctx,
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
	)
	return row != nil, err
}

func (s *ControlService) HasChannelAccount(
	ctx context.Context,
	ownerUserID string,
	channelType string,
	accountID string,
) (bool, error) {
	rows, err := s.listChannelAccountRows(
		ctx,
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
	)
	if err != nil {
		return false, err
	}
	accountID = strings.TrimSpace(accountID)
	for _, row := range rows {
		if row.AccountID == accountID {
			return true, nil
		}
	}
	return false, nil
}

func (s *ControlService) HasAnyChannelAccount(
	ctx context.Context,
	ownerUserID string,
	channelType string,
) (bool, error) {
	rows, err := s.listChannelAccountRows(
		ctx,
		normalizeChannelOwnerUserID(ownerUserID),
		normalizeIMChannelType(channelType),
	)
	return len(rows) > 0, err
}

func (s *ControlService) HasPairing(ctx context.Context, ownerUserID string, pairingID string) (bool, error) {
	row, err := s.getPairingRow(
		ctx,
		normalizeChannelOwnerUserID(ownerUserID),
		strings.TrimSpace(pairingID),
	)
	return row != nil, err
}
