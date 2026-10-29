// Package chat holds the messages of a match: sending (text and files),
// history, deleting, read marks and unread counts. Callers check first that
// the user is part of a live match (swipes.Get).
package chat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"mirror/server/internal/cvtext"
	"mirror/server/internal/photos"
	"mirror/server/internal/profiles"
	"mirror/server/internal/storage"
	"mirror/server/internal/swipes"
)

const (
	MaxBody      = 4000
	MaxFileBytes = 10 << 20
	PageSize     = 50

	maxImageSide   = 1600
	maxClientID    = 100
	maxFileNameLen = 200
)

var (
	ErrEmpty    = errors.New("write a message or attach a file")
	ErrTooLong  = errors.New("a message can be up to 4000 characters")
	ErrClientID = errors.New("send a client_id (1 to 100 characters)")
	ErrFileType = errors.New("files must be JPEG, PNG or WebP images, or PDF or DOCX documents")
	ErrNotFound = errors.New("message not found")
)

type Message struct {
	ID           string `gorm:"type:uuid;default:gen_random_uuid()"`
	MatchID      string
	SenderUserID string
	Body         string
	FileID       *string
	File         *profiles.File `gorm:"foreignKey:FileID"`
	FileName     string
	ClientID     string
	CreatedAt    time.Time
	DeletedAt    *time.Time
}

// Upload is a file attached to a message.
type Upload struct {
	Name string
	Data []byte
}

// IsImage reports whether a stored chat file is shown inline.
func IsImage(f *profiles.File) bool {
	return f != nil && f.ContentType == "image/jpeg"
}

// Send saves a message from userID in match m. Sending the same client id
// again returns the first message.
func Send(ctx context.Context, db *gorm.DB, files *storage.Local, m swipes.Match, userID, body, clientID string, upload *Upload) (Message, error) {
	body = strings.TrimSpace(body)
	switch {
	case clientID == "" || len(clientID) > maxClientID:
		return Message{}, ErrClientID
	case body == "" && upload == nil:
		return Message{}, ErrEmpty
	case utf8.RuneCountInString(body) > MaxBody:
		return Message{}, ErrTooLong
	}
	if old, err := byClientID(ctx, db, m.ID, clientID); err == nil {
		return old, nil
	}

	msg := Message{MatchID: m.ID, SenderUserID: userID, Body: body, ClientID: clientID}
	if upload != nil {
		f, name, err := saveFile(ctx, db, files, userID, *upload)
		if err != nil {
			return Message{}, err
		}
		msg.FileID, msg.FileName = &f.ID, name
	}
	res := db.WithContext(ctx).Omit("File").Clauses(clause.OnConflict{DoNothing: true}).Create(&msg)
	if res.Error != nil {
		return Message{}, res.Error
	}
	if res.RowsAffected == 0 {
		// The same send raced in twice; the first one won.
		return byClientID(ctx, db, m.ID, clientID)
	}
	return get(ctx, db, m.ID, msg.ID)
}

// saveFile checks, cleans and stores an attachment. Images are re-encoded,
// which drops their metadata (such as GPS location).
func saveFile(ctx context.Context, db *gorm.DB, files *storage.Local, userID string, up Upload) (profiles.File, string, error) {
	name := cleanName(up.Name)
	data := up.Data
	var kind string
	switch http.DetectContentType(data) {
	case "image/jpeg", "image/png", "image/webp":
		jpg, err := photos.Resize(data, maxImageSide)
		if err != nil {
			return profiles.File{}, "", ErrFileType
		}
		data, kind = jpg, "image/jpeg"
		name = strings.TrimSuffix(name, filepath.Ext(name)) + ".jpg"
	default:
		t, err := cvtext.DetectType(data)
		if err != nil {
			return profiles.File{}, "", ErrFileType
		}
		kind = t
	}

	key, err := files.Save(data)
	if err != nil {
		return profiles.File{}, "", err
	}
	sum := sha256.Sum256(data)
	f := profiles.File{
		OwnerUserID: userID, Kind: "chat_file", StorageKey: key,
		ContentType: kind, SizeBytes: int64(len(data)), Sha256: hex.EncodeToString(sum[:]),
	}
	if err := db.WithContext(ctx).Create(&f).Error; err != nil {
		files.Delete(key)
		return profiles.File{}, "", err
	}
	return f, name, nil
}

// cleanName keeps the last part of an uploaded file name, short and
// printable.
func cleanName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == '"' || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "." || name == "/" || name == "" {
		name = "file"
	}
	if len(name) > maxFileNameLen {
		name = name[len(name)-maxFileNameLen:]
	}
	return name
}

func byClientID(ctx context.Context, db *gorm.DB, matchID, clientID string) (Message, error) {
	var msg Message
	err := db.WithContext(ctx).Preload("File").Where("match_id = ? AND client_id = ?", matchID, clientID).First(&msg).Error
	return msg, err
}

func get(ctx context.Context, db *gorm.DB, matchID, id string) (Message, error) {
	var msg Message
	err := db.WithContext(ctx).Preload("File").Where("match_id = ? AND id::text = ?", matchID, id).First(&msg).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return Message{}, ErrNotFound
	}
	return msg, err
}

