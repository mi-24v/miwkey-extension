package drive

import (
	"github.com/mi-24v/miwkey-extension/internal/infra"
	"gorm.io/gorm"
	"log/slog"
)

type Service struct {
	db           *gorm.DB
	misskeyDrive infra.MisskeyDriveClientInterface
}

func (s *Service) GetDriveIdsRemoteOrphanMedia() {
	var ids []string
	sql := `SELECT df.*
FROM drive_file df
         LEFT JOIN "user" u ON df."userId" = u.id
WHERE u."followersCount" = 0 and u.host is not null and df."userHost" is not null
   OR NOT EXISTS (
    SELECT 1
    FROM note n
        JOIN "note_favorite" fav on n.id = fav."noteId"
    WHERE to_jsonb(n."fileIds")::jsonb @> to_jsonb(df.id)::jsonb
      AND df."userHost" is not null
      AND n."clippedCount" = 0
      AND n."userHost" is not null
)`
	err := s.db.Raw(sql).Scan(&ids).Error
	if err != nil {
		// Handle the error
		return
	}

	for _, id := range ids {
		errDelete := s.misskeyDrive.Delete(id)
		if errDelete != nil {
			slog.Error("delete error", errDelete)
		}
	}
}
