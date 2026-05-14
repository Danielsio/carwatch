package sqlite

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

// generateShareToken returns a 32-character hex string from 16 random bytes.
func generateShareToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random bytes: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func migrate(db *sql.DB) error {
	steps := []func(*sql.DB) error{
		migrateInitialSchema,
		migrateUserDigestColumns,
		migrateListingHistoryChatID,
		migrateUserLanguage,
		migrateSearchKeywords,
		migrateSearchUserSeq,
		migrateUserTierColumns,
		migrateUserDailyDigest,
		migrateListingPerfIndexes,
		migrateDropLegacyCacheTables,
		migrateUserChannelColumns,
		migrateChannelIDIndexAndBackfill,
		migrateSearchShareToken,
		migratePriceHistoryV2,
		migrateListingHistoryFitnessScore,
		migrateListingHistoryImageURL,
		migrateUserLastSeenAt,
		migrateListingHistorySearchID,
		migrateLinkTokensTable,
		migrateUserLinkedWebID,
		migrateUserLinkedWebIndex,
		migrateUsersActiveIndex,
		migrateMissingIndexes,
		migrateListingVehicleDetails,
		migrateSellerFilterAndListingCommercial,
		migrateListingMarketValue,
		migrateListingSubModelIDAndBasePrice,
		migrateListingUserSeen,
		migrateSearchPricePhotoFilters,
	}
	for _, step := range steps {
		if err := step(db); err != nil {
			return err
		}
	}
	return nil
}
