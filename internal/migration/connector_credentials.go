// INPUT: 已完成 00129 schema 的 Connector connection 密文、active key 与显式 legacy keyring。
// OUTPUT: 为旧密文补齐稳定 key_id，并把 legacy-key 密文逐条 CAS 重加密到 active key。
// POS: schema migration 之后、业务 service 启动之前的幂等数据迁移；单条未知密钥不阻断其他记录。
package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/nexus-research-lab/nexus/internal/config"
	"github.com/nexus-research-lab/nexus/internal/connectors/credentials"
	"github.com/nexus-research-lab/nexus/internal/storage"
)

// ConnectorCredentialKeyReport 描述一次可重试密钥识别/轮换，不包含密钥或明文。
type ConnectorCredentialKeyReport struct {
	Scanned          int
	AlreadyCurrent   int
	Identified       int
	Reencrypted      int
	RecoveryRequired int
	Corrupt          int
	Conflicted       int
	KeyringAvailable bool
}

type encryptedConnectorCredential struct {
	ownerUserID string
	connectorID string
	encrypted   string
	keyID       sql.NullString
}

// RunConnectorCredentialKeyMigration 逐条识别/轮换 Connector connection 密文。
func RunConnectorCredentialKeyMigration(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) (ConnectorCredentialKeyReport, error) {
	if logger == nil {
		logger = slog.Default()
	}
	db, err := storage.OpenDB(cfg)
	if err != nil {
		return ConnectorCredentialKeyReport{}, fmt.Errorf("打开 Connector credentials migration 数据库: %w", err)
	}
	defer db.Close()

	records, err := listEncryptedConnectorCredentials(ctx, db)
	if err != nil {
		return ConnectorCredentialKeyReport{}, err
	}
	report := ConnectorCredentialKeyReport{Scanned: len(records)}
	if len(records) == 0 {
		return report, nil
	}

	keyring, keyringErr := credentials.NewKeyring(
		cfg.ConnectorCredentialsKey,
		cfg.ConnectorCredentialsLegacyKeys,
	)
	if keyringErr != nil {
		report.RecoveryRequired = len(records)
		logger.Warn("Connector credentials keyring 不可用，保留原密文等待恢复",
			"records", len(records),
			"err", keyringErr,
		)
		return report, nil
	}
	report.KeyringAvailable = true
	activeKeyID := keyring.ActiveKeyID()

	for _, record := range records {
		storedKeyID := strings.TrimSpace(record.keyID.String)
		if storedKeyID == activeKeyID {
			report.AlreadyCurrent++
			continue
		}
		plain, matchedKeyID, decryptErr := keyring.Decrypt(storedKeyID, record.encrypted)
		if decryptErr != nil {
			if errors.Is(decryptErr, credentials.ErrKeyUnavailable) ||
				errors.Is(decryptErr, credentials.ErrNoMatchingKey) {
				report.RecoveryRequired++
			} else {
				report.Corrupt++
			}
			continue
		}

		nextEncrypted := record.encrypted
		if matchedKeyID != activeKeyID {
			nextEncrypted, _, err = keyring.Encrypt(plain)
			if err != nil {
				return report, fmt.Errorf("重加密 Connector %s: %w", record.connectorID, err)
			}
		}
		changed, updateErr := compareAndSwapConnectorCredentialKey(
			ctx,
			db,
			storage.NormalizeSQLDriver(cfg.DatabaseDriver),
			record,
			nextEncrypted,
			activeKeyID,
		)
		if updateErr != nil {
			return report, updateErr
		}
		if !changed {
			report.Conflicted++
			continue
		}
		if matchedKeyID == activeKeyID {
			report.Identified++
		} else {
			report.Reencrypted++
		}
	}

	if report.Identified > 0 || report.Reencrypted > 0 ||
		report.RecoveryRequired > 0 || report.Corrupt > 0 || report.Conflicted > 0 {
		logger.Info("Connector credentials key migration 完成",
			"scanned", report.Scanned,
			"already_current", report.AlreadyCurrent,
			"identified", report.Identified,
			"reencrypted", report.Reencrypted,
			"recovery_required", report.RecoveryRequired,
			"corrupt", report.Corrupt,
			"conflicted", report.Conflicted,
		)
	}
	return report, nil
}

func listEncryptedConnectorCredentials(
	ctx context.Context,
	db *sql.DB,
) ([]encryptedConnectorCredential, error) {
	rows, err := db.QueryContext(ctx, `
SELECT owner_user_id, connector_id, credentials_encrypted, credentials_key_id
  FROM connector_connections
 WHERE COALESCE(credentials_encrypted, '') <> ''
 ORDER BY owner_user_id, connector_id`)
	if err != nil {
		return nil, fmt.Errorf("列举 Connector encrypted credentials: %w", err)
	}
	defer rows.Close()
	records := make([]encryptedConnectorCredential, 0)
	for rows.Next() {
		var record encryptedConnectorCredential
		if err = rows.Scan(
			&record.ownerUserID,
			&record.connectorID,
			&record.encrypted,
			&record.keyID,
		); err != nil {
			return nil, fmt.Errorf("读取 Connector encrypted credentials: %w", err)
		}
		records = append(records, record)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历 Connector encrypted credentials: %w", err)
	}
	return records, nil
}

func compareAndSwapConnectorCredentialKey(
	ctx context.Context,
	db *sql.DB,
	driver string,
	record encryptedConnectorCredential,
	nextEncrypted string,
	nextKeyID string,
) (bool, error) {
	query := `
UPDATE connector_connections
   SET credentials_encrypted = ?, credentials_key_id = ?
 WHERE owner_user_id = ? AND connector_id = ?
   AND credentials_encrypted = ?
   AND COALESCE(credentials_key_id, '') = ?`
	if driver == "pgx" {
		query = `
UPDATE connector_connections
   SET credentials_encrypted = $1, credentials_key_id = $2
 WHERE owner_user_id = $3 AND connector_id = $4
   AND credentials_encrypted = $5
   AND COALESCE(credentials_key_id, '') = $6`
	}
	result, err := db.ExecContext(
		ctx,
		query,
		nextEncrypted,
		nextKeyID,
		record.ownerUserID,
		record.connectorID,
		record.encrypted,
		strings.TrimSpace(record.keyID.String),
	)
	if err != nil {
		return false, fmt.Errorf("提交 Connector credentials key migration %s: %w", record.connectorID, err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("读取 Connector credentials key migration 结果 %s: %w", record.connectorID, err)
	}
	return changed == 1, nil
}
