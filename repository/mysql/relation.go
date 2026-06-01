package mysql

import (
	"ceddit/models"
	"ceddit/pkg/snowflake"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// CreateFollow inserts a following row and an outbox event in a single transaction.
// ON DUPLICATE KEY UPDATE handles concurrent races on uk_from_to:
// the last writer's id wins, both outbox events are produced, consumer is idempotent.
func CreateFollow(fromUserID, toUserID int64) (*models.Following, error) {
	now := time.Now()
	id := snowflake.GenID()
	eventID := fmt.Sprintf("%d", snowflake.GenID())

	payload := models.OutboxPayload{
		EventID:    eventID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		OccurredAt: now.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	tx, err := db.Beginx()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.Exec(
		`INSERT INTO following (id, from_user_id, to_user_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON DUPLICATE KEY UPDATE
		     id = VALUES(id),
		     updated_at = VALUES(updated_at)`,
		id, fromUserID, toUserID, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert following: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO outbox (id, aggregate_type, aggregate_id, type, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snowflake.GenID(), "FOLLOW", id, "USER_RELATION_CHANGE", string(payloadJSON), now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert outbox: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	return &models.Following{
		ID:         id,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		CreatedAt:  now,
		UpdatedAt:  now,
	}, nil
}

// DeleteFollow removes a following row and inserts an outbox event in one transaction.
// It SELECTs FOR UPDATE first to capture the row id for the outbox event.
func DeleteFollow(fromUserID, toUserID int64) error {
	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var rel models.Following
	err = tx.Get(&rel,
		`SELECT id, from_user_id, to_user_id, created_at, updated_at
		 FROM following
		 WHERE from_user_id = ? AND to_user_id = ?
		 FOR UPDATE`,
		fromUserID, toUserID,
	)
	if err != nil {
		return fmt.Errorf("not following: %w", err)
	}

	_, err = tx.Exec(`DELETE FROM following WHERE id = ?`, rel.ID)
	if err != nil {
		return fmt.Errorf("delete following: %w", err)
	}

	now := time.Now()
	eventID := fmt.Sprintf("%d", snowflake.GenID())

	payload := models.OutboxPayload{
		EventID:    eventID,
		FromUserID: fromUserID,
		ToUserID:   toUserID,
		OccurredAt: now.UTC().Format("2006-01-02T15:04:05.000Z"),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	_, err = tx.Exec(
		`INSERT INTO outbox (id, aggregate_type, aggregate_id, type, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		snowflake.GenID(), "UNFOLLOW", rel.ID, "USER_RELATION_CHANGE", string(payloadJSON), now,
	)
	if err != nil {
		return fmt.Errorf("insert outbox: %w", err)
	}

	return tx.Commit()
}

// IsFollowing checks whether a follow relationship exists.
func IsFollowing(fromUserID, toUserID int64) (bool, error) {
	var count int
	err := db.Get(&count,
		`SELECT COUNT(*) FROM following WHERE from_user_id = ? AND to_user_id = ?`,
		fromUserID, toUserID,
	)
	return count > 0, err
}

// encodeCursor packs a (created_at, id) pair into a cursor string.
func encodeCursor(t time.Time, id int64) string {
	raw := fmt.Sprintf("%d,%d", t.UnixNano(), id)
	return base64.URLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor unpacks a cursor string into (created_at, id).
func decodeCursor(cursor string) (time.Time, int64, error) {
	decoded, err := base64.URLEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor: %w", err)
	}
	parts := strings.SplitN(string(decoded), ",", 2)
	if len(parts) != 2 {
		return time.Time{}, 0, fmt.Errorf("invalid cursor format")
	}
	ns, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor time: %w", err)
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, 0, fmt.Errorf("invalid cursor id: %w", err)
	}
	return time.Unix(0, ns).UTC(), id, nil
}

// GetFollowingList returns users that fromUserID follows, cursor-paginated by (created_at DESC, id DESC).
func GetFollowingList(fromUserID int64, cursor string, limit int) ([]models.FollowListItem, string, error) {
	var rows []struct {
		UserID    int64     `db:"to_user_id"`
		Username  string    `db:"username"`
		CreatedAt time.Time `db:"created_at"`
	}

	if cursor == "" {
		if err := db.Select(&rows,
			`SELECT f.to_user_id, u.username, f.created_at
			 FROM following f
			 JOIN user u ON u.user_id = f.to_user_id
			 WHERE f.from_user_id = ?
			 ORDER BY f.created_at DESC, f.to_user_id DESC
			 LIMIT ?`,
			fromUserID, limit+1,
		); err != nil {
			return nil, "", fmt.Errorf("get following list: %w", err)
		}
	} else {
		cursorTime, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		if err := db.Select(&rows,
			`SELECT f.to_user_id, u.username, f.created_at
			 FROM following f
			 JOIN user u ON u.user_id = f.to_user_id
			 WHERE f.from_user_id = ?
			   AND (f.created_at, f.to_user_id) < (?, ?)
			 ORDER BY f.created_at DESC, f.to_user_id DESC
			 LIMIT ?`,
			fromUserID, cursorTime, cursorID, limit+1,
		); err != nil {
			return nil, "", fmt.Errorf("get following list: %w", err)
		}
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	list := make([]models.FollowListItem, len(rows))
	for i, r := range rows {
		list[i] = models.FollowListItem{
			UserID:    r.UserID,
			Username:  r.Username,
			CreatedAt: r.CreatedAt,
		}
	}

	var nextCursor string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.UserID)
	}

	return list, nextCursor, nil
}

// GetFollowerList returns followers of toUserID from the follower projection table,
// cursor-paginated by (created_at DESC, from_user_id DESC).
// It also computes mutual status by checking the following table.
func GetFollowerList(toUserID int64, cursor string, limit int) ([]models.FollowerListItem, string, error) {
	var rows []struct {
		UserID    int64     `db:"from_user_id"`
		Username  string    `db:"username"`
		CreatedAt time.Time `db:"created_at"`
	}

	if cursor == "" {
		if err := db.Select(&rows,
			`SELECT f.from_user_id, u.username, f.created_at
			 FROM follower f
			 JOIN user u ON u.user_id = f.from_user_id
			 WHERE f.to_user_id = ?
			 ORDER BY f.created_at DESC, f.from_user_id DESC
			 LIMIT ?`,
			toUserID, limit+1,
		); err != nil {
			return nil, "", fmt.Errorf("get follower list: %w", err)
		}
	} else {
		cursorTime, cursorID, err := decodeCursor(cursor)
		if err != nil {
			return nil, "", err
		}
		if err := db.Select(&rows,
			`SELECT f.from_user_id, u.username, f.created_at
			 FROM follower f
			 JOIN user u ON u.user_id = f.from_user_id
			 WHERE f.to_user_id = ?
			   AND (f.created_at, f.from_user_id) < (?, ?)
			 ORDER BY f.created_at DESC, f.from_user_id DESC
			 LIMIT ?`,
			toUserID, cursorTime, cursorID, limit+1,
		); err != nil {
			return nil, "", fmt.Errorf("get follower list: %w", err)
		}
	}

	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	// Batch-check mutual status by querying which of these followers the
	// current user follows back.
	followerIDs := make([]int64, len(rows))
	for i, r := range rows {
		followerIDs[i] = r.UserID
	}

	mutualSet := make(map[int64]bool, len(followerIDs))
	if len(followerIDs) > 0 {
		query, args, err := sqlx.In(
			`SELECT to_user_id FROM following
			 WHERE from_user_id = ? AND to_user_id IN (?)`,
			toUserID, followerIDs,
		)
		if err == nil {
			query = db.Rebind(query)
			var mutualIDs []int64
			if err := db.Select(&mutualIDs, query, args...); err == nil {
				for _, mid := range mutualIDs {
					mutualSet[mid] = true
				}
			}
		}
	}

	list := make([]models.FollowerListItem, len(rows))
	for i, r := range rows {
		list[i] = models.FollowerListItem{
			UserID:    r.UserID,
			Username:  r.Username,
			IsMutual:  mutualSet[r.UserID],
			CreatedAt: r.CreatedAt,
		}
	}

	var nextCursor string
	if hasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		nextCursor = encodeCursor(last.CreatedAt, last.UserID)
	}

	return list, nextCursor, nil
}

// GetFollowingCount returns the number of users fromUserID follows.
func GetFollowingCount(userID int64) (int64, error) {
	var count int64
	err := db.Get(&count,
		`SELECT COUNT(*) FROM following WHERE from_user_id = ?`, userID,
	)
	return count, err
}

// GetFollowerCountFromProjection returns the fan count from the follower projection.
func GetFollowerCountFromProjection(userID int64) (int64, error) {
	var count int64
	err := db.Get(&count,
		`SELECT COUNT(*) FROM follower WHERE to_user_id = ?`, userID,
	)
	return count, err
}