// MatchOf returns the match id of a message, for checking access first.
func MatchOf(ctx context.Context, db *gorm.DB, id string) (string, error) {
	var matchID string
	err := db.WithContext(ctx).Model(&Message{}).Select("match_id").Where("id::text = ?", id).Scan(&matchID).Error
	if err == nil && matchID == "" {
		err = ErrNotFound
	}
	return matchID, err
}

// History returns up to PageSize messages older than the message before
// (the newest ones when before is empty), oldest first, and whether there
// are older ones.
func History(ctx context.Context, db *gorm.DB, matchID, before string) ([]Message, bool, error) {
	q := db.WithContext(ctx).Preload("File").Where("match_id = ?", matchID)
	if before != "" {
		q = q.Where("(created_at, id) < (SELECT created_at, id FROM messages WHERE id::text = ? AND match_id = ?)", before, matchID)
	}
	var list []Message
	if err := q.Order("created_at DESC, id DESC").Limit(PageSize + 1).Find(&list).Error; err != nil {
		return nil, false, err
	}
	more := len(list) > PageSize
	if more {
		list = list[:PageSize]
	}
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
	return list, more, nil
}

// Delete hides one of the user's own messages. The row stays for
// moderation.
func Delete(ctx context.Context, db *gorm.DB, matchID, userID, id string) (Message, error) {
	res := db.WithContext(ctx).Model(&Message{}).
		Where("match_id = ? AND id::text = ? AND sender_user_id = ? AND deleted_at IS NULL", matchID, id, userID).
		Update("deleted_at", time.Now())
	if res.Error != nil {
		return Message{}, res.Error
	}
	if res.RowsAffected == 0 {
		return Message{}, ErrNotFound
	}
	return get(ctx, db, matchID, id)
}

// FileOf returns a message's attachment, unless the message was deleted.
func FileOf(ctx context.Context, db *gorm.DB, files *storage.Local, matchID, id string) (Message, []byte, error) {
	msg, err := get(ctx, db, matchID, id)
	if err != nil {
		return Message{}, nil, err
	}
	if msg.DeletedAt != nil || msg.File == nil {
		return Message{}, nil, ErrNotFound
	}
	data, err := files.Open(msg.File.StorageKey)
	return msg, data, err
}

// MarkRead records that the user has read the chat up to now.
func MarkRead(ctx context.Context, db *gorm.DB, m swipes.Match, userID string) (time.Time, error) {
	column := "employer_read_at"
	if userID == m.CandidateUserID {
		column = "candidate_read_at"
	}
	now := time.Now()
	err := db.WithContext(ctx).Model(&swipes.Match{}).Where("id = ?", m.ID).Update(column, now).Error
	return now, err
}

// Summary is what the inbox shows for one match.
type Summary struct {
	Unread int
	Last   *Message
}

// myReadAt is the user's read time on a match row aliased m.
const myReadAt = "CASE WHEN m.candidate_user_id = @me THEN m.candidate_read_at ELSE m.employer_read_at END"

// unreadFor counts messages from the other person after the user's read time.
const unreadFor = `(SELECT count(*) FROM messages x
	WHERE x.match_id = m.id AND x.deleted_at IS NULL AND x.sender_user_id <> @me
		AND x.created_at > COALESCE(` + myReadAt + `, '-infinity'))`

// Summaries returns the unread count and last message of each match.
func Summaries(ctx context.Context, db *gorm.DB, userID string, matchIDs []string) (map[string]Summary, error) {
	out := map[string]Summary{}
	if len(matchIDs) == 0 {
		return out, nil
	}
	var counts []struct {
		ID     string
		Unread int
	}
	err := db.WithContext(ctx).Raw(`SELECT m.id, `+unreadFor+` AS unread FROM matches m WHERE m.id IN @ids`,
		map[string]any{"me": userID, "ids": matchIDs}).Scan(&counts).Error
	if err != nil {
		return nil, err
	}
	var last []Message
	err = db.WithContext(ctx).Preload("File").
		Where(`id IN (SELECT DISTINCT ON (match_id) id FROM messages WHERE match_id IN ?
			ORDER BY match_id, created_at DESC, id DESC)`, matchIDs).
		Find(&last).Error
	if err != nil {
		return nil, err
	}
	for _, c := range counts {
		out[c.ID] = Summary{Unread: c.Unread}
	}
	for i := range last {
		s := out[last[i].MatchID]
		s.Last = &last[i]
		out[last[i].MatchID] = s
	}
	return out, nil
}

// Unread counts the user's live matches not opened yet, and the messages
// waiting in all of them.
func Unread(ctx context.Context, db *gorm.DB, userID string) (newMatches, messages int, err error) {
	var row struct {
		NewMatches int
		Messages   int
	}
	err = db.WithContext(ctx).Raw(`SELECT
			count(*) FILTER (WHERE `+myReadAt+` IS NULL) AS new_matches,
			COALESCE(sum(`+unreadFor+`), 0) AS messages
		FROM matches m
		WHERE m.state = 'matched' AND (m.candidate_user_id = @me OR m.employer_user_id = @me)`,
		map[string]any{"me": userID}).Scan(&row).Error
	return row.NewMatches, row.Messages, err
}
